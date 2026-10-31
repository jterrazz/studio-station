package status

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	defaultBudget = 2 * time.Second
	updateBudget  = 10 * time.Second
	repoBudget    = 5 * time.Second
	scanBudget    = 20 * time.Second
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
	interpret func(name string, r reply) Check
}

func (s *Service) ask(ctx context.Context, c system.Cmd, budget time.Duration) reply {
	if budget <= 0 {
		budget = defaultBudget
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	c.Timeout = budget
	out, err := s.h.Run.Output(ctx, c)
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
	r := s.ask(ctx, p.cmd, budget)
	if r.late {
		return Check{Name: p.name, State: catalog.StateUnknown, Detail: "no answer in " + budget.String()}
	}
	return p.interpret(p.name, r)
}

func (s *Service) probes(ctx context.Context, table []probe) []Check {
	checks := make([]Check, len(table))
	var wg sync.WaitGroup
	for i, p := range table {
		wg.Add(1)
		go func() {
			defer wg.Done()
			checks[i] = s.probe(ctx, p)
		}()
	}
	wg.Wait()
	return checks
}

func cmd(name string, args ...string) system.Cmd { return system.Cmd{Name: name, Args: args} }
