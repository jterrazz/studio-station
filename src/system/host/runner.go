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
	"strings"
	"syscall"

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
	return ctx, cmd, cancel, nil
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
	code, err := exitOf(ctx, cmd.Run())
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
	code, err := exitOf(ctx, cmd.Run())
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
