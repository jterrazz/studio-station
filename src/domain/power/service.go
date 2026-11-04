package power

import (
	"context"
	"errors"
	"time"

	"github.com/jterrazz/studio-station/src/awake/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	DaemonRunning = "running"
	DaemonStopped = "stopped"
	DaemonAbsent  = "not installed"
	DaemonPending = "pending"
)

var ErrDaemonDown = errors.New("the station daemon does not answer")

type Report struct {
	Now time.Time
	// value: the zone a page shows its clock times in (the owner's, read once
	// by the caller); nil is UTC.
	Zone    *time.Location
	Daemon  string
	Reading Reading
	Session *Session
	LastEnd *End
	Guards  []Guard
}

type Awoken struct {
	Session
	Refreshed bool `json:"refreshed"`
}

type Ports struct {
	// value: the daemon's power op; ErrDaemonDown when it does not answer.
	View  func(ctx context.Context) (View, error)
	Awake func(ctx context.Context, r Request) (Awoken, error)
	Sleep func(ctx context.Context) (Slept, error)
	// value: whether the daemon's agent plist is on this host.
	Installed func() bool
}

type ServiceOptions struct {
	Power    Port
	Here     catalog.MachineKind
	HereName string
	Zone     *time.Location
}

type Service struct {
	h system.Host
	p Ports
	o ServiceOptions
}

type OffHostError struct{}

func (*OffHostError) Error() string { return "Power runs on a host; here is a VM" }

type DaemonDownError struct {
	Host      string
	Installed bool
}

func (e *DaemonDownError) Error() string {
	if e.Installed {
		return "the station daemon is stopped on " + e.Host
	}
	return "the station daemon is not installed on " + e.Host
}

func NewService(h system.Host, p Ports, o ServiceOptions) *Service { return &Service{h: h, p: p, o: o} }

func (s *Service) Check() error {
	if s.o.Here != catalog.Host {
		return &OffHostError{}
	}
	if s.o.Power == nil {
		return &UnsupportedError{}
	}
	if _, err := s.o.Power.Read(); errors.Is(err, contract.ErrUnsupported) {
		return &UnsupportedError{}
	}
	return nil
}

func (s *Service) down() error {
	return &DaemonDownError{Host: s.o.HereName, Installed: s.p.Installed()}
}

func (s *Service) daemonState() string {
	if s.p.Installed() {
		return DaemonStopped
	}
	return DaemonAbsent
}

func (s *Service) Read(ctx context.Context) <-chan Report {
	// invariant: the reading is the CLI's own, at once and with the daemon
	// stopped; only the session is the daemon's, so the first report holds the
	// reading and a pending session.
	out := make(chan Report, 2)
	go func() {
		defer close(out)
		r, _ := s.o.Power.Read()
		first := Report{Now: s.h.Clock.Now(), Zone: s.o.Zone, Daemon: DaemonPending, Reading: r, Guards: Guards(r, nil)}
		out <- first
		v, err := s.p.View(ctx)
		if ctx.Err() != nil {
			return
		}
		last := first
		last.Now = s.h.Clock.Now()
		if err != nil {
			last.Daemon = s.daemonState()
		} else {
			last.Daemon, last.Session, last.LastEnd = DaemonRunning, v.Session, v.LastEnd
			last.Guards = Guards(r, v.Session)
		}
		out <- last
	}()
	return out
}

func (s *Service) Awake(ctx context.Context, r Request) (Awoken, error) {
	if r.For == 0 {
		r.For = Default
	}
	if err := CheckFor(r.For); err != nil {
		return Awoken{}, err
	}
	a, err := s.p.Awake(ctx, r)
	if errors.Is(err, ErrDaemonDown) {
		return a, s.down()
	}
	return a, err
}

func (s *Service) Sleep(ctx context.Context) (Slept, error) {
	sl, err := s.p.Sleep(ctx)
	if errors.Is(err, ErrDaemonDown) {
		return sl, s.down()
	}
	return sl, err
}

func (s *Service) Now() time.Time { return s.h.Clock.Now() }

func (s *Service) Zone() *time.Location { return s.o.Zone }
