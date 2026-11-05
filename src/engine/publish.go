package engine

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/system"
)

const issuingDetail = "issuing certificate"

const (
	bannerWait = 300 * time.Millisecond
	headWait   = 2 * time.Second
	busyRetry  = 10 * time.Second
)

type svc struct {
	vm        string
	port      int
	ip        string
	process   string
	st        services.State
	listeners []system.Listener
	served    bool
	detail    string
	serveNote string
	refused   bool
	retryAt   time.Time
}

func key(vm string, port int) string { return vm + ":" + strconv.Itoa(port) }

func (d *Daemon) leaseIndex(vm string, port int) int {
	_, i := services.Find(d.leases, vm, port)
	return i
}

func (d *Daemon) busy(p int) bool {
	l, err := d.h.Sockets.ListenTCP("127.0.0.1", p)
	if err != nil {
		return true
	}
	_ = l.Close()
	return false
}

func asOp(err error) error {
	var full *ports.RangeFull
	var perVM *ports.VMFull
	switch {
	case errors.As(err, &full):
		return fault("range_full", err.Error(), "station ports stop-sharing <vm> <port>")
	case errors.As(err, &perVM):
		return perVM.Refusal()
	}
	return fault("internal", err.Error(), "station ports")
}

func (d *Daemon) svcOf(vm string, port int) *svc {
	s, ok := d.svcs[key(vm, port)]
	if !ok {
		s = &svc{vm: vm, port: port}
		if i := d.leaseIndex(vm, port); i >= 0 {
			s.st.Pinned = d.leases[i].Pinned
			s.st.LastSeenAt = cmp.Or(d.leases[i].LastSeenAt, d.leases[i].ReservedAt)
			s.st.Phase = services.Waiting
		}
		d.svcs[key(vm, port)] = s
	}
	return s
}

func (d *Daemon) apply(ctx context.Context, s *svc, acts []services.Action) {
	for _, a := range acts {
		switch a {
		case services.ActPublish:
			d.publish(ctx, s)
		case services.ActUnpublish:
			d.unpublish(ctx, s)
		case services.ActRelease:
			if i := d.leaseIndex(s.vm, s.port); i >= 0 {
				d.leases = append(d.leases[:i:i], d.leases[i+1:]...)
				d.saveLeases()
			}
			delete(d.svcs, key(s.vm, s.port))
		}
	}
}

// invariant: a workspace's port is shared as it starts listening; a
// server's only once the owner shares it, its tooling choosing its routes.
func (d *Daemon) listen(ctx context.Context, vm string, port int, ip, process string) {
	p := d.policy()
	if p.Ignores(port) || p.Stopped(vm, port) {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if v := d.vms[vm]; v != nil && v.rec.Lifetime == machines.AlwaysOn && d.leaseIndex(vm, port) < 0 {
		return
	}
	s := d.svcOf(vm, port)
	s.ip, s.process = ip, process
	now := d.h.Clock.Now().UTC().Truncate(time.Second)
	if s.st.Phase == services.Refused {
		s.st.Phase = services.None
	}
	var acts []services.Action
	s.st, acts = services.Next(s.st, services.EvListen, now)
	if i := d.leaseIndex(vm, port); i >= 0 && now.Sub(d.leases[i].LastSeenAt) >= seenEvery {
		d.leases[i].LastSeenAt = now
		d.saveLeases()
	}
	d.apply(ctx, s, acts)
}

func (d *Daemon) unlisten(vm string, port int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if s, ok := d.svcs[key(vm, port)]; ok {
		s.st, _ = services.Next(s.st, services.EvUnlisten, d.h.Clock.Now())
	}
}

func (d *Daemon) unlistenExcept(vm string, seen map[int]bool) {
	d.mu.Lock()
	var gone []int
	for _, s := range d.svcs {
		if s.vm == vm && !seen[s.port] {
			gone = append(gone, s.port)
		}
	}
	d.mu.Unlock()
	for _, port := range gone {
		d.unlisten(vm, port)
	}
}

func (d *Daemon) tickServices(ctx context.Context) {
	// invariant: every reserved port ticks, listened on in this engine's
	// life or not, so one idle past its days is released, and one served
	// that its VM says nothing listens on is unshared.
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.h.Clock.Now()
	for _, l := range slices.Clone(d.leases) {
		d.svcOf(l.VM, l.GuestPort)
	}
	for _, s := range d.svcs {
		var acts []services.Action
		s.st, acts = services.Next(s.st, services.EvTick, now)
		d.apply(ctx, s, acts)
		_, told := d.heard[s.vm]
		if v := d.vms[s.vm]; s.st.Phase == services.Waiting && s.served && (told || v == nil || v.state != StateRunning) {
			d.unpublish(ctx, s)
		}
		if s.st.Phase == services.Published && len(s.listeners) == 0 && !s.retryAt.IsZero() && !now.Before(s.retryAt) {
			d.publish(ctx, s)
		}
	}
}

func (d *Daemon) vmStopped(ctx context.Context, vm string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.heard, vm)
	for _, s := range d.svcs {
		if s.vm == vm {
			var acts []services.Action
			s.st, acts = services.Next(s.st, services.EvVMStopped, d.h.Clock.Now())
			d.apply(ctx, s, acts)
		}
	}
}

func (d *Daemon) releaseVM(ctx context.Context, vm string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.svcs {
		if s.vm == vm {
			d.unpublish(ctx, s)
			delete(d.svcs, key(s.vm, s.port))
		}
	}
	httpsMode := d.policy().HTTPS
	kept, gone := services.Without(d.leases, func(l services.Lease) bool { return l.VM != vm })
	if d.self.On {
		for _, l := range gone {
			_ = d.tn.Unshare(ctx, services.ShareOf(l, httpsMode))
		}
	}
	if len(gone) > 0 {
		d.leases = kept
		d.saveLeases()
	}
}

// invariant: publication binds 127.0.0.1 only, never the tailnet ip or a
// wildcard: the tailnet carries its own bytes, so no firewall stands in the
// way; a pinned lease never moves, an unpinned one moves off a busy port and
// says so.
func (d *Daemon) publish(ctx context.Context, s *svc) {
	p := d.policy()
	s.detail, s.serveNote, s.refused = "", "", false
	i := d.leaseIndex(s.vm, s.port)
	if i < 0 {
		l, err := ports.Reserve(p, d.leases, s.vm, s.port, d.busy)
		if err != nil {
			s.st.Phase, s.refused, s.detail = services.Refused, true, err.Error()
			return
		}
		l.ReservedAt = d.h.Clock.Now().UTC().Truncate(time.Second)
		l.LastSeenAt = l.ReservedAt
		if pr, ok := ports.ProfileOf(s.port); ok {
			l.HTTP = &pr.HTTP
		}
		d.leases = append(d.leases, l)
		i = len(d.leases) - 1
		d.saveLeases()
	}
	lease := d.leases[i]
	local, err := d.h.Sockets.ListenTCP(services.LocalHost, lease.HostPort)
	if err != nil {
		if lease.Pinned {
			s.detail, s.retryAt = "host port "+strconv.Itoa(lease.HostPort)+" is busy", d.h.Clock.Now().Add(busyRetry)
			s.refused = true
			return
		}
		was := lease.HostPort
		others := append(append([]services.Lease(nil), d.leases[:i]...), d.leases[i+1:]...)
		moved, allocErr := ports.Reserve(p, others, s.vm, s.port, func(hp int) bool { return hp == was || d.busy(hp) })
		if allocErr != nil {
			s.st.Phase, s.refused, s.detail = services.Refused, true, allocErr.Error()
			return
		}
		lease.HostPort = moved.HostPort
		d.leases[i] = lease
		d.saveLeases()
		if local, err = d.h.Sockets.ListenTCP(services.LocalHost, lease.HostPort); err != nil {
			s.detail = err.Error()
			return
		}
		s.detail = "host port " + strconv.Itoa(was) + " was busy: now " + strconv.Itoa(lease.HostPort)
	}
	s.listeners = []system.Listener{local}
	go d.carry(ctx, s, local)
	if lease.HTTP == nil {
		http := d.detectHTTP(ctx, s)
		lease.HTTP = &http
		d.leases[i] = lease
		d.saveLeases()
	}
	d.expose(ctx, s, lease, p.HTTPS)
}

// invariant: with Serve off no serve runs: the app would block waiting for the
// admin; a serve refused while Serve is on leaves the service local, its
// public URL unchanged, and says the tailnet's own line.
func (d *Daemon) expose(ctx context.Context, s *svc, lease services.Lease, httpsMode string) {
	s.serveNote = ""
	if !d.self.On {
		return
	}
	if _, err := d.tn.Share(ctx, services.ShareOf(lease, httpsMode)); err != nil {
		s.serveNote = "serve: " + err.Error()
		if refused := (*ports.Refused)(nil); errors.As(err, &refused) {
			s.serveNote = "serve: " + refused.Detail
		}
		return
	}
	s.served = true
}

func (d *Daemon) serveAll(ctx context.Context) {
	p := d.policy()
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.svcs {
		if i := d.leaseIndex(s.vm, s.port); i >= 0 && s.st.Phase == services.Published && len(s.listeners) > 0 {
			d.expose(ctx, s, d.leases[i], p.HTTPS)
		}
	}
}

func (d *Daemon) unserveAll() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.svcs {
		s.served, s.serveNote = false, ""
	}
}

func (d *Daemon) unpublish(ctx context.Context, s *svc) {
	for _, l := range s.listeners {
		_ = l.Close()
	}
	s.listeners = nil
	if s.served && d.self.On {
		if i := d.leaseIndex(s.vm, s.port); i >= 0 {
			_ = d.tn.Unshare(ctx, services.ShareOf(d.leases[i], d.policy().HTTPS))
		}
	}
	s.served, s.serveNote = false, ""
}

func (d *Daemon) clientOf(vm string) Guest {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.clientLocked(vm)
}

func (d *Daemon) clientLocked(vm string) Guest {
	if v, ok := d.vms[vm]; ok {
		return v.client
	}
	return nil
}

func (d *Daemon) carry(ctx context.Context, s *svc, l system.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		go func() {
			client := d.clientOf(s.vm)
			if client == nil {
				_ = conn.Close()
				return
			}
			remote, err := client.Forward(ctx, services.Dialed(s.ip), s.port)
			if err != nil {
				_ = conn.Close()
				return
			}
			d.h.Sockets.Join(conn, remote)
		}()
	}
}

type readResult struct {
	n   int
	err error
}

// invariant: HTTP is detected once per lease: a server that speaks first
// (ssh, mysql, smtp) is not HTTP; else a HEAD answered with "HTTP/" is.
func (d *Daemon) detectHTTP(ctx context.Context, s *svc) bool {
	client := d.clientLocked(s.vm)
	if client == nil {
		return false
	}
	conn, err := client.Forward(ctx, services.Dialed(s.ip), s.port)
	if err != nil {
		return false
	}
	defer func() { _ = conn.Close() }()
	buf := make([]byte, 64)
	got := make(chan readResult, 1)
	go func() {
		n, err := conn.Read(buf)
		got <- readResult{n, err}
	}()
	wait, cancel := context.WithTimeout(ctx, bannerWait)
	select {
	case r := <-got:
		cancel()
		return r.n >= 5 && string(buf[:5]) == "HTTP/"
	case <-wait.Done():
		cancel()
	}
	if _, err := conn.Write([]byte("HEAD / HTTP/1.0\r\n\r\n")); err != nil {
		return false
	}
	wait, cancel = context.WithTimeout(ctx, headWait)
	defer cancel()
	select {
	case r := <-got:
		return r.n >= 5 && string(buf[:5]) == "HTTP/"
	case <-wait.Done():
		return false
	}
}

func (d *Daemon) view(l services.Lease, s *svc, httpsMode string) services.View {
	v := services.ViewOf(l, services.ExposureOf(d.self.On, d.self.Host), httpsMode, services.Waiting)
	if s != nil {
		switch s.st.Phase {
		case services.Published, services.Closing:
			v.State = services.Published
		case services.Refused:
			v.State = services.Refused
		case services.None, services.Waiting, services.Gone, services.Unknown:
		}
		v.Process, v.Bind = s.process, s.ip
		v.Detail = services.JoinDetail(services.JoinDetail(s.detail, s.serveNote), v.Detail)
		if d.issuing && s.served && l.HTTP != nil && *l.HTTP {
			v.Detail = services.JoinDetail(v.Detail, issuingDetail)
		}
		if s.refused {
			v.State = services.Refused
		}
	}
	if vs := d.vms[l.VM]; v.State == services.Waiting && vs != nil && vs.state != StateRunning {
		v.Detail = services.JoinDetail(catalog.StoppedDetail(l.VM), v.Detail)
	}
	return v
}

func (d *Daemon) hostView(vm string) services.HostView {
	p := d.policy()
	d.mu.Lock()
	defer d.mu.Unlock()
	e := services.ExposureOf(d.self.On, d.self.Host)
	out := services.HostView{
		Name: d.m.HereName(), Here: true, Reachable: true, Daemon: services.DaemonInfo{Running: true, Version: d.o.Version},
		PublicHost: e.PublicHost, Serve: e.Word(), HTTPS: p.HTTPS, Services: []services.View{},
	}
	if vm == "" {
		out.Listening = d.listeningLocked()
	}
	off := ""
	if !e.On {
		off = catalog.ServeOffDetail
	}
	for _, l := range d.leases {
		if vm == "" || l.VM == vm {
			out.Services = append(out.Services, d.view(l, d.svcs[key(l.VM, l.GuestPort)], p.HTTPS))
		}
	}
	for _, s := range d.svcs {
		if s.st.Phase == services.Refused && d.leaseIndex(s.vm, s.port) < 0 && (vm == "" || s.vm == vm) {
			out.Services = append(out.Services, services.View{VM: s.vm, GuestPort: s.port, State: services.Refused, Process: s.process, Detail: services.JoinDetail(s.detail, off)})
		}
	}
	sort.Slice(out.Services, func(i, j int) bool {
		a, b := out.Services[i], out.Services[j]
		if a.VM != b.VM {
			return a.VM < b.VM
		}
		return a.GuestPort < b.GuestPort
	})
	return out
}

func (d *Daemon) reserve(ctx context.Context, vm string, port int, pinned bool, profile string) (services.View, error) {
	if _, err := d.find(vm); err != nil {
		return services.View{}, err
	}
	p := d.policy()
	d.mu.Lock()
	defer d.mu.Unlock()
	l, err := ports.Reserve(p, d.leases, vm, port, d.busy)
	if err != nil {
		return services.View{}, asOp(err)
	}
	if i := d.leaseIndex(vm, port); i < 0 {
		l.ReservedAt = d.h.Clock.Now().UTC().Truncate(time.Second)
		l.Pinned, l.Profile = pinned, profile
		if pr, ok := ports.ProfileOf(port); ok && profile != "" {
			l.HTTP = &pr.HTTP
		}
		d.leases = append(d.leases, l)
		d.saveLeases()
	}
	s := d.svcs[key(vm, port)]
	if s != nil && (s.st.Phase == services.Refused || s.refused) {
		s.st.Phase = services.None
		var acts []services.Action
		s.st, acts = services.Next(s.st, services.EvListen, d.h.Clock.Now())
		d.apply(ctx, s, acts)
	}
	if d.self.Backend != ports.Running {
		return services.View{}, ports.SignedOut(d.m.HereName())
	}
	return d.view(l, s, p.HTTPS), nil
}

func (d *Daemon) release(ctx context.Context, vm string, port int) (services.Lease, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	i := d.leaseIndex(vm, port)
	if i < 0 {
		return services.Lease{}, services.NotFound(services.Target{VM: vm, Port: port})
	}
	l := d.leases[i]
	if s, ok := d.svcs[key(vm, port)]; ok {
		d.unpublish(ctx, s)
		delete(d.svcs, key(vm, port))
	}
	d.leases = append(d.leases[:i:i], d.leases[i+1:]...)
	d.saveLeases()
	return l, nil
}

// invariant: station owns only the serve entries of its range (port mode and
// raw TCP) or of a path /<vm>/ of this host (path mode), and removes those no
// lease justifies, both kinds; it never touches another entry, and waits for
// Serve to be on.
func (d *Daemon) reconcileServe(ctx context.Context) {
	d.mu.Lock()
	on := d.self.On
	d.mu.Unlock()
	if !on {
		return
	}
	shared, err := d.tn.Shared(ctx)
	if err != nil {
		return
	}
	p := d.policy()
	d.mu.Lock()
	leases := append([]services.Lease(nil), d.leases...)
	vms := map[string]bool{}
	for name := range d.vms {
		vms[name] = true
	}
	d.mu.Unlock()
	for _, e := range shared {
		switch {
		case p.InRange(e.Port):
			justified := false
			for _, l := range leases {
				justified = justified || l.HostPort == e.Port
			}
			if !justified {
				e.Path = ""
				_ = d.tn.Unshare(ctx, e)
				continue
			}
			d.servedBefore(e.Port)
		case e.Port == 443 && strings.Count(e.Path, "/") >= 2:
			parts := strings.Split(strings.Trim(e.Path, "/"), "/")
			justified := false
			for _, l := range leases {
				justified = justified || (l.VM == parts[0] && strconv.Itoa(l.GuestPort) == parts[len(parts)-1])
			}
			if vms[parts[0]] && !justified {
				_ = d.tn.Unshare(ctx, e)
			}
		}
	}
}

// invariant: a reserved port the tailnet already serves when the engine
// starts is served until its VM says it no longer listens there.
func (d *Daemon) servedBefore(hostPort int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, l := range d.leases {
		if l.HostPort == hostPort {
			if s := d.svcOf(l.VM, l.GuestPort); s.st.Phase != services.Published {
				s.served = true
			}
		}
	}
}
