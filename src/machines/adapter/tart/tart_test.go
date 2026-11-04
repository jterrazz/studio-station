package tart

import (
	"context"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/machines/contract/contracttest"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

const (
	bin   = "/home/o/.local/bin/tart"
	state = "/s"
)

type lockedFS struct {
	mu sync.Mutex
	*systemtest.FS
}

func (f *lockedFS) ReadFile(p string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.FS.ReadFile(p)
}

func (f *lockedFS) WriteFile(p string, data []byte, perm fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.FS.WriteFile(p, data, perm)
}

func (f *lockedFS) MkdirAll(p string, perm fs.FileMode) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.FS.MkdirAll(p, perm)
}

func (f *lockedFS) Remove(p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.FS.Remove(p)
}

type yielding struct{}

func (yielding) Now() time.Time { return time.Unix(0, 0) }

func (yielding) Sleep(ctx context.Context, _ time.Duration) error {
	runtime.Gosched()
	return ctx.Err()
}

func recorded(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type machine struct {
	run   *systemtest.Runner
	fs    *lockedFS
	disks map[string]string
}

func newMachine(t *testing.T) *machine {
	run := systemtest.NewRunner(t)
	run.Paths["tart"] = bin
	return &machine{run: run, fs: &lockedFS{FS: systemtest.NewFS()}, disks: map[string]string{}}
}

func (m *machine) hypervisor() contract.Hypervisor { return New(m.run, m.fs, yielding{}, state) }

func (m *machine) listing() system.Output {
	type row struct{ Source, Name, State string }
	rows := make([]row, 0, len(m.disks)+1)
	rows = append(rows, row{Source: "OCI", Name: contracttest.Image, State: "stopped"})
	for _, vm := range slices.Sorted(maps.Keys(m.disks)) {
		rows = append(rows, row{Source: "local", Name: vm, State: m.disks[vm]})
	}
	data, _ := json.Marshal(rows)
	return system.Output{Stdout: string(data)}
}

func (m *machine) boot(vm string, extra ...string) {
	argv := append([]string{bin, "run", "--no-graphics", "--no-audio", "--serial-path", state + "/consoles/" + vm}, extra...)
	m.run.On("sh", wrapped(append(argv, vm), state+"/runs/"+vm+".exit").Args...).Answer(func() system.Output {
		m.disks[vm] = "running"
		return system.Output{}
	})
}

func (m *machine) end(r contract.Run, how machines.EndReason) {
	code := "0"
	if how == machines.GuestPoweroff {
		_ = m.fs.WriteFile(state+"/consoles/"+r.VM(), []byte("[  OK  ] Reached target Power-Off.\nstation-intent=poweroff\n"), 0o600)
	}
	if how == machines.Crashed {
		code = "1"
	}
	_ = m.fs.WriteFile(state+"/runs/"+r.VM()+".exit", []byte(code+"\n"), 0o600)
	m.disks[r.VM()] = "stopped"
	m.run.Exit(r.PID())
}

func TestTartKeepsTheHypervisorContract(t *testing.T) {
	contracttest.Hypervisor(t, func(t *testing.T) contracttest.Subject {
		m := newMachine(t)
		vm, renamed := contracttest.VM, contracttest.Renamed
		m.run.On(bin, "pull", contracttest.Image)
		m.run.On(bin, "clone", contracttest.Image, vm).Answer(func() system.Output {
			m.disks[vm] = "stopped"
			return system.Output{}
		})
		m.run.On(bin, "set", vm, "--cpu", "4", "--memory", "8192", "--disk-size", "50")
		m.run.On(bin, "list", "--source", "local", "--format", "json").Answer(m.listing)
		m.boot(vm)
		m.run.On(bin, "ip", vm, "--wait", "5").Reply(system.Output{Stdout: "192.168.64.5\n"})
		m.run.On(bin, "rename", vm, renamed).Answer(func() system.Output {
			m.disks[renamed] = m.disks[vm]
			delete(m.disks, vm)
			return system.Output{}
		})
		m.run.On(bin, "delete", renamed).Answer(func() system.Output {
			delete(m.disks, renamed)
			return system.Output{}
		})
		return contracttest.Subject{Hypervisor: m.hypervisor(), End: m.end, Restart: m.hypervisor}
	})
}

func TestDisksKeepLocalVMsAndTheirSize(t *testing.T) {
	m := newMachine(t)
	m.run.On(bin, "list", "--source", "local", "--format", "json").Reply(system.Output{Stdout: recorded(t, "list.json")})
	disks, err := m.hypervisor().Disks(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := []contract.Disk{
		{VM: "station-image-ubuntu-1", Bytes: 7e9, State: contract.DiskStopped},
		{VM: "dev", Bytes: 12e9, State: contract.DiskRunning},
		{VM: "paused", Bytes: 9e9, State: contract.DiskSuspended},
	}
	if !slices.Equal(disks, want) {
		t.Fatalf("disks %+v, want %+v without the OCI entry", disks, want)
	}
}

func TestABridgedVMBootsOnTheMacsDefaultInterface(t *testing.T) {
	m := newMachine(t)
	m.run.On("route", "-n", "get", "default").Reply(system.Output{Stdout: "   route to: default\ndestination: default\n  interface: en7\n"})
	m.boot("dev", "--net-bridged=en7")
	r, err := m.hypervisor().Boot(t.Context(), "dev", contract.BootOptions{Network: machines.Network{Bridged: true}})
	if err != nil {
		t.Fatal(err)
	}
	m.end(r, machines.GuestPoweroff)
	<-r.Ended()
}

func TestTheEndOfAnOlderRunKeepsTheNewerRunAdoptable(t *testing.T) {
	m := newMachine(t)
	m.boot("dev")
	h := m.hypervisor()
	older, err := h.Boot(t.Context(), "dev", contract.BootOptions{})
	if err != nil {
		t.Fatal(err)
	}
	newer, err := h.Boot(t.Context(), "dev", contract.BootOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m.run.Exit(older.PID())
	<-older.Ended()
	if again, ok := m.hypervisor().Adopt(t.Context(), "dev"); !ok || again.PID() != newer.PID() {
		t.Errorf("after the older run ended, a restart adopted %v (ok %v), want the newer run", again, ok)
	}
	m.end(newer, machines.GuestReboot)
	<-newer.Ended()
}

func TestARunThatEndedWhileNoEngineWatchedIsNotAdopted(t *testing.T) {
	m := newMachine(t)
	_ = m.fs.WriteFile(state+"/runs/dev.pid", []byte("77\n"), 0o600)
	if _, ok := m.hypervisor().Adopt(t.Context(), "dev"); ok {
		t.Fatalf("a run whose process is gone was adopted")
	}
	if _, err := m.fs.ReadFile(state + "/runs/dev.pid"); err == nil {
		t.Errorf("the pid file of the ended run was kept")
	}
}

func TestAFailedCommandCarriesItsStderr(t *testing.T) {
	m := newMachine(t)
	m.run.On(bin, "delete", "dev").Reply(system.Output{ExitCode: 1, Stderr: "no such VM"})
	err := m.hypervisor().Delete(t.Context(), "dev")
	if err == nil || !strings.Contains(err.Error(), "no such VM") {
		t.Fatalf("err = %v", err)
	}
}
