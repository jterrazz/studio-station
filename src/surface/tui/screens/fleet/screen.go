package fleetscreen

import (
	"context"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/page"
	pagefleet "github.com/jterrazz/studio-station/src/pages/fleet"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "fleet"

type Machines interface {
	Fleet(ctx context.Context, hereOnly bool) (machines.Fleet, error)
	CheckCreate(ctx context.Context, r machines.CreateRequest) (machines.CreateRequest, error)
	Create(ctx context.Context, r machines.CreateRequest, progress func(machines.Step)) (machines.Created, error)
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Delete(ctx context.Context, name string) (machines.DeleteReport, error)
	ForgetHost(ctx context.Context, name string) error
	Restart(ctx context.Context, name string, progress func(machines.RestartEvent)) (machines.RestartReport, error)
	Probe(ctx context.Context, name string) (machines.ProbeReport, error)
	Unlock(ctx context.Context, name string) error
}

type Images interface {
	List(ctx context.Context) ([]images.Report, error)
	Build(ctx context.Context, r images.BuildRequest, progress func(images.Step)) ([]images.Record, error)
}

type Secrets interface {
	List(ctx context.Context) (secrets.Report, error)
}

type Services interface {
	List(ctx context.Context, hereOnly bool) (services.Fleet, error)
}

type Tailnet interface {
	Status(ctx context.Context) (ports.Status, error)
}

type Remote func(ctx context.Context, machine string, words []string) error

type Deps struct {
	Machines Machines
	Images   Images
	Secrets  Secrets
	Services Services
	Tailnet  Tailnet
	Remote   Remote
	Describe tab.Describe
}

type Model struct {
	tab.View[pagefleet.Reading]
	ctx context.Context
	d   Deps
}

func read(d Deps) tab.Source[pagefleet.Reading] {
	return func(ctx context.Context, _ bool) (<-chan pagefleet.Reading, error) {
		out := make(chan pagefleet.Reading)
		go func() {
			defer close(out)
			var mu sync.Mutex
			r := pagefleet.Reading{Failed: map[string]page.Error{}}
			send := func(change func(*pagefleet.Reading)) {
				mu.Lock()
				change(&r)
				snap := r
				snap.Failed = make(map[string]page.Error, len(r.Failed))
				for k, v := range r.Failed {
					snap.Failed[k] = v
				}
				mu.Unlock()
				select {
				case out <- snap:
				case <-ctx.Done():
				}
			}
			fail := func(part string, err error) func(*pagefleet.Reading) {
				message, hint := d.Describe(err)
				return func(r *pagefleet.Reading) { r.Failed[part] = page.Error{Message: message, Hint: hint} }
			}
			var wg sync.WaitGroup
			run := func(f func()) {
				wg.Add(1)
				go func() { defer wg.Done(); f() }()
			}
			run(func() {
				f, err := d.Machines.Fleet(ctx, false)
				if err != nil {
					send(fail("fleet", err))
					return
				}
				send(func(r *pagefleet.Reading) { r.Machines = &f })
			})
			run(func() {
				list, err := d.Images.List(ctx)
				if err != nil {
					send(fail("images", err))
					return
				}
				send(func(r *pagefleet.Reading) { r.Images = &list })
			})
			run(func() {
				s, err := d.Secrets.List(ctx)
				if err != nil {
					send(fail("secrets", err))
					return
				}
				send(func(r *pagefleet.Reading) { r.Secrets = &s })
			})
			run(func() {
				f, err := d.Services.List(ctx, false)
				if err == nil {
					send(func(r *pagefleet.Reading) { r.Services = &f })
				}
			})
			run(func() {
				s, err := d.Tailnet.Status(ctx)
				if err != nil {
					send(fail("tailnet", err))
					return
				}
				send(func(r *pagefleet.Reading) { r.Tailnet = &s })
			})
			wg.Wait()
		}()
		return out, nil
	}
}

func New(ctx context.Context, d Deps, here page.Context) Model {
	build := func(r pagefleet.Reading) page.Page { return pagefleet.Tab(r, here) }
	return Model{View: tab.NewView(ID, "Fleet", "hosts", here, read(d), build, d.Describe), ctx: ctx, d: d}
}

func (m Model) Update(msg tea.Msg) (tab.Screen, tea.Cmd) {
	view, cmd, ev := m.View.Update(msg)
	m.View = view
	if ev == nil {
		return m, cmd
	}
	var act tea.Cmd
	name := ev.Row
	switch ev.Action.Key {
	case pagemachines.KeyCreate:
		m.View, act = m.Fill(pagefleet.CreateForm(m.hosts(), imageNames()), func(values map[string]string) tea.Cmd {
			return func() tea.Msg { return tab.Begin(ID, m.create(values)) }
		})
	case pagefleet.KeyBuild:
		m.View, act = m.Act(nil, m.exec(func() (tab.Status, error) {
			records, err := m.d.Images.Build(m.ctx, images.BuildRequest{}, nil)
			if err != nil {
				return tab.Status{}, err
			}
			names := make([]string, 0, len(records))
			for _, r := range records {
				names = append(names, r.Name+" "+r.Version)
			}
			return tab.Outcome("built", strings.Join(names, ", "), catalog.StateOK, ""), nil
		}))
	case pagemachines.KeyDelete:
		c := pagefleet.DeleteConfirm(name)
		m.View, act = m.Act(&c, m.exec(func() (tab.Status, error) {
			report, err := m.d.Machines.Delete(m.ctx, name)
			if len(report.Warnings) > 0 {
				return tab.Outcome("deleted", name, catalog.StateDegraded, report.Warnings[0]), err
			}
			return tab.Outcome("deleted", name, catalog.StateOK, ""), err
		}))
	case pagefleet.KeyForget:
		c := pagefleet.ForgetConfirm(name)
		m.View, act = m.Act(&c, m.exec(func() (tab.Status, error) {
			return tab.Outcome("forgot", name, catalog.StateOK, ""), m.d.Machines.ForgetHost(m.ctx, name)
		}))
	case pagemachines.KeyRestart:
		c := pagefleet.RestartConfirm(name, m.isHost(name))
		m.View, act = m.Act(&c, m.exec(func() (tab.Status, error) {
			_, err := m.d.Machines.Restart(m.ctx, name, nil)
			return tab.Outcome("restarted", name, catalog.StateOK, ""), err
		}))
	case pagemachines.KeyStart:
		running := m.running(name)
		m.View, act = m.Act(nil, m.exec(func() (tab.Status, error) {
			if running {
				return tab.Outcome("stopped", name, catalog.StateOK, ""), m.d.Machines.Stop(m.ctx, name)
			}
			return tab.Outcome("started", name, catalog.StateOK, ""), m.d.Machines.Start(m.ctx, name)
		}))
	case pagefleet.KeyProbe:
		m.View, act = m.Act(nil, m.exec(func() (tab.Status, error) {
			report, err := m.d.Machines.Probe(m.ctx, name)
			if err != nil {
				return tab.Status{}, err
			}
			return probed(report), nil
		}))
	case pagefleet.KeyUnlock:
		m.View, act = m.Act(nil, m.exec(func() (tab.Status, error) {
			return tab.Outcome("unlocked", name, catalog.StateOK, ""), m.d.Machines.Unlock(m.ctx, name)
		}))
	case pagemachines.KeyEnter:
		m.View, act = m.Act(nil, m.exec(func() (tab.Status, error) {
			return tab.Outcome("back from", name, catalog.StateOK, ""), m.d.Remote(m.ctx, name, nil)
		}))
	}
	return m, tea.Batch(cmd, act)
}

func (m Model) exec(action func() (tab.Status, error)) tea.Cmd { return tab.Exec(ID, action) }

// value: the status bar names the worst of the probe's checks, or that all answered.
func probed(r machines.ProbeReport) tab.Status {
	for _, want := range []catalog.State{catalog.StateBroken, catalog.StateDegraded, catalog.StateAbsent} {
		for _, c := range r.Checks {
			if c.State == want {
				return tab.Outcome("probed", r.Name, catalog.StateDegraded, c.Name+" "+c.Value)
			}
		}
	}
	return tab.Outcome("probed", r.Name, catalog.StateOK, strconv.Itoa(len(r.Checks))+" checks answered")
}

func (m Model) isHost(name string) bool {
	if f := m.Last.Machines; f != nil {
		for _, h := range f.Hosts {
			if h.Name == name {
				return true
			}
		}
	}
	return false
}

func (m Model) running(name string) bool {
	if f := m.Last.Machines; f != nil {
		for _, h := range f.Hosts {
			for _, vm := range h.VMs {
				if vm.Name == name {
					return vm.State == machines.Running
				}
			}
		}
	}
	return false
}

func (m Model) hosts() []pagemachines.HostChoice {
	var out []pagemachines.HostChoice
	if f := m.Last.Machines; f != nil {
		for _, h := range f.Hosts {
			if h.Reachable {
				out = append(out, pagemachines.HostChoice{Name: h.Name, Here: h.Here})
			}
		}
	}
	return out
}

func imageNames() []string {
	names := make([]string, 0, len(catalog.Images))
	for _, img := range catalog.Images {
		names = append(names, img.Name)
	}
	return names
}

func (m Model) create(values map[string]string) tea.Cmd {
	here := ""
	for _, h := range m.hosts() {
		if h.Here {
			here = h.Name
		}
	}
	host := values["host"]
	if host != "" && host != here {
		words := []string{"machines", "create", "--lifetime", values["lifetime"], "--image", values["image"]}
		if values["name"] != "" {
			words = append(words, "--name", values["name"])
		}
		return m.exec(func() (tab.Status, error) {
			return tab.Outcome("created a VM on", host, catalog.StateOK, ""), m.d.Remote(m.ctx, host, words)
		})
	}
	request := machines.CreateRequest{Image: values["image"], Name: values["name"], Lifetime: machines.Lifetime(values["lifetime"])}
	return m.exec(func() (tab.Status, error) {
		resources, err := machines.ParseResources(0, "", "")
		if err != nil {
			return tab.Status{}, err
		}
		request.Resources = resources
		checked, err := m.d.Machines.CheckCreate(m.ctx, request)
		if err != nil {
			return tab.Status{}, err
		}
		created, err := m.d.Machines.Create(m.ctx, checked, nil)
		return tab.Outcome("created", created.VM.Name, catalog.StateOK, "on "+created.VM.Host), err
	})
}
