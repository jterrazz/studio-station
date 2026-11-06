package ssh

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/jterrazz/studio-station/src/machines/contract"
)

const (
	keepalive = 15 * time.Second
	missed    = 3
)

type dialer struct{}

func New() contract.GuestDialer { return dialer{} }

type client struct {
	c    *ssh.Client
	done chan struct{}
	once sync.Once
}

// invariant: the VM's host key is checked against the same per-machine
// known_hosts file the CLI's ssh uses, accept-new: an absent entry is appended,
// a different key refuses.
func hostKeys(file, name string) ssh.HostKeyCallback {
	return func(_ string, remote net.Addr, key ssh.PublicKey) error {
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			return err
		}
		f, err := os.OpenFile(file, os.O_CREATE|os.O_RDONLY, 0o600)
		if err != nil {
			return err
		}
		_ = f.Close()
		check, err := knownhosts.New(file)
		if err != nil {
			return err
		}
		err = check(name+":22", remote, key)
		var unknown *knownhosts.KeyError
		if errors.As(err, &unknown) && len(unknown.Want) == 0 {
			w, openErr := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0o600)
			if openErr != nil {
				return openErr
			}
			defer func() { _ = w.Close() }()
			_, writeErr := w.WriteString(knownhosts.Line([]string{name}, key) + "\n")
			return writeErr
		}
		return err
	}
}

// invariant: a VM already pinned is asked only for the key types of its pins,
// so a pin OpenSSH made with one type (ed25519) is never refused because Go
// would negotiate another (ecdsa).
func pinnedAlgorithms(file, name string, remote net.Addr) []string {
	check, err := knownhosts.New(file)
	if err != nil {
		return nil
	}
	placeholder, err := ssh.NewPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)))
	if err != nil {
		return nil
	}
	var mismatch *knownhosts.KeyError
	if !errors.As(check(name+":22", remote, placeholder), &mismatch) {
		return nil
	}
	var algos []string
	for _, want := range mismatch.Want {
		switch t := want.Key.Type(); t {
		case ssh.KeyAlgoRSA:
			algos = append(algos, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256, ssh.KeyAlgoRSA)
		default:
			algos = append(algos, t)
		}
	}
	return algos
}

func auth(t contract.GuestTarget) (ssh.AuthMethod, error) {
	if t.Password != "" {
		return ssh.Password(t.Password), nil
	}
	key, err := os.ReadFile(t.Key)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	return ssh.PublicKeys(signer), nil
}

func (dialer) Dial(ctx context.Context, t contract.GuestTarget) (contract.Guest, error) {
	method, err := auth(t)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:    t.User,
		Auth:    []ssh.AuthMethod{method},
		Timeout: 10 * time.Second,
	}
	// invariant: a build VM is a fresh clone of a base image: its host key is new
	// at every clone, so it is neither checked nor kept.
	if t.KnownHosts == "" {
		cfg.HostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec // outside: the trust of a fresh clone is its image key, as above.
	} else {
		cfg.HostKeyCallback = hostKeys(t.KnownHosts, t.VM)
	}
	addr := t.Address.String()
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if t.KnownHosts != "" {
		cfg.HostKeyAlgorithms = pinnedAlgorithms(t.KnownHosts, t.VM, conn.RemoteAddr())
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	cl := &client{c: ssh.NewClient(c, chans, reqs), done: make(chan struct{})}
	go cl.watch()
	go cl.keepalive()
	return cl, nil
}

func (c *client) watch() {
	_ = c.c.Wait()
	c.once.Do(func() { close(c.done) })
}

func (c *client) keepalive() {
	t := time.NewTicker(keepalive)
	defer t.Stop()
	lost := 0
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			if _, _, err := c.c.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				lost++
				if lost >= missed {
					_ = c.c.Close()
					return
				}
				continue
			}
			lost = 0
		}
	}
}

func (c *client) Exec(ctx context.Context, cmd contract.GuestCommand) (contract.GuestResult, error) {
	session, err := c.c.NewSession()
	if err != nil {
		return contract.GuestResult{}, err
	}
	defer func() { _ = session.Close() }()
	var out, errs bytes.Buffer
	session.Stdin, session.Stdout, session.Stderr = cmd.Stdin, cmd.Stdout, cmd.Stderr
	if cmd.Stdout == nil {
		session.Stdout = &out
	}
	if cmd.Stderr == nil {
		session.Stderr = &errs
	}
	if err := session.Start(cmd.Line); err != nil {
		return contract.GuestResult{}, err
	}
	ended := make(chan error, 1)
	go func() { ended <- session.Wait() }()
	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		_ = session.Close()
		return contract.GuestResult{}, ctx.Err()
	case err := <-ended:
		res := contract.GuestResult{Stdout: out.String(), Stderr: errs.String()}
		var exit *ssh.ExitError
		if errors.As(err, &exit) {
			res.Code = exit.ExitStatus()
			return res, nil
		}
		return res, err
	}
}

func quote(v string) string { return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'" }

func (c *client) Put(ctx context.Context, path string, data []byte, mode fs.FileMode) error {
	line := "/bin/cat > " + quote(path) + " && /bin/chmod " + strconv.FormatUint(uint64(mode.Perm()), 8) + " " + quote(path)
	res, err := c.Exec(ctx, contract.GuestCommand{Line: line, Stdin: bytes.NewReader(data)})
	if err != nil {
		return err
	}
	if res.Code != 0 {
		return errors.New("writing " + path + " exited " + strconv.Itoa(res.Code) + ": " + strings.TrimSpace(res.Stderr))
	}
	return nil
}

func (c *client) Forward(ctx context.Context, ip string, port int) (io.ReadWriteCloser, error) {
	type result struct {
		conn net.Conn
		err  error
	}
	got := make(chan result, 1)
	go func() {
		conn, err := c.c.Dial("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
		got <- result{conn, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-got:
		return r.conn, r.err
	}
}

func (c *client) Close() error          { return c.c.Close() }
func (c *client) Done() <-chan struct{} { return c.done }
