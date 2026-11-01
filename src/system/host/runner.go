package host

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

type runner struct {
	env    []string
	dirs   []string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func (r *runner) LookPath(name string) (string, bool) {
	if strings.Contains(name, "/") {
		return name, executable(name)
	}
	for _, dir := range r.dirs {
		candidate := filepath.Join(dir, name)
		if executable(candidate) {
			return candidate, true
		}
	}
	return "", false
}

func executable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func (r *runner) command(ctx context.Context, c system.Cmd) (context.Context, *exec.Cmd, context.CancelFunc, error) {
	path, ok := r.LookPath(c.Name)
	if !ok {
		return ctx, nil, nil, fmt.Errorf("%s: %w", c.Name, system.ErrNotFound)
	}
	cancel := context.CancelFunc(func() {})
	if c.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.Timeout)
	}
	cmd := exec.CommandContext(ctx, path, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(append([]string{}, r.env...), c.Env...)
	if c.Track != "" {
		// invariant: a tracked child leads its own process group, so the end of
		// its context kills whatever it spawned too, never only the direct child.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
		cmd.WaitDelay = time.Second
	}
	return ctx, cmd, cancel, nil
}

func run(cmd *exec.Cmd, track string) error {
	if track == "" {
		return cmd.Run()
	}
	if err := os.MkdirAll(track, 0o700); err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	// invariant: a tracked child's file lives while it runs, so a parent killed
	// with -9 leaves the record its next start reaps.
	file := filepath.Join(track, strconv.Itoa(cmd.Process.Pid))
	_ = os.WriteFile(file, []byte(strconv.FormatInt(time.Now().Unix(), 10)+"\n"), 0o600)
	err := cmd.Wait()
	_ = os.Remove(file)
	return err
}

func (r *runner) Reap(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	killed := 0
	for _, e := range entries {
		file := filepath.Join(dir, e.Name())
		data, _ := os.ReadFile(file)
		_ = os.Remove(file)
		pid, err := strconv.Atoi(e.Name())
		if err != nil || pid <= 1 {
			continue
		}
		started, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil || !startedAt(pid, started) {
			continue
		}
		if syscall.Kill(-pid, syscall.SIGKILL) == nil {
			killed++
		}
	}
	return killed
}

// invariant: a recorded pid is killed only while it still names the process
// that was recorded: its start time must match, so a recycled pid survives.
func startedAt(pid int, unix int64) bool {
	out, err := exec.Command("/bin/ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return false
	}
	t, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", strings.TrimSpace(string(out)), time.Local)
	if err != nil {
		return false
	}
	d := t.Unix() - unix
	return d >= -2 && d <= 2
}

func (r *runner) Output(ctx context.Context, c system.Cmd) (system.Output, error) {
	ctx, cmd, cancel, err := r.command(ctx, c)
	if err != nil {
		return system.Output{}, err
	}
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	code, err := exitOf(ctx, run(cmd, c.Track))
	return system.Output{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}, err
}

func (r *runner) Attach(ctx context.Context, c system.Cmd, out system.Stream) (system.Attached, error) {
	ctx, cmd, cancel, err := r.command(ctx, c)
	if err != nil {
		return system.Attached{}, err
	}
	defer cancel()
	stderr := &lastLine{w: r.stderr}
	cmd.Stdout, cmd.Stderr = stderr, stderr
	if out == system.ToStdout {
		cmd.Stdout = r.stdout
	}
	cmd.Stdin = r.stdin
	if c.Stdin != "" {
		cmd.Stdin = strings.NewReader(c.Stdin)
	}
	code, err := exitOf(ctx, run(cmd, c.Track))
	return system.Attached{ExitCode: code, LastStderr: stderr.last()}, err
}

type lastLine struct {
	w        io.Writer
	line     []byte
	nonEmpty string
}

func (l *lastLine) Write(p []byte) (int, error) {
	for _, b := range p {
		if b != '\n' {
			l.line = append(l.line, b)
			continue
		}
		l.keep()
	}
	return l.w.Write(p)
}

func (l *lastLine) keep() {
	if line := strings.TrimSpace(string(l.line)); line != "" {
		l.nonEmpty = line
	}
	l.line = l.line[:0]
}

func (l *lastLine) last() string {
	l.keep()
	return l.nonEmpty
}

func exitOf(ctx context.Context, err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return -1, system.ErrTimeout
		}
		return -1, ctxErr
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return -1, err
}

func (r *runner) Start(c system.Cmd, logPath string) (int, error) {
	path, ok := r.LookPath(c.Name)
	if !ok {
		return 0, fmt.Errorf("%s: %w", c.Name, system.ErrNotFound)
	}
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer func() { _ = log.Close() }()
	cmd := exec.Command(path, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = append(append([]string{}, r.env...), c.Env...)
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	// invariant: the child is reaped here when it ends, so a long-lived parent (the
	// daemon) never keeps a zombie that Alive would read as running.
	go func() { _ = cmd.Wait() }()
	return cmd.Process.Pid, nil
}

func (r *runner) Alive(pid int) bool {
	return pid > 0 && syscall.Kill(pid, 0) == nil
}

func (r *runner) Signal(pid int, sig system.Sig) error {
	s := syscall.SIGTERM
	if sig == system.SigKill {
		s = syscall.SIGKILL
	}
	return syscall.Kill(pid, s)
}
