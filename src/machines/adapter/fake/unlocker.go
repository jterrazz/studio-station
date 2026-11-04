package fake

import (
	"context"
	"sync"

	"github.com/jterrazz/studio-station/src/machines/contract"
)

type MacState int

const (
	Answering MacState = iota
	AtPreBoot
	Offline
	Refusing
)

type mac struct {
	state              MacState
	refusal            string
	sessions, restarts int
}

type Macs struct {
	mu   sync.Mutex
	macs map[string]*mac
}

func NewMacs() *Macs { return &Macs{macs: map[string]*mac{}} }

func (m *Macs) of(name string) *mac {
	if m.macs[name] == nil {
		m.macs[name] = &mac{state: Offline}
	}
	return m.macs[name]
}

func (m *Macs) Set(name string, s MacState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.of(name).state = s
}

func (m *Macs) Refuse(name, detail string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.of(name).state, m.of(name).refusal = Refusing, detail
}

func (m *Macs) Sessions(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.of(name).sessions
}

func (m *Macs) Restarts(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.of(name).restarts
}

func (m *Macs) Unlock(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	at := m.of(name)
	if at.state == Offline {
		return contract.ErrUnreachable
	}
	at.sessions++
	if at.state == AtPreBoot {
		at.state = Answering
	}
	return nil
}

func (m *Macs) PlannedRestart(_ context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	at := m.of(name)
	switch at.state {
	case Refusing:
		return &contract.RestartRefusedError{Detail: at.refusal}
	case Answering:
		at.restarts++
	case AtPreBoot, Offline:
		// invariant: as ssh does, a restart sent to a Mac that does not answer
		// reads as sent; the caller learns otherwise when it never goes down.
	}
	return nil
}
