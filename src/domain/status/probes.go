package status

import (
	"context"
	"errors"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	schedule "github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/system"
)

const ReadBudget = defaultBudget

const (
	defaultBudget = 2 * time.Second
	updateBudget  = 10 * time.Second
	repoBudget    = 5 * time.Second
	scanBudget    = 20 * time.Second
	ghBudget      = 5 * time.Second
	commandLane   = 16
)

type reply struct {
	out   system.Output
	found bool
	late  bool
}

func (r reply) ok() bool { return r.found && !r.late && r.out.ExitCode == 0 }

func (r reply) text() string { return r.out.Stdout }

func (r reply) all() string { return r.out.Stdout + r.out.Stderr }

type probe struct {
	name      string
	cmd       system.Cmd
	budget    time.Duration
	shared    bool
	interpret func(name string, r reply) Check
	read      func(ctx context.Context, budget time.Duration) Check
}

func (s *Service) ask(ctx context.Context, c system.Cmd, budget time.Duration) reply {
	return s.asked(ctx, c, budget, false)
}

// invariant: a shared command (gh auth status, read by config too) runs once per
// run of the pool, so one reading never asks the network twice.
func (s *Service) asked(ctx context.Context, c system.Cmd, budget time.Duration, shared bool) reply {
	if budget <= 0 {
		budget = defaultBudget
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	c.Timeout = budget
	var out system.Output
	var err error
	if shared {
		out, err = schedule.Shared(ctx, s.o.Pool, schedule.Command(c.Name, c.Args), func(ctx context.Context) (system.Output, error) { return s.h.Run.Output(ctx, c) })
	} else {
		out, err = s.h.Run.Output(ctx, c)
	}
	switch {
	case errors.Is(err, system.ErrTimeout), errors.Is(ctx.Err(), context.DeadlineExceeded):
		return reply{late: true}
	case errors.Is(err, system.ErrNotFound):
		return reply{}
	case err != nil:
		return reply{found: true, out: system.Output{ExitCode: -1}}
	}
	return reply{found: true, out: out}
}

// invariant: a probe past its budget answers unknown, whatever its table says.
func (s *Service) probe(ctx context.Context, p probe) Check {
	budget := p.budget
	if budget <= 0 {
		budget = defaultBudget
	}
	if p.read != nil {
		return p.read(ctx, budget)
	}
	r := s.asked(ctx, p.cmd, budget, p.shared)
	if r.late {
		return late(p.name, budget)
	}
	return p.interpret(p.name, r)
}

func late(name string, budget time.Duration) Check {
	return Check{Name: name, State: catalog.StateUnknown, Value: "?", Detail: "no answer in " + budget.String()}
}

func cmd(name string, args ...string) system.Cmd { return system.Cmd{Name: name, Args: args} }
