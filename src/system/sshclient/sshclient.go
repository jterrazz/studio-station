package sshclient

import (
	"context"
	"crypto/ed25519"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/jterrazz/studio-station/src/system"
)

const (
	keepalive = 15 * time.Second
	missed    = 3
)

type dialer struct{}

func New() system.SSH { return dialer{} }

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
	probe, err := ssh.NewPublicKey(ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)))
	if err != nil {
		return nil
	}
	var mismatch *knownhosts.KeyError
	if !errors.As(check(name+":22", remote, probe), &mismatch) {
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

func (dialer) Dial(ctx context.Context, t system.SSHTarget) (system.SSHClient, error) {
	key, err := os.ReadFile(t.KeyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}
	cfg := &ssh.ClientConfig{
		User:            t.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: hostKeys(t.KnownHosts, t.Name),
		Timeout:         10 * time.Second,
	}
	d := net.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", t.Addr)
	if err != nil {
		return nil, err
	}
	cfg.HostKeyAlgorithms = pinnedAlgorithms(t.KnownHosts, t.Name, conn.RemoteAddr())
	c, chans, reqs, err := ssh.NewClientConn(conn, t.Addr, cfg)
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

func (c *client) Exec(ctx context.Context, command string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	session, err := c.c.NewSession()
	if err != nil {
		return 0, err
	}
	defer func() { _ = session.Close() }()
	session.Stdin, session.Stdout, session.Stderr = stdin, stdout, stderr
	if err := session.Start(command); err != nil {
		return 0, err
	}
	ended := make(chan error, 1)
	go func() { ended <- session.Wait() }()
	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		_ = session.Close()
		return 0, ctx.Err()
	case err := <-ended:
		var exit *ssh.ExitError
		if errors.As(err, &exit) {
			return exit.ExitStatus(), nil
		}
		return 0, err
	}
}

func (c *client) Forward(ctx context.Context, ip string, port int) (system.Conn, error) {
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
