package calls

import (
	"context"
	"errors"
	"time"

	"github.com/jterrazz/studio-station/src/awake"

	"github.com/jterrazz/studio-station/src/awake/contract"
	"github.com/jterrazz/studio-station/src/catalog"
)

type ServicePorts struct {
	// value: the daemon's power op; awake.ErrDaemonDown when it does not answer.
	View  func(ctx context.Context) (awake.View, error)
	Awake func(ctx context.Context, r awake.Request) (awake.Awoken, error)
	Sleep func(ctx context.Context) (awake.Slept, error)
	// value: whether the daemon's agent plist is on this host.
	Installed func() bool
}

type ServiceOptions struct {
	Power       contract.Power
	ThisMachine catalog.MachineKind
	ThisMacName string
	Zone        *time.Location
}

type Service struct {
	clock Clock
	p     ServicePorts
	o     ServiceOptions
}

func NewService(clock Clock, p ServicePorts, o ServiceOptions) *Service {
	return &Service{clock: clock, p: p, o: o}
}

func (s *Service) Check() error {
	if s.o.ThisMachine != catalog.Mac {
		return &awake.OffMacError{}
	}
	if s.o.Power == nil {
		return &awake.UnsupportedError{}
	}
	if _, err := s.o.Power.Read(); errors.Is(err, contract.ErrUnsupported) {
		return &awake.UnsupportedError{}
	}
	return nil
}

func (s *Service) down() error {
	return &awake.EngineDownError{Mac: s.o.ThisMacName, Installed: s.p.Installed()}
}

func (s *Service) engineState() string {
	if s.p.Installed() {
		return awake.EngineStopped
	}
	return awake.EngineAbsent
}

func (s *Service) Read(ctx context.Context) <-chan awake.Report {
	// invariant: the reading is the CLI's own, at once and with the daemon
	// stopped; only the session is the daemon's, so the first report holds the
	// reading and a pending session.
	out := make(chan awake.Report, 2)
	go func() {
		defer close(out)
		r, _ := s.o.Power.Read()
		first := awake.Report{Now: s.clock.Now(), Zone: s.o.Zone, Engine: awake.EnginePending, Reading: r, Guards: awake.GuardStates(r, nil)}
		out <- first
		v, err := s.p.View(ctx)
		if ctx.Err() != nil {
			return
		}
		last := first
		last.Now = s.clock.Now()
		if err != nil {
			last.Engine = s.engineState()
		} else {
			last.Engine, last.Session, last.LastEnd = awake.DaemonRunning, v.Session, v.LastEnd
			last.NeverSleeps, last.NeverSince, last.NeverHeld = v.NeverSleeps, v.NeverSince, v.NeverHeld
			last.Guards = awake.GuardStates(r, v.Session)
		}
		out <- last
	}()
	return out
}

func (s *Service) Awake(ctx context.Context, r awake.Request) (awake.Awoken, error) {
	if r.For == 0 {
		r.For = awake.DefaultFor
	}
	if err := awake.CheckFor(r.For); err != nil {
		return awake.Awoken{}, err
	}
	a, err := s.p.Awake(ctx, r)
	if errors.Is(err, awake.ErrEngineDown) {
		return a, s.down()
	}
	return a, err
}

func (s *Service) Sleep(ctx context.Context) (awake.Slept, error) {
	sl, err := s.p.Sleep(ctx)
	if errors.Is(err, awake.ErrEngineDown) {
		return sl, s.down()
	}
	return sl, err
}

func (s *Service) Now() time.Time { return s.clock.Now() }

func (s *Service) Zone() *time.Location { return s.o.Zone }
