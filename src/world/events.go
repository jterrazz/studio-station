package world

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/machines"
)

type Event struct {
	Name string
	Args []string
}

type event struct {
	name  string
	args  []string
	apply func(w *World, args []string) error
}

var ErrNotModelled = errors.New("this world does not model it yet")

// invariant: the closed list of world events (USE-CASES §0.2); an event the
// fakes do not model yet is known, and refused as such.
func events() []event {
	return []event{
		{"reboot", []string{"<mac>"}, reboot},
		{"power-cut", []string{"<mac>"}, powerCut},
		{"unlock", []string{"<mac>"}, unlock},
		{"offline", []string{"<mac>"}, offline(true)},
		{"online", []string{"<mac>"}, offline(false)},
		{"engine-crash", []string{"<mac>"}, engineCrash},
		{"crash", []string{"<vm>"}, ended(machines.RunEnd{Reason: machines.Crashed, ExitCode: 1})},
		{"poweroff-inside", []string{"<vm>"}, ended(machines.RunEnd{Reason: machines.GuestPoweroff})},
		{"reboot-inside", []string{"<vm>"}, ended(machines.RunEnd{Reason: machines.GuestReboot})},
		{"listen", []string{"<machine>", "<bind>:<port>", "<process>"}, nil},
		{"unlisten", []string{"<machine>", "<port>"}, nil},
		{"edit", []string{"<machine>", "<file>"}, nil},
		{"unplug", nil, nil},
		{"plug", nil, nil},
		{"close-lid", nil, nil},
		{"open-lid", nil, nil},
		{"thermal", []string{"<level>"}, nil},
		{"battery", []string{"<percent>"}, nil},
		{"low-power", []string{"on|off"}, nil},
		{"advance", []string{"<duration>"}, advance},
		{"fault", []string{"<Contract>.<Method>", "[<n> times]"}, nil},
		{"upgrade", []string{"<mac>", "<version>"}, upgrade},
		{"address", []string{"<vm>", "<ip>"}, address},
		{"slow", []string{"<machine>", "<duration>"}, nil},
		{"certificate", []string{"<mac>", "pending|issued"}, nil},
	}
}

func Names() []string {
	out := make([]string, 0, len(events()))
	for _, e := range events() {
		out = append(out, e.name)
	}
	return out
}

func Parse(args []string) (Event, error) {
	if len(args) == 0 {
		return Event{}, fmt.Errorf("no world event: one of %s", strings.Join(Names(), ", "))
	}
	i := slices.IndexFunc(events(), func(e event) bool { return e.name == args[0] })
	if i < 0 {
		return Event{}, fmt.Errorf("unknown world event %q: one of %s", args[0], strings.Join(Names(), ", "))
	}
	e := events()[i]
	need := 0
	for _, a := range e.args {
		if !strings.HasPrefix(a, "[") {
			need++
		}
	}
	if len(args)-1 < need || len(args)-1 > len(e.args) {
		return Event{}, fmt.Errorf("world %s takes %s", e.name, strings.Join(e.args, " "))
	}
	return Event{Name: e.name, Args: args[1:]}, nil
}

func (w *World) Apply(ev Event) error {
	i := slices.IndexFunc(events(), func(e event) bool { return e.name == ev.Name })
	if i < 0 {
		return fmt.Errorf("unknown world event %q", ev.Name)
	}
	if events()[i].apply == nil {
		return fmt.Errorf("world %s: %w", ev.Name, ErrNotModelled)
	}
	return events()[i].apply(w, ev.Args)
}

func (w *World) known(mac string) (*Mac, error) {
	if _, ok := w.Macs[mac]; !ok {
		return nil, fmt.Errorf("no Mac %q in this world", mac)
	}
	return w.Mac(mac), nil
}

func (w *World) vm(name string) (*VM, error) {
	mac := w.Holder(name)
	if mac == "" {
		return nil, fmt.Errorf("no VM %q in this world", name)
	}
	return w.Macs[mac].VMs[name], nil
}

// invariant: a Mac that goes down ends every run on it with nobody to read
// how; its engine starts again at login, and finds no run to adopt.
func reboot(w *World, args []string) error {
	m, err := w.known(args[0])
	if err != nil {
		return err
	}
	for _, v := range m.VMs {
		v.PID, v.Ended = 0, nil
	}
	return nil
}

func powerCut(w *World, args []string) error {
	if err := reboot(w, args); err != nil {
		return err
	}
	w.Macs[args[0]].Locked = true
	return nil
}

func unlock(w *World, args []string) error {
	m, err := w.known(args[0])
	if err != nil {
		return err
	}
	m.Locked = false
	return nil
}

func offline(off bool) func(w *World, args []string) error {
	return func(w *World, args []string) error {
		m, err := w.known(args[0])
		if err != nil {
			return err
		}
		m.Offline = off
		return nil
	}
}

// invariant: the engine's job manager starts a crashed engine again: the
// runs it supervised outlive it and are adopted.
func engineCrash(w *World, args []string) error {
	m, err := w.known(args[0])
	if err != nil {
		return err
	}
	if m.Engine == nil {
		return fmt.Errorf("no engine runs on %s", args[0])
	}
	return nil
}

func ended(end machines.RunEnd) func(w *World, args []string) error {
	return func(w *World, args []string) error {
		v, err := w.vm(args[0])
		if err != nil {
			return err
		}
		if !v.Running() {
			return fmt.Errorf("%s does not run", args[0])
		}
		v.PID, v.Ended = 0, &end
		return nil
	}
}

func advance(w *World, args []string) error {
	d, err := time.ParseDuration(args[0])
	if err != nil {
		return fmt.Errorf("world advance: %w", err)
	}
	w.Now = w.Now.Add(d)
	return nil
}

func upgrade(w *World, args []string) error {
	m, err := w.known(args[0])
	if err != nil {
		return err
	}
	if m.Engine == nil {
		m.Engine = &Engine{}
	}
	m.Engine.Version = args[1]
	return nil
}

func address(w *World, args []string) error {
	v, err := w.vm(args[0])
	if err != nil {
		return err
	}
	v.Address = args[1]
	return nil
}
