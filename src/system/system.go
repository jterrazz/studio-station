package system

import (
	"context"
	"errors"
	"io/fs"
	"time"
)

var (
	ErrNotFound = errors.New("command not found")
	ErrTimeout  = errors.New("command timed out")
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
}

type Output struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type Sig int

const (
	SigTerm Sig = iota
	SigKill
)

type Runner interface {
	Output(ctx context.Context, c Cmd) (Output, error)
	Attach(ctx context.Context, c Cmd, out Stream) (exitCode int, err error)
	Start(c Cmd, logPath string) (pid int, err error)
	LookPath(name string) (string, bool)
	Alive(pid int) bool
	Signal(pid int, sig Sig) error
}

type FS interface {
	fs.FS
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(path string) error
	RemoveAll(path string) error
	Stat(path string) (fs.FileInfo, error)
	ReadDir(path string) ([]fs.DirEntry, error)
	Glob(pattern string) ([]string, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
	DirSize(path string) int64
	Chmod(path string, perm fs.FileMode) error
	Chown(path string, uid, gid int) error
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
	Post(ctx context.Context, url string, header map[string]string, body []byte) (status int, respBody []byte, err error)
	Delete(ctx context.Context, url string, header map[string]string) (status int, respBody []byte, err error)
}

type Clock interface {
	Now() time.Time
	Sleep(ctx context.Context, d time.Duration) error
}

type Host struct {
	Run   Runner
	FS    FS
	Env   Env
	Net   Net
	Clock Clock
}
