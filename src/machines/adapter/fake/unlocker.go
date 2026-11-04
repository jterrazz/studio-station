package fake

import (
	"context"
	"sync"

	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/world"
)

type MacState int

const (
	Answering MacState = iota
	AtPreBoot
	Offline
	Refusing
)

type Macs struct {
	// invariant: the Macs of the world as their pre-boot ssh and their sshd
	// see them: one locked waits at pre-boot, reached on its LAN even off the
	// tailnet; one off the tailnet and unlocked is not reached; an unlock
	// lets it boot, a planned restart reboots it.
	w        *world.World
	mu       sync.Mutex
	refusal  map[string]string
	sessions map[string]int
	restarts map[string]int
}

func MacsOf(w *world.World) *Macs {
	return &Macs{w: w, refusal: map[string]string{}, sessions: map[string]int{}, restarts: map[string]int{}}
}

func NewMacs() *Macs { return MacsOf(&world.World{}) }

func (m *Macs) Set(name string, s MacState) {
	m.w.Lock()
	defer m.w.Unlock()
	mac := m.w.Mac(name)
	mac.Locked, mac.Offline = s == AtPreBoot, s == Offline
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.refusal, name)
}

func (m *Macs) Refuse(name, detail string) {
	m.Set(name, Answering)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refusal[name] = detail
}

func (m *Macs) Sessions(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[name]
}

func (m *Macs) Restarts(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.restarts[name]
}

func (m *Macs) state(name string) MacState {
	mac, ok := m.w.Macs[name]
	switch {
	case !ok:
		return Offline
	case mac.Locked:
		return AtPreBoot
	case mac.Offline:
		return Offline
	}
	return Answering
}

func (m *Macs) Unlock(_ context.Context, name string) error {
	m.w.Lock()
	defer m.w.Unlock()
	switch m.state(name) {
	case Offline:
		return contract.ErrUnreachable
	case AtPreBoot:
		m.w.Macs[name].Locked, m.w.Macs[name].Offline = false, false
	case Answering, Refusing:
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[name]++
	return nil
}

func (m *Macs) PlannedRestart(_ context.Context, name string) error {
	m.w.Lock()
	defer m.w.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if detail, ok := m.refusal[name]; ok {
		return &contract.RestartRefusedError{Detail: detail}
	}
	if m.w.Fails("Unlocker.PlannedRestart") {
		return &contract.RestartRefusedError{Detail: "refused, as the world scripted it"}
	}
	if m.state(name) != Answering {
		// invariant: as ssh does, a restart sent to a Mac that does not answer
		// reads as sent; the caller learns otherwise when it never goes down.
		return nil
	}
	m.restarts[name]++
	return m.w.Apply(world.Event{Name: "reboot", Args: []string{name}})
}
