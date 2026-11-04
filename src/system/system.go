package system

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"time"
)

var (
	ErrNotFound    = errors.New("command not found")
	ErrTimeout     = errors.New("command timed out")
	ErrCrossDevice = errors.New("another volume: a rename cannot move it")
	ErrWildcard    = errors.New("refused: station never listens on every interface")
)

type Stream int

const (
	ToStdout Stream = iota
	ToStderr
)

type Cmd struct {
	Name    string
	Args    []string
	Dir     string
	Env     []string
	Stdin   string
	Timeout time.Duration
	Track   string
}

type Output struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Attached struct {
	ExitCode   int
	LastStderr string
}

type Sig int

const (
	SigTerm Sig = iota
	SigKill
)

type Runner interface {
	Output(ctx context.Context, c Cmd) (Output, error)
	Attach(ctx context.Context, c Cmd, out Stream) (Attached, error)
	Start(c Cmd, logPath string) (pid int, err error)
	LookPath(name string) (string, bool)
	Alive(pid int) bool
	Signal(pid int, sig Sig) error
	Reap(dir string) int
}

type FS interface {
	fs.FS
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(path string) error
	RemoveAll(path string) error
	Rename(from, to string) error
	Stat(path string) (fs.FileInfo, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Glob(pattern string) ([]string, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
	Sizes(ctx context.Context, roots []string, each func(i int, bytes int64))
	Chmod(path string, perm fs.FileMode) error
	Chown(path string, uid, gid int) error
	Readlink(path string) (string, error)
}

type Env interface {
	Executable() string
	Home() string
	User() string
	Shell() string
	Hostname() string
	Euid() int
	GOOS() string
	GOARCH() string
}

type Net interface {
	Dial(ctx context.Context, host string, port int, timeout time.Duration) bool
	Get(ctx context.Context, url string, header map[string]string) (status int, body []byte, err error)
}

type Conn interface{ io.ReadWriteCloser }

type Listener interface {
	Accept() (Conn, error)
	Close() error
	Port() int
}

type Sockets interface {
	ListenUnix(path string) (Listener, error)
	DialUnix(ctx context.Context, path string, timeout time.Duration) (Conn, error)
	ListenTCP(ip string, port int) (Listener, error)
	Pipe(ctx context.Context, ip string, port int, in io.Reader, out io.Writer) error
	Join(a, b Conn)
}

type SSHTarget struct{ Addr, User, KeyPath, KnownHosts, Name string }

type SSHClient interface {
	Exec(ctx context.Context, command string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int, err error)
	Forward(ctx context.Context, ip string, port int) (Conn, error)
	Close() error
	Done() <-chan struct{}
}

type SSH interface {
	Dial(ctx context.Context, t SSHTarget) (SSHClient, error)
}

type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

type Host struct {
	Run     Runner
	FS      FS
	Env     Env
	Net     Net
	Sockets Sockets
	Clock   Clock
}
