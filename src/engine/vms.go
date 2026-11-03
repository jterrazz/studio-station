package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	StateRunning = "running"
	StateStopped = "stopped"
	StateStale   = "stale"
	StateBroken  = "broken"

	GuestConnected  = "connected"
	GuestConnecting = "connecting"
	GuestAbsent     = "absent"

	guestCommand = "/usr/local/bin/station guest"
	maxBackoff   = 30 * time.Second
)

type vmState struct {
	rec      machines.VMRecord
	pid      int
	state    string
	detail   string
	reruns   []time.Time
	rerunAt  time.Time
	guest    string
	ip       string
	stopping bool
	client   system.SSHClient
	cancel   context.CancelFunc
	in       io.WriteCloser
	wmu      sync.Mutex
	keys     string
	lastErr  string
	gen      int
	asked    map[string]chan guestMessage
	seq      int
}

func (d *Daemon) runsDir() string { return path.Join(d.o.Paths.State, "runs") }

func (d *Daemon) pidFile(vm string) string { return path.Join(d.runsDir(), vm+".pid") }

func (d *Daemon) exitFile(vm string) string { return path.Join(d.runsDir(), vm+".exit") }

func (d *Daemon) readPID(vm string) int {
	data, err := d.h.FS.ReadFile(d.pidFile(vm))
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

func (d *Daemon) readExit(vm string) int {
	data, err := d.h.FS.ReadFile(d.exitFile(vm))
	if err != nil {
		return -1
	}
	code, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return -1
	}
	return code
}

// invariant: at start, a live run is ADOPTED (never touched); a dead one or none
// leaves a temporary VM stale (a host reboot or a power cut ended it) and runs a
// lasting one its record wants running.
func (d *Daemon) adopt(ctx context.Context) {
	records, _ := d.m.Records()
	for _, r := range records {
		v := &vmState{rec: r, state: StateStopped, guest: GuestAbsent}
		d.mu.Lock()
		d.vms[r.Name] = v
		d.mu.Unlock()
		if pid := d.readPID(r.Name); pid > 0 && d.h.Run.Alive(pid) {
			v.pid, v.state = pid, StateRunning
			d.connect(ctx, v)
			continue
		}
		_ = d.h.FS.Remove(d.pidFile(r.Name))
		switch {
		case d.m.EngineRunning(ctx, r.Name):
			v.state = StateRunning
		case r.Lifetime == machines.Temporary:
			v.state = StateStale
		case r.Want != machines.WantStopped:
			_ = d.start(ctx, v)
		}
	}
}

func (d *Daemon) find(vm string) (*vmState, error) {
	d.mu.Lock()
	v, ok := d.vms[vm]
	d.mu.Unlock()
	if ok {
		return v, nil
	}
	records, _ := d.m.Records()
	for _, r := range records {
		if r.Name != vm {
			continue
		}
		v := &vmState{rec: r, state: StateStopped, guest: GuestAbsent}
		d.mu.Lock()
		if held, ok := d.vms[vm]; ok {
			v = held
		} else {
			d.vms[vm] = v
		}
		d.mu.Unlock()
		return v, nil
	}
	return nil, fault("unknown_vm", "unknown VM \""+vm+"\"", "station machines")
}

func (d *Daemon) start(ctx context.Context, v *vmState) error {
	name := v.rec.Name
	if records, err := d.m.Records(); err == nil {
		for _, r := range records {
			if r.Name == name {
				d.mu.Lock()
				v.rec = r
				d.mu.Unlock()
			}
		}
	}
	d.refreshSelf(ctx)
	d.renderEnv(v)
	console := d.m.ConsolePath(name)
	for _, dir := range []string{d.runsDir(), path.Dir(console), path.Dir(d.m.LogPath(name))} {
		if err := d.h.FS.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	if err := d.h.FS.WriteFile(console, nil, 0o600); err != nil {
		return err
	}
	_ = d.h.FS.Remove(d.exitFile(name))
	opts := system.RunOptions{}
	if v.rec.Network == machines.Bridged {
		opts.Bridged = d.defaultInterface(ctx)
	}
	argv := d.engine.RunArgv(name, console, opts)
	if abs, ok := d.h.Run.LookPath(argv[0]); ok {
		argv[0] = abs
	}
	pid, err := d.h.Run.Start(RunCmd(argv, d.exitFile(name)), d.m.LogPath(name))
	if err != nil {
		return err
	}
	if err := d.h.FS.WriteFile(d.pidFile(name), []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		return err
	}
	d.mu.Lock()
	v.pid, v.state, v.detail, v.stopping = pid, StateRunning, "", false
	d.mu.Unlock()
	d.connect(ctx, v)
	return nil
}

func (d *Daemon) defaultInterface(ctx context.Context) string {
	out, err := d.h.Run.Output(ctx, system.Cmd{Name: "route", Args: []string{"-n", "get", "default"}, Timeout: 5 * time.Second})
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

func (d *Daemon) tick(ctx context.Context) {
	records, _ := d.m.Records()
	known := map[string]bool{}
	for _, r := range records {
		known[r.Name] = true
		v, _ := d.find(r.Name)
		d.mu.Lock()
		v.rec = r
		ended := v.pid > 0 && !v.stopping && !d.h.Run.Alive(v.pid)
		later := !v.rerunAt.IsZero() && !d.h.Clock.Now().Before(v.rerunAt)
		d.mu.Unlock()
		if ended {
			d.ended(ctx, v)
		}
		if later {
			d.mu.Lock()
			v.rerunAt = time.Time{}
			d.mu.Unlock()
			_ = d.start(ctx, v)
		}
		d.rehello(v)
	}
	d.mu.Lock()
	for name, v := range d.vms {
		if !known[name] && !v.stopping {
			if v.cancel != nil {
				v.cancel()
			}
			delete(d.vms, name)
		}
	}
	d.mu.Unlock()
	d.tickServices(ctx)
}

func (d *Daemon) ended(ctx context.Context, v *vmState) {
	name := v.rec.Name
	code := d.readExit(name)
	intent := d.m.DeclaredPoweroff(d.m.ConsolePath(name))
	d.mu.Lock()
	act, reruns := machines.AfterRun(v.rec.Lifetime, intent, code, v.reruns, d.h.Clock.Now())
	v.reruns, v.pid = reruns, 0
	d.mu.Unlock()
	_ = d.h.FS.Remove(d.pidFile(name))
	d.disconnect(v)
	d.vmStopped(ctx, name)
	switch act {
	case machines.ActRerun:
		_ = d.start(ctx, v)
	case machines.ActRerunLater:
		d.mu.Lock()
		v.state, v.rerunAt = StateStopped, d.h.Clock.Now().Add(machines.CrashDelay)
		d.mu.Unlock()
	case machines.ActDelete:
		d.releaseVM(ctx, name)
		_, _ = d.m.Remove(ctx, name)
		d.forget(name)
	case machines.ActStopped:
		v.rec.Want = machines.WantStopped
		_ = d.m.Save(v.rec)
		d.setState(v, StateStopped, "")
	case machines.ActBroken:
		d.setState(v, StateBroken, machines.BrokenDetail())
	case machines.ActStale:
		d.setState(v, StateStale, "the run ended with exit "+strconv.Itoa(code))
	}
}

func (d *Daemon) setState(v *vmState, state, detail string) {
	d.mu.Lock()
	v.state, v.detail = state, detail
	d.mu.Unlock()
}

func (d *Daemon) forget(name string) {
	d.mu.Lock()
	if v, ok := d.vms[name]; ok && v.cancel != nil {
		v.cancel()
	}
	delete(d.vms, name)
	d.mu.Unlock()
}

func (d *Daemon) vmViews() []VMView {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]VMView, 0, len(d.vms))
	for name, v := range d.vms {
		out = append(out, VMView{Name: name, State: v.state, RunPID: v.pid, Guest: v.guest, IP: v.ip, Detail: v.detail})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func lasting(v *vmState) error {
	if v.rec.Lifetime != machines.Lasting {
		return fault("temporary", v.rec.Name+" is temporary", "station machines create --lifetime temporary")
	}
	return nil
}

func (d *Daemon) opRun(ctx context.Context, vm string) error {
	v, err := d.find(vm)
	if err != nil {
		return err
	}
	d.mu.Lock()
	running := v.pid > 0 && d.h.Run.Alive(v.pid)
	d.mu.Unlock()
	if running {
		return nil
	}
	return d.start(ctx, v)
}

// invariant: run answers once the guest is connected (it has written the VM's
// keys block and services.env), or when the run ended first (its end is the
// supervision's), else guest_timeout after guestReady.
func (d *Daemon) awaitGuest(ctx context.Context, vm string) error {
	v, err := d.find(vm)
	if err != nil {
		return err
	}
	for waited := time.Duration(0); waited < guestReady; waited += pollEvery {
		d.mu.Lock()
		connected, ended := v.guest == GuestConnected, v.pid == 0 || !d.h.Run.Alive(v.pid)
		d.mu.Unlock()
		if connected || ended {
			return nil
		}
		if err := d.h.Clock.Sleep(ctx, pollEvery); err != nil {
			return err
		}
	}
	return fault("guest_timeout", vm+"'s guest did not connect in "+strconv.Itoa(int(guestReady.Seconds()))+" s", "station machines probe "+vm)
}

func (d *Daemon) opStart(ctx context.Context, vm string) error {
	v, err := d.find(vm)
	if err != nil {
		return err
	}
	if err := lasting(v); err != nil {
		return err
	}
	v.rec.Want = machines.WantRunning
	if err := d.m.Save(v.rec); err != nil {
		return err
	}
	d.mu.Lock()
	v.reruns, v.rerunAt = nil, time.Time{}
	d.mu.Unlock()
	return d.opRun(ctx, vm)
}

// invariant: a graceful stop is the guest's own poweroff, sent on the VM's one
// ssh connection (the NAT ssh when it is down); the hard stop comes only after
// the wait; the run's end is the daemon's own, so it is never read as a reboot.
func (d *Daemon) opStop(ctx context.Context, vm string, want bool) error {
	v, err := d.find(vm)
	if err != nil {
		return err
	}
	if err := lasting(v); err != nil {
		return err
	}
	if want {
		v.rec.Want = machines.WantStopped
		if err := d.m.Save(v.rec); err != nil {
			return err
		}
	}
	d.mu.Lock()
	v.stopping = true
	pid, client := v.pid, v.client
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		v.stopping = false
		d.mu.Unlock()
	}()
	if pid == 0 && !d.m.EngineRunning(ctx, vm) {
		d.disconnect(v)
		d.setState(v, StateStopped, "")
		return nil
	}
	sent := false
	if client != nil {
		_, err := client.Exec(ctx, "sudo systemctl --no-block poweroff", nil, io.Discard, io.Discard)
		sent = err == nil
	}
	if !sent {
		if pid == 0 {
			if err := d.m.PowerOff(ctx, vm); err != nil {
				return err
			}
		} else {
			go func() { _ = d.m.PowerOff(context.WithoutCancel(ctx), vm) }()
		}
	}
	for waited := time.Duration(0); pid > 0 && d.h.Run.Alive(pid) && waited < stopWait; waited += pollEvery {
		if err := d.h.Clock.Sleep(ctx, pollEvery); err != nil {
			return err
		}
	}
	if pid > 0 && d.h.Run.Alive(pid) {
		_ = d.engine.Stop(ctx, vm, hardWait)
	}
	_ = d.h.FS.Remove(d.pidFile(vm))
	d.mu.Lock()
	v.pid = 0
	d.mu.Unlock()
	d.disconnect(v)
	d.vmStopped(ctx, vm)
	d.setState(v, StateStopped, "")
	return nil
}

func (d *Daemon) opDelete(ctx context.Context, vm string) error {
	v, err := d.find(vm)
	if err != nil {
		return err
	}
	d.mu.Lock()
	v.stopping = true
	d.mu.Unlock()
	d.vmStopped(ctx, vm)
	d.releaseVM(ctx, vm)
	_, err = d.m.Remove(ctx, vm)
	d.forget(vm)
	return err
}

func (d *Daemon) connect(ctx context.Context, v *vmState) {
	d.mu.Lock()
	if v.cancel != nil {
		d.mu.Unlock()
		return
	}
	gctx, cancel := context.WithCancel(ctx)
	v.gen++
	v.cancel, v.guest = cancel, GuestConnecting
	gen := v.gen
	d.mu.Unlock()
	go d.session(gctx, v, gen)
}

// invariant: a run that ended takes its guest session with it, so the next
// run's guest is awaited afresh: a stopped VM's ssh connection may outlive it,
// and a session it kept would answer for the new run before its sshd is up.
func (d *Daemon) disconnect(v *vmState) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if v.cancel != nil {
		v.cancel()
	}
	v.gen++
	v.cancel, v.guest, v.client, v.in = nil, GuestAbsent, nil, nil
}

func (d *Daemon) current(v *vmState, gen int) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return v.gen == gen
}

func (d *Daemon) alive(v *vmState) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return v.pid > 0 && !v.stopping && d.h.Run.Alive(v.pid)
}

func (d *Daemon) session(ctx context.Context, v *vmState, gen int) {
	defer func() {
		d.mu.Lock()
		if v.gen == gen {
			v.cancel, v.guest, v.client, v.in = nil, GuestAbsent, nil, nil
		}
		d.mu.Unlock()
	}()
	backoff := time.Second
	for d.alive(v) {
		d.guestOnce(ctx, v, gen)
		if !d.current(v, gen) {
			return
		}
		d.vmStopped(ctx, v.rec.Name)
		if ctx.Err() != nil || d.h.Clock.Sleep(ctx, backoff) != nil {
			return
		}
		if backoff *= 2; backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (d *Daemon) guestOnce(ctx context.Context, v *vmState, gen int) {
	name := v.rec.Name
	ip, err := d.engine.IP(ctx, name, 5*time.Second)
	if err != nil {
		d.guestFailed(ctx, v, "no ip: "+err.Error())
		return
	}
	client, err := d.ssh.Dial(ctx, system.SSHTarget{
		Addr: hostPort(ip, d.o.GuestPort), User: d.m.VMUser(),
		KeyPath: d.m.KeyPath(), KnownHosts: d.m.KnownHostsPath(name), Name: name,
	})
	if err != nil {
		d.guestFailed(ctx, v, err.Error())
		return
	}
	defer func() { _ = client.Close() }()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	d.mu.Lock()
	if v.gen != gen {
		d.mu.Unlock()
		return
	}
	v.client, v.ip, v.in, v.guest = client, ip, inW, GuestConnecting
	d.mu.Unlock()
	go func() {
		_, _ = client.Exec(ctx, guestCommand, inR, outW, io.Discard)
		_ = outW.Close()
		_ = inR.Close()
	}()
	go func() {
		select {
		case <-ctx.Done():
		case <-client.Done():
		}
		_ = outW.Close()
	}()
	d.hello(v)
	scanner := bufio.NewScanner(outR)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		d.fromGuest(ctx, v, scanner.Bytes())
	}
	_ = inW.Close()
}

// invariant: why a guest does not connect reaches daemon.log once per new
// reason, so a guest stuck connecting is never a silent log.
func (d *Daemon) guestFailed(ctx context.Context, v *vmState, reason string) {
	if ctx.Err() != nil {
		return
	}
	d.mu.Lock()
	repeated := v.lastErr == reason
	v.lastErr = reason
	d.mu.Unlock()
	if !repeated {
		d.logf(v.rec.Name+" guest", reason)
	}
}

func (d *Daemon) write(v *vmState, msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	v.wmu.Lock()
	defer v.wmu.Unlock()
	d.mu.Lock()
	in := v.in
	d.mu.Unlock()
	if in != nil {
		_, _ = in.Write(append(data, '\n'))
	}
}

func (d *Daemon) hello(v *vmState) {
	p := d.policy()
	keys := d.hostKeys()
	d.mu.Lock()
	v.keys = strings.Join(keys, "\n")
	public := services.ExposureOf(d.self.ServeOn, d.self.DNSName).PublicHost
	d.mu.Unlock()
	env := v.rec.Env
	if env == nil {
		env = map[string]string{}
	}
	d.write(v, map[string]any{
		"v": 1, "type": "hello", "host": d.m.HereName(), "vm": v.rec.Name, "public_host": public,
		"keys": keys, "env": env, "ignored": p.Ignored, "min_port": p.MinPort,
	})
}

func (d *Daemon) rehello(v *vmState) {
	d.mu.Lock()
	connected, was := v.guest == GuestConnected, v.keys
	d.mu.Unlock()
	if connected && strings.Join(d.hostKeys(), "\n") != was {
		d.hello(v)
	}
}

type guestMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id"`
	OK      bool            `json:"ok"`
	Result  json.RawMessage `json:"result"`
	Error   *OpError        `json:"error"`
	Op      string          `json:"op"`
	Port    int             `json:"port"`
	IP      string          `json:"ip"`
	Process string          `json:"process"`
	Ports   []struct {
		Port    int    `json:"port"`
		IP      string `json:"ip"`
		Process string `json:"process"`
	} `json:"ports"`
}

func (d *Daemon) fromGuest(ctx context.Context, v *vmState, line []byte) {
	var m guestMessage
	if ctx.Err() != nil || json.Unmarshal(line, &m) != nil {
		return
	}
	name := v.rec.Name
	switch m.Type {
	case "ready":
		d.recallBeforeReady(ctx, name)
		d.mu.Lock()
		v.guest, v.lastErr = GuestConnected, ""
		d.mu.Unlock()
	case "snapshot":
		seen := map[int]bool{}
		for _, p := range m.Ports {
			seen[p.Port] = true
			d.listen(ctx, name, p.Port, p.IP, p.Process)
		}
		d.unlistenExcept(name, seen)
	case "listen":
		d.listen(ctx, name, m.Port, m.IP, m.Process)
	case "unlisten":
		d.unlisten(name, m.Port)
	case "reply":
		d.mu.Lock()
		ch := v.asked[m.ID]
		delete(v.asked, m.ID)
		d.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	case "request":
		result, err := d.guestRequest(ctx, name, m)
		reply := map[string]any{"v": 1, "type": "reply", "id": m.ID, "ok": err == nil}
		if err != nil {
			var op *OpError
			if !errors.As(err, &op) {
				op = fault("internal", err.Error(), "station services")
			}
			reply["error"] = op
		} else {
			reply["result"] = result
		}
		d.write(v, reply)
	}
}

func (d *Daemon) guestRequest(ctx context.Context, vm string, m guestMessage) (any, error) {
	switch m.Op {
	case "services":
		return d.hostView(vm), nil
	case "reserve":
		return d.reserve(ctx, vm, m.Port, false, "")
	case "release":
		return d.release(ctx, vm, m.Port)
	}
	return nil, fault("unknown_op", "unknown op \""+m.Op+"\"", "station services")
}

func (d *Daemon) askGuest(ctx context.Context, v *vmState, op string, budget time.Duration) (json.RawMessage, error) {
	ch := make(chan guestMessage, 1)
	d.mu.Lock()
	if v.asked == nil {
		v.asked = map[string]chan guestMessage{}
	}
	v.seq++
	id := "h" + strconv.Itoa(v.seq)
	v.asked[id] = ch
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		delete(v.asked, id)
		d.mu.Unlock()
	}()
	d.write(v, map[string]any{"v": 1, "type": "request", "id": id, "op": op})
	wait, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	select {
	case m := <-ch:
		if !m.OK {
			if m.Error != nil {
				return nil, m.Error
			}
			return nil, fault("internal", v.rec.Name+"'s guest refused "+op, "station machines probe "+v.rec.Name)
		}
		return m.Result, nil
	case <-wait.Done():
		return nil, fault("guest_timeout", v.rec.Name+"'s guest did not answer "+op+" in "+budget.String(), "station machines probe "+v.rec.Name)
	}
}

func hostPort(ip string, port int) string {
	if strings.Contains(ip, ":") {
		return "[" + ip + "]:" + strconv.Itoa(port)
	}
	return ip + ":" + strconv.Itoa(port)
}

// invariant: a VM's profile environment is rendered at each of its runs from
// Serve's state read then, never at an adoption, so it holds for the whole of a run
// and a lasting VM picks up the public URLs at its first start after Serve
// turns on.
func (d *Daemon) renderEnv(v *vmState) {
	p := d.policy()
	d.mu.Lock()
	e := services.ExposureOf(d.self.ServeOn, d.self.DNSName)
	var reserved []machines.Reserved
	for _, pr := range catalog.Profiles {
		if l, i := services.Find(d.leases, v.rec.Name, pr.Port); i >= 0 {
			view := services.ViewOf(l, e, p.HTTPS, services.Waiting)
			reserved = append(reserved, machines.Reserved{GuestPort: l.GuestPort, HostPort: l.HostPort, Profile: pr.Name, URL: view.URLs.Public, PublicHost: e.PublicHost})
		}
	}
	rec := v.rec
	d.mu.Unlock()
	if len(reserved) == 0 {
		return
	}
	env := machines.ProfileEnv(reserved)
	if maps.Equal(env, rec.Env) {
		return
	}
	rec.Env = env
	if d.m.Save(rec) == nil {
		d.mu.Lock()
		v.rec = rec
		d.mu.Unlock()
	}
}
