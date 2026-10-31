package systemtest

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/system"
)

type Runner struct {
	t       testing.TB
	script  []*Call
	Paths   map[string]string
	Live    map[int]bool
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
	return &Runner{t: t, Paths: map[string]string{}, Live: map[int]bool{}}
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

func (r *Runner) Attach(_ context.Context, c system.Cmd, _ system.Stream) (int, error) {
	call := r.answer(c)
	return call.output.ExitCode, call.err
}

func (r *Runner) Start(c system.Cmd, _ string) (int, error) {
	call := r.answer(c)
	if call.err != nil {
		return 0, call.err
	}
	pid := 1000 + len(r.Calls)
	r.Live[pid] = true
	return pid, nil
}

func (r *Runner) LookPath(name string) (string, bool) {
	path, ok := r.Paths[name]
	return path, ok
}

func (r *Runner) Alive(pid int) bool { return r.Live[pid] }

func (r *Runner) Signal(pid int, sig system.Sig) error {
	if !r.Live[pid] {
		return fmt.Errorf("no process %d", pid)
	}
	r.Signals = append(r.Signals, sig)
	delete(r.Live, pid)
	return nil
}
