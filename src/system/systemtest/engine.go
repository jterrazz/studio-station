package systemtest

import (
	"context"
	"fmt"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

type Engine struct {
	Tart  string
	VMs   []system.VMInfo
	IPs   map[string]string
	Err   map[string]error
	Calls []string
}

func NewEngine() *Engine {
	return &Engine{Tart: "/opt/tart", IPs: map[string]string{}, Err: map[string]error{}}
}

func (e *Engine) record(verb string, args ...any) error {
	e.Calls = append(e.Calls, fmt.Sprint(append([]any{verb}, args...)...))
	return e.Err[verb]
}

func (e *Engine) find(name string) int {
	for i, vm := range e.VMs {
		if vm.Name == name {
			return i
		}
	}
	return -1
}

func (e *Engine) Version(context.Context) (string, error) {
	return "2.41.1", e.record("version")
}

func (e *Engine) List(context.Context) ([]system.VMInfo, error) {
	return append([]system.VMInfo{}, e.VMs...), e.record("list")
}

func (e *Engine) Pull(_ context.Context, ref string) error { return e.record("pull", " ", ref) }

func (e *Engine) Clone(_ context.Context, from, to string) error {
	if err := e.record("clone", " ", from, " ", to); err != nil {
		return err
	}
	e.VMs = append(e.VMs, system.VMInfo{Name: to, State: system.VMStopped})
	return nil
}

func (e *Engine) Rename(_ context.Context, from, to string) error {
	if err := e.record("rename", " ", from, " ", to); err != nil {
		return err
	}
	if i := e.find(from); i >= 0 {
		e.VMs[i].Name = to
	}
	return nil
}

func (e *Engine) Resize(_ context.Context, name string, r system.Resources) error {
	return e.record("resize", " ", name, " ", r.CPUs, " ", r.MemoryBytes, " ", r.DiskBytes)
}

func (e *Engine) RunArgv(name, console string) []string {
	return []string{e.Tart, "run", "--no-graphics", "--no-audio", "--serial-path", console, name}
}

func (e *Engine) Stop(_ context.Context, name string, _ time.Duration) error {
	if err := e.record("stop", " ", name); err != nil {
		return err
	}
	if i := e.find(name); i >= 0 {
		e.VMs[i].State = system.VMStopped
	}
	return nil
}

func (e *Engine) Delete(_ context.Context, name string) error {
	if err := e.record("delete", " ", name); err != nil {
		return err
	}
	if i := e.find(name); i >= 0 {
		e.VMs = append(e.VMs[:i], e.VMs[i+1:]...)
	}
	return nil
}

func (e *Engine) IP(_ context.Context, name string, _ time.Duration) (string, error) {
	if err := e.record("ip", " ", name); err != nil {
		return "", err
	}
	ip, ok := e.IPs[name]
	if !ok {
		return "", system.ErrTimeout
	}
	return ip, nil
}
