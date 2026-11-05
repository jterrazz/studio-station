package accounts

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	probe "github.com/jterrazz/studio-station/src/core/call/lanes"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	KindHost = accounts.KindHost
	KindVM   = accounts.KindVM

	DaemonRunning   = "running"
	DaemonStopped   = "stopped"
	DaemonAbsent    = "not installed"
	GuestConnected  = "connected"
	GuestAbsent     = "absent"
	expiringWithin  = time.Hour
	daemonBudget    = 13 * time.Second
	providerReserve = 500 * time.Millisecond
)

type (
	Machine = accounts.Machine
	Report  = accounts.Report
)

type VM struct {
	// value: a VM of this host as its record and the daemon's list say.
	Name, Lifetime string
	Running        bool
	Lends          []Lend
}

type VMRead struct {
	Name     string    `json:"name"`
	Guest    string    `json:"guest"`
	Accounts []Account `json:"accounts"`
}

type TailnetAccount struct {
	State            catalog.State
	Login, Tailnet   string
	Variant, Problem string
}

type Ports struct {
	VMs      func(ctx context.Context) ([]VM, error)
	Daemon   func(ctx context.Context) ([]VMRead, string)
	Tailnet  func(ctx context.Context) TailnetAccount
	LendList func() []string
}

type Options struct {
	Here     catalog.MachineKind
	HereName string
	// value: in a VM, the guest's hello keys: who may ssh in.
	Keys func() []string
}

type Service struct {
	h    system.Host
	pool *probe.Pool
	ids  contract.Identities
	p    Ports
	o    Options
}

func New(h system.Host, pool *probe.Pool, ids contract.Identities, p Ports, o Options) *Service {
	return &Service{h: h, pool: pool, ids: ids, p: p, o: o}
}

func (s *Service) where() string {
	if s.o.Here == catalog.Host {
		return KindHost
	}
	return KindVM
}

func (s *Service) providers() []catalog.Provider {
	var out []catalog.Provider
	for _, p := range catalog.Providers {
		if s.where() == KindHost && !p.Host || s.where() == KindVM && !p.VM {
			continue
		}
		if !s.ids.Offers(p) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func (s *Service) Own(ctx context.Context) []Account {
	ps := s.providers()
	out := make([]Account, len(ps))
	var wg sync.WaitGroup
	for i, p := range ps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = s.read(ctx, p)
		}()
	}
	wg.Wait()
	return out
}

func (s *Service) read(ctx context.Context, p catalog.Provider) Account {
	var acc Account
	switch p.ID {
	case catalog.ProviderTailscale:
		acc = s.tailnet(ctx)
	case catalog.ProviderEntry:
		acc = s.entry()
	default:
		id, err := s.ids.Read(ctx, p)
		if err != nil {
			id = accounts.Broken(p.ID, ReasonFailing, err.Error())
		}
		acc = own(id)
	}
	return Redact(acc)
}

func (s *Service) tailnet(ctx context.Context) Account {
	if s.p.Tailnet == nil {
		return absent(catalog.ProviderTailscale, ReasonMissing, "not installed")
	}
	t := s.p.Tailnet(ctx)
	switch {
	case t.State == catalog.StateAbsent:
		return absent(catalog.ProviderTailscale, ReasonMissing, "not installed")
	case t.State == catalog.StateBroken:
		return Account{Provider: catalog.ProviderTailscale, State: catalog.StateBroken, Reason: ReasonFailing, Origin: OriginOwn, Identity: []string{Unknown}, Detail: "tailscale status did not answer"}
	case t.Problem != "Running":
		return absent(catalog.ProviderTailscale, ReasonUnset, "not logged in")
	case t.Login == "":
		t.Login = Unknown
	}
	acc := ok(catalog.ProviderTailscale)
	acc.Identity, acc.Detail = []string{t.Login}, t.Variant
	if t.Tailnet != "" {
		acc.Scopes = []string{"tailnet " + t.Tailnet}
	}
	return acc
}

func (s *Service) entry() Account {
	var keys []string
	if s.o.Keys != nil {
		keys = s.o.Keys()
	}
	acc := ok(catalog.ProviderEntry)
	n := 0
	for _, k := range keys {
		fields := strings.Fields(k)
		if len(fields) < 2 {
			continue
		}
		n++
		if comment := strings.Join(fields[2:], " "); comment != "" && !slices.Contains(acc.Scopes, comment) {
			acc.Scopes = append(acc.Scopes, comment)
		}
		if fp, _, found := accounts.Fingerprint(k); found && acc.Detail == "" {
			acc.Detail = fp
		}
	}
	if n == 0 {
		return absent(catalog.ProviderEntry, ReasonUnset, "no host key")
	}
	acc.Identity = []string{strconv.Itoa(n) + " host keys"}
	if n == 1 {
		acc.Identity = []string{"1 host key"}
	}
	return acc
}

func (s *Service) Read(ctx context.Context, withVMs bool) <-chan Report {
	now := s.h.Clock.Now().UTC()
	here := Machine{Name: s.o.HereName, Kind: s.where(), Here: true}
	ps := s.providers()
	for _, p := range ps {
		here.Accounts = append(here.Accounts, Account{Provider: p.ID, State: catalog.StatePending})
	}
	frame := Report{Machines: []Machine{here}, Now: now}
	var vms []VM
	if s.where() == KindHost && s.p.VMs != nil {
		vms, _ = s.p.VMs(ctx)
		slices.SortStableFunc(vms, func(a, b VM) int {
			if a.Running != b.Running {
				if a.Running {
					return -1
				}
				return 1
			}
			return 0
		})
		for _, v := range vms {
			frame.Machines = append(frame.Machines, s.vmFrame(v, now))
		}
	}
	jobs := make([]probe.Job[Report], 0, len(ps)+1)
	for i, p := range ps {
		jobs = append(jobs, probe.Job[Report]{
			Key: "accounts/" + p.ID, Lane: probe.Commands, Budget: p.Budget + providerReserve,
			Do: func(ctx context.Context) func(*Report) {
				acc := withHint(s.read(ctx, p), "")
				return func(r *Report) { r.Machines[0].Accounts[i] = acc }
			},
			Late: func(r *Report) { r.Machines[0].Accounts[i] = withHint(own(accounts.Late(p)), "") },
		})
	}
	if withVMs && len(vms) > 0 && s.p.Daemon != nil {
		jobs = append(jobs, probe.Job[Report]{
			Key: "accounts/vms", Lane: probe.Inline, Budget: daemonBudget,
			Do: func(ctx context.Context) func(*Report) {
				reads, daemon := s.p.Daemon(ctx)
				return func(r *Report) { s.merge(r, vms, reads, daemon) }
			},
			Late: func(r *Report) { s.merge(r, vms, nil, DaemonRunning) },
		})
	}
	if len(jobs) == 0 {
		out := make(chan Report, 1)
		out <- frame
		close(out)
		return out
	}
	return probe.Run(ctx, s.pool, frame, jobs, Report.Clone)
}

func (s *Service) vmFrame(v VM, now time.Time) Machine {
	m := Machine{Name: v.Name, Kind: KindVM, Lifetime: v.Lifetime, Running: v.Running, Pending: v.Running}
	m.Accounts = lentRows(v, nil, false, now)
	return m
}

func (s *Service) merge(r *Report, vms []VM, reads []VMRead, daemon string) {
	r.Daemon = daemon
	byName := map[string]VMRead{}
	for _, v := range reads {
		byName[v.Name] = v
	}
	for i := range r.Machines {
		m := &r.Machines[i]
		if m.Kind != KindVM || m.Here {
			continue
		}
		var v VM
		for _, x := range vms {
			if x.Name == m.Name {
				v = x
			}
		}
		m.Pending = false
		read, asked := byName[m.Name]
		switch {
		case !v.Running:
			m.Guest = ""
		case asked:
			m.Guest = read.Guest
		default:
			m.Guest = GuestAbsent
		}
		m.Accounts = lentRows(v, read.Accounts, m.Guest == GuestAbsent, r.Now)
	}
}

// invariant: a running VM whose guest is absent keeps its rows from its
// record: its lent rows whole, every other provider of a VM read `?`.
func lentRows(v VM, own []Account, unreachable bool, now time.Time) []Account {
	var out []Account
	for _, p := range catalog.Providers {
		if !p.VM {
			continue
		}
		var acc *Account
		for i := range own {
			if own[i].Provider == p.ID {
				a := own[i]
				acc = &a
			}
		}
		lend := lendFor(v.Lends, p.ID)
		switch {
		case acc == nil && lend == nil && unreachable && p.ID != catalog.ProviderDocker:
			a := Account{Provider: p.ID, State: catalog.StateDegraded, Reason: ReasonUnreachable, Origin: OriginOwn, Identity: []string{Unknown}, Detail: "guest not connected"}
			acc = &a
		case acc == nil && lend == nil:
			continue
		case acc == nil:
			a := Account{Provider: p.ID, State: catalog.StateOK, Origin: OriginLent, Identity: []string{Unknown}}
			acc = &a
		case lend != nil && acc.State == catalog.StateOK:
			acc.Origin = OriginBoth
		case lend != nil:
			acc.State, acc.Reason, acc.Origin, acc.Detail, acc.Identity = catalog.StateOK, "", OriginLent, "", []string{Unknown}
		}
		if lend != nil {
			l := *lend
			acc.Lent = &l
			acc.Scopes = append([]string{l.Name}, acc.Scopes...)
			switch left := l.Until.Sub(now); {
			case left <= 0:
				acc.State, acc.Reason, acc.Detail = catalog.StateBroken, ReasonFailing, "expired, not recalled"
			case left <= expiringWithin:
				acc.State, acc.Reason = catalog.StateDegraded, ReasonExpiring
			}
		}
		out = append(out, withHint(*acc, v.Name))
	}
	return out
}

func lendFor(lends []Lend, provider string) *Lend {
	for _, l := range lends {
		for _, sc := range catalog.LendScopes {
			if sc.Name == l.Name && sc.Provider == provider {
				return &l
			}
		}
	}
	return nil
}

func (s *Service) Forget() {
	if s.pool != nil {
		s.pool.Forget("accounts/")
	}
}

func (s *Service) LendList() []string {
	if s.p.LendList == nil {
		return nil
	}
	return s.p.LendList()
}

func withHint(a Account, vm string) Account {
	a.Hint = ""
	if a.Lent != nil {
		switch a.Reason {
		case ReasonExpiring:
			a.Hint = "station accounts share " + accounts.AccountOf(a.Lent.Name) + " --with " + vm
		case ReasonFailing:
			a.Hint = "station accounts take-back " + accounts.AccountOf(a.Lent.Name) + " --from " + vm
		}
		return a
	}
	switch a.Provider + "/" + a.Reason {
	case "github/unset":
		a.Hint = "station settings apply github-login"
		if vm != "" {
			a.Hint = "ssh " + vm + " gh auth login"
		}
	case "github/missing":
		if vm == "" {
			a.Hint = "station apps install gh"
		}
	case "npm/unset", "npm/refused":
		a.Hint = "npm login"
	case "claude/unset":
		if vm == "" {
			a.Hint = "claude /login"
		}
	case "cloudflare/unset":
		a.Hint = "wrangler login"
	case "git/unset":
		a.Hint = "git config --global user.name <name>"
	}
	return a
}

func RowID(machine, provider string) string { return accounts.RowID(machine, provider) }
