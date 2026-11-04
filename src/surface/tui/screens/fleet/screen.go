package fleetscreen

import (
	"context"
	"strconv"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
	"github.com/jterrazz/studio-station/src/machines"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagefleet "github.com/jterrazz/studio-station/src/pages/fleet"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	portscalls "github.com/jterrazz/studio-station/src/ports/calls"
	"github.com/jterrazz/studio-station/src/surface/tui/screens/tab"
)

const ID = "fleet"

type Remote func(ctx context.Context, machine string) error

type Deps struct {
	Calls  call.Router
	Target machine.Ref
	// value: opens station's own screen on that machine, the terminal handed to it.
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
				l, err := call.Read(ctx, d.Calls, d.Target, machinescalls.List, machinescalls.ListIn{Unwritten: true}, nil)
				if err != nil {
					send(fail("fleet", err))
					return
				}
				send(func(r *pagefleet.Reading) { r.Machines = &l.Fleet })
			})
			run(func() {
				list, err := call.Invoke(ctx, d.Calls, d.Target, imagescalls.List, imagescalls.ListIn{}, nil)
				if err != nil {
					send(fail("images", err))
					return
				}
				send(func(r *pagefleet.Reading) { r.Images = &list })
			})
			run(func() {
				f, err := call.Invoke(ctx, d.Calls, d.Target, portscalls.List, portscalls.ListIn{}, nil)
				if err == nil {
					send(func(r *pagefleet.Reading) { r.Services = &f })
				}
			})
			run(func() {
				s, err := call.Invoke(ctx, d.Calls, d.Target, portscalls.Tailnet, portscalls.TailnetIn{}, nil)
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
		m.View, act = m.Act(m.exec(func() (tab.Status, error) {
			records, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, imagescalls.Build, imagescalls.BuildIn{}, nil)
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
		m.View, act = m.Act(tab.ConfirmedInPlace(ID, pagefleet.DeleteConfirm(name), func(answer string) (tab.Status, error) {
			report, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, machinescalls.Delete, machinescalls.DeleteIn{VM: name, Confirm: answer}, nil)
			if len(report.Warnings) > 0 {
				return tab.Outcome("deleted", name, catalog.StateDegraded, report.Warnings[0]), err
			}
			return tab.Outcome("deleted", name, catalog.StateOK, ""), err
		}))
	case pagefleet.KeyForget:
		m.View, act = m.Act(tab.ConfirmedInPlace(ID, pagefleet.ForgetConfirm(name), func(answer string) (tab.Status, error) {
			_, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, machinescalls.Forget, machinescalls.ForgetIn{Name: name, Confirm: answer}, nil)
			return tab.Outcome("forgot", name, catalog.StateOK, ""), err
		}))
	case pagemachines.KeyRestart:
		restart := tab.ConfirmedInPlace
		if m.isFarHost(name) {
			restart = tab.Confirmed
		}
		m.View, act = m.Act(restart(ID, pagefleet.RestartConfirm(name, m.isHost(name)), func(answer string) (tab.Status, error) {
			_, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, machinescalls.Restart, machinescalls.RestartIn{Name: name, Confirm: answer}, nil)
			return tab.Outcome("restarted", name, catalog.StateOK, ""), err
		}))
	case pagemachines.KeyStart:
		if m.running(name) {
			m.View, act = m.Act(tab.ConfirmedInPlace(ID, page.Confirm{Question: "Stop " + name + "? It is kept, stopped, until you start it", Yes: "stop"}, func(answer string) (tab.Status, error) {
				_, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, machinescalls.Stop, machinescalls.StopIn{VM: name, Confirm: answer}, nil)
				return tab.Outcome("stopped", name, catalog.StateOK, ""), err
			}))
			break
		}
		m.View, act = m.Act(m.exec(func() (tab.Status, error) {
			_, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, machinescalls.Start, machinescalls.VMIn{VM: name}, nil)
			return tab.Outcome("started", name, catalog.StateOK, ""), err
		}))
	case pagefleet.KeyProbe:
		m.View, act = m.Act(m.exec(func() (tab.Status, error) {
			report, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, machinescalls.Probe, machinescalls.ProbeIn{Name: name}, nil)
			if err != nil {
				return tab.Status{}, err
			}
			return probed(report), nil
		}))
	case pagefleet.KeyUnlock:
		m.View, act = m.Act(tab.Exec(ID, func() (tab.Status, error) {
			_, err := call.Invoke(m.ctx, m.d.Calls, m.d.Target, machinescalls.Unlock, machinescalls.UnlockIn{Name: name, Interactive: true}, nil)
			return tab.Outcome("unlocked", name, catalog.StateOK, ""), err
		}))
	case pagemachines.KeyEnter:
		m.View, act = m.Act(tab.Exec(ID, func() (tab.Status, error) {
			return tab.Outcome("back from", name, catalog.StateOK, ""), m.d.Remote(m.ctx, name)
		}))
	}
	return m, tea.Batch(cmd, act)
}

func (m Model) exec(action func() (tab.Status, error)) tea.Cmd { return tab.InPlace(ID, action) }

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

// value: a far Mac restarts over ssh, which may ask its password on the terminal.
func (m Model) isFarHost(name string) bool {
	if f := m.Last.Machines; f != nil {
		for _, h := range f.Hosts {
			if h.Name == name {
				return !h.Here
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
	target := m.d.Target
	if host := values["host"]; host != "" && host != here {
		target = machine.Ref{Name: host}
	}
	request := machines.CreateRequest{Image: values["image"], Name: values["name"], Lifetime: machines.Lifetime(values["lifetime"])}
	return m.exec(func() (tab.Status, error) {
		created, err := call.Invoke(m.ctx, m.d.Calls, target, machinescalls.CreateWorkspace, machinescalls.CreateIn{Request: request}, nil)
		return tab.Outcome("created", created.VM.Name, catalog.StateOK, "on "+created.VM.Host), err
	})
}
