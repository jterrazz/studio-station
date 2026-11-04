package tart

import (
	"context"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/machines"
	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	poweroffIntent = "station-intent=poweroff"
	watchEvery     = 200 * time.Millisecond
)

type files struct{ state string }

func (f files) runs() string             { return path.Join(f.state, "runs") }
func (f files) pid(vm string) string     { return path.Join(f.runs(), vm+".pid") }
func (f files) exit(vm string) string    { return path.Join(f.runs(), vm+".exit") }
func (f files) console(vm string) string { return path.Join(f.state, "consoles", vm) }
func (f files) log(vm string) string     { return path.Join(f.state, "logs", vm+".log") }
func (f files) all(vm string) []string   { return []string{f.console(vm), f.pid(vm), f.exit(vm)} }
func (f files) drop(fs system.FS, vm string) {
	for _, file := range f.all(vm) {
		_ = fs.Remove(file)
	}
}

func (f files) number(fs system.FS, file string) (int, bool) {
	data, err := fs.ReadFile(file)
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return n, err == nil
}

// invariant: a run is detached from the engine, and its exit code is written by
// its own wrapper, so an engine restart reads the end of a run it never waited on.
func wrapped(argv []string, exitFile string) system.Cmd {
	return system.Cmd{Name: "sh", Args: append([]string{"-c", `"$@"; echo "$?" >"` + exitFile + `"`, "sh"}, argv...)}
}

func (h *hypervisor) Boot(ctx context.Context, vm string, o contract.BootOptions) (contract.Run, error) {
	console := h.files.console(vm)
	for _, dir := range []string{h.files.runs(), path.Dir(console), path.Dir(h.files.log(vm))} {
		if err := h.fs.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	// invariant: tart opens --serial-path for reading and writing and refuses to run when the file is missing.
	if err := h.fs.WriteFile(console, nil, 0o600); err != nil {
		return nil, err
	}
	_ = h.fs.Remove(h.files.exit(vm))
	argv := []string{h.path, "run", "--no-graphics", "--no-audio", "--serial-path", console}
	if o.Network.Bridged {
		if iface := h.defaultInterface(ctx); iface != "" {
			argv = append(argv, "--net-bridged="+iface)
		}
	}
	pid, err := h.run.Start(wrapped(append(argv, vm), h.files.exit(vm)), h.files.log(vm))
	if err != nil {
		return nil, err
	}
	if err := h.fs.WriteFile(h.files.pid(vm), []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return nil, err
	}
	return h.watch(ctx, vm, pid), nil
}

func (h *hypervisor) Adopt(ctx context.Context, vm string) (contract.Run, bool) {
	if pid, ok := h.files.number(h.fs, h.files.pid(vm)); ok && pid > 0 && h.run.Alive(pid) {
		return h.watch(ctx, vm, pid), true
	}
	_ = h.fs.Remove(h.files.pid(vm))
	return nil, false
}

func (h *hypervisor) defaultInterface(ctx context.Context) string {
	out, err := h.run.Output(ctx, system.Cmd{Name: "route", Args: []string{"-n", "get", "default"}, Timeout: 5 * time.Second})
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(out.Stdout, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(line), ":"); ok && k == "interface" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

type run struct {
	vm    string
	pid   int
	ended chan machines.RunEnd
}

func (r *run) VM() string                    { return r.vm }
func (r *run) PID() int                      { return r.pid }
func (r *run) Ended() <-chan machines.RunEnd { return r.ended }

func (h *hypervisor) watch(ctx context.Context, vm string, pid int) *run {
	r := &run{vm: vm, pid: pid, ended: make(chan machines.RunEnd, 1)}
	watching := context.WithoutCancel(ctx)
	go func() {
		for h.run.Alive(pid) {
			if h.clock.Sleep(watching, watchEvery) != nil {
				return
			}
		}
		end := h.endOf(vm)
		if held, ok := h.files.number(h.fs, h.files.pid(vm)); ok && held == pid {
			_ = h.fs.Remove(h.files.pid(vm))
		}
		r.ended <- end
		close(r.ended)
	}()
	return r
}

// invariant: a poweroff is the guest's own word on its serial console, whatever
// the exit code; a clean exit without it is a reboot; an exit code nobody wrote
// is an end station cannot read, which never deletes a VM.
func (h *hypervisor) endOf(vm string) machines.RunEnd {
	code, ok := h.files.number(h.fs, h.files.exit(vm))
	if !ok {
		code = -1
	}
	if h.declaredPoweroff(vm) {
		return machines.RunEnd{Reason: machines.GuestPoweroff, ExitCode: code}
	}
	switch {
	case !ok:
		return machines.RunEnd{Reason: machines.Unknown, ExitCode: code}
	case code == 0:
		return machines.RunEnd{Reason: machines.GuestReboot}
	}
	return machines.RunEnd{Reason: machines.Crashed, ExitCode: code}
}

func (h *hypervisor) declaredPoweroff(vm string) bool {
	data, err := h.fs.ReadFile(h.files.console(vm))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == poweroffIntent {
			return true
		}
	}
	return false
}
