package host

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

type sockets struct{}

type listener struct{ l net.Listener }

func (l listener) Accept() (system.Conn, error) { return l.l.Accept() }
func (l listener) Close() error                 { return l.l.Close() }
func (l listener) Port() int {
	if a, ok := l.l.Addr().(*net.TCPAddr); ok {
		return a.Port
	}
	return 0
}

const sunPathMax = 100

// hazard: a unix socket path is capped near 104 bytes (sun_path); a longer one is
// reached through a short symlink to its folder, the same for the listener and
// every client, so both name one socket.
func reachable(path string) string {
	if len(path) < sunPathMax {
		return path
	}
	dir, name := filepath.Split(path)
	sum := sha256.Sum256([]byte(dir))
	for _, base := range []string{os.TempDir(), "/tmp"} {
		link := filepath.Join(base, "station-"+hex.EncodeToString(sum[:6]))
		short := filepath.Join(link, name)
		if len(short) >= sunPathMax {
			continue
		}
		if target, err := os.Readlink(link); err != nil || target != dir {
			_ = os.Remove(link)
			_ = os.Symlink(dir, link)
		}
		return short
	}
	return path
}

func (sockets) ListenUnix(path string) (system.Listener, error) {
	l, err := net.Listen("unix", reachable(path))
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = l.Close()
		return nil, err
	}
	return listener{l: l}, nil
}

func (sockets) DialUnix(ctx context.Context, path string, timeout time.Duration) (system.Conn, error) {
	d := net.Dialer{Timeout: timeout}
	return d.DialContext(ctx, "unix", reachable(path))
}

// invariant: no code path binds every interface: an empty or unspecified ip is
// refused here, the one place a TCP listener is made.
func (sockets) ListenTCP(ip string, port int) (system.Listener, error) {
	parsed := net.ParseIP(ip)
	if ip == "" || parsed == nil || parsed.IsUnspecified() {
		return nil, system.ErrWildcard
	}
	l, err := net.Listen("tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return nil, err
	}
	return listener{l: l}, nil
}

func (sockets) Pipe(ctx context.Context, ip string, port int, in io.Reader, out io.Writer) error {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return err
	}
	go func() {
		_, _ = io.Copy(conn, in)
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
	}()
	_, err = io.Copy(out, conn)
	_ = conn.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

type halfCloser interface{ CloseWrite() error }

// invariant: each direction ends on its own: the end of one side's bytes
// half-closes the other's write, so a request's EOF never cuts its response;
// both close once both directions ended.
func (sockets) Join(a, b system.Conn) {
	var wg sync.WaitGroup
	carry := func(dst, src system.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if h, ok := dst.(halfCloser); ok {
			_ = h.CloseWrite()
		} else {
			_ = dst.Close()
		}
	}
	wg.Add(2)
	go carry(a, b)
	go carry(b, a)
	wg.Wait()
	_ = a.Close()
	_ = b.Close()
}
