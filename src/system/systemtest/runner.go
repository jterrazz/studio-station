package systemtest

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/jterrazz/studio-station/src/system"
)

type Runner struct {
	mu      sync.Mutex
	t       testing.TB
	script  []*Call
	Paths   map[string]string
	live    map[int]bool
	Calls   []system.Cmd
	Signals []system.Sig
}

type Call struct {
	name   string
	args   []string
	output system.Output
	err    error
	Seen   int
}

func NewRunner(t testing.TB) *Runner {
	return &Runner{t: t, Paths: map[string]string{}, live: map[int]bool{}}
}

func (r *Runner) On(name string, args ...string) *Call {
	c := &Call{name: name, args: args}
	r.script = append(r.script, c)
	return c
}

func (c *Call) Reply(o system.Output) *Call {
	c.output = o
	return c
}

func (c *Call) Fail(err error) *Call {
	c.err = err
	return c
}

func (r *Runner) answer(c system.Cmd) *Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Calls = append(r.Calls, c)
	for _, call := range r.script {
		if call.name == c.Name && slices.Equal(call.args, c.Args) {
			call.Seen++
			return call
		}
	}
	r.t.Fatalf("systemtest: unscripted call %s %s", c.Name, strings.Join(c.Args, " "))
	return &Call{}
}

func (r *Runner) Output(_ context.Context, c system.Cmd) (system.Output, error) {
	call := r.answer(c)
	return call.output, call.err
}

func (r *Runner) Attach(_ context.Context, c system.Cmd, _ system.Stream) (system.Attached, error) {
	call := r.answer(c)
	last := ""
	for _, line := range strings.Split(call.output.Stderr, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			last = line
		}
	}
	return system.Attached{ExitCode: call.output.ExitCode, LastStderr: last}, call.err
}

func (r *Runner) Start(c system.Cmd, _ string) (int, error) {
	call := r.answer(c)
	if call.err != nil {
		return 0, call.err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	pid := 1000 + len(r.Calls)
	r.live[pid] = true
	return pid, nil
}

func (r *Runner) LookPath(name string) (string, bool) {
	path, ok := r.Paths[name]
	return path, ok
}

func (r *Runner) Alive(pid int) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.live[pid]
}

func (r *Runner) Spawn(pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.live[pid] = true
}

func (r *Runner) Exit(pid int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.live, pid)
}

func (r *Runner) Signal(pid int, sig system.Sig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.live[pid] {
		return fmt.Errorf("no process %d", pid)
	}
	r.Signals = append(r.Signals, sig)
	delete(r.live, pid)
	return nil
}
