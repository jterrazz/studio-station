package daemon

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/system"
)

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
	httpsDown bool
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

func asOp(err error) *OpError {
	var full *services.RangeFullError
	var perVM *services.MaxPerVMError
	switch {
	case errors.As(err, &full):
		return fault("range_full", err.Error(), "station services release <vm>:<port>")
	case errors.As(err, &perVM):
		return fault("max_per_vm", err.Error(), "station services release <vm>:<port>")
	}
	return fault("internal", err.Error(), "station services")
}

func (d *Daemon) svcOf(vm string, port int) *svc {
	s, ok := d.svcs[key(vm, port)]
	if !ok {
		s = &svc{vm: vm, port: port}
		if i := d.leaseIndex(vm, port); i >= 0 {
			s.st.Pinned = d.leases[i].Pinned
			s.st.LastSeenAt = d.leases[i].LastSeenAt
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

func (d *Daemon) listen(ctx context.Context, vm string, port int, ip, process string) {
	p := d.policy()
	if p.Ignores(port) {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
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
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.h.Clock.Now()
	for _, s := range d.svcs {
		var acts []services.Action
		s.st, acts = services.Next(s.st, services.EvTick, now)
		d.apply(ctx, s, acts)
		if s.st.Phase == services.Published && len(s.listeners) == 0 && !s.retryAt.IsZero() && !now.Before(s.retryAt) {
			d.publish(ctx, s)
		}
	}
}

func (d *Daemon) vmStopped(ctx context.Context, vm string) {
	d.mu.Lock()
	defer d.mu.Unlock()
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
	for _, l := range gone {
		if l.HTTP == nil || *l.HTTP {
			if args := services.ServeArgs(l, httpsMode, false); args != nil {
				_ = d.tn.Serve(ctx, args...)
			}
		}
	}
	if len(gone) > 0 {
		d.leases = kept
		d.saveLeases()
	}
}

// invariant: publication binds 127.0.0.1 and the host's tailnet ip, never a
// wildcard; an HTTP service with https on its own port leaves the tailnet port
// to tailscale serve; a pinned lease never moves, an unpinned one moves off a
// busy port and says so.
func (d *Daemon) publish(ctx context.Context, s *svc) {
	p := d.policy()
	s.detail, s.httpsDown, s.refused = "", false, false
	i := d.leaseIndex(s.vm, s.port)
	if i < 0 {
		l, err := services.Allocate(p, d.leases, s.vm, s.port, d.busy)
		if err != nil {
			s.st.Phase, s.refused, s.detail = services.Refused, true, err.Error()
			return
		}
		l.ReservedAt = d.h.Clock.Now().UTC().Truncate(time.Second)
		l.LastSeenAt = l.ReservedAt
		if pr, ok := services.ProfileOf(s.port); ok {
			l.HTTP = &pr.HTTP
		}
		d.leases = append(d.leases, l)
		i = len(d.leases) - 1
		d.saveLeases()
	}
	lease := d.leases[i]
	local, err := d.h.Sockets.ListenTCP("127.0.0.1", lease.HostPort)
	if err != nil {
		if lease.Pinned {
			s.detail, s.retryAt = "host port "+strconv.Itoa(lease.HostPort)+" is busy", d.h.Clock.Now().Add(busyRetry)
			s.refused = true
			return
		}
		was := lease.HostPort
		others := append(append([]services.Lease(nil), d.leases[:i]...), d.leases[i+1:]...)
		moved, allocErr := services.Allocate(p, others, s.vm, s.port, func(hp int) bool { return hp == was || d.busy(hp) })
		if allocErr != nil {
			s.st.Phase, s.refused, s.detail = services.Refused, true, allocErr.Error()
			return
		}
		lease.HostPort = moved.HostPort
		d.leases[i] = lease
		d.saveLeases()
		if local, err = d.h.Sockets.ListenTCP("127.0.0.1", lease.HostPort); err != nil {
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
	httpsNote := ""
	if *lease.HTTP {
		if args := services.ServeArgs(lease, p.HTTPS, true); args != nil {
			if err := d.tn.Serve(ctx, args...); err != nil {
				s.httpsDown, httpsNote = true, "https: "+strings.TrimPrefix(err.Error(), "tailscale serve refused: ")
			} else {
				s.served = true
			}
		}
	}
	if !s.served || p.HTTPS != services.HTTPSPort {
		note := ""
		switch d.self.IP {
		case "":
			note = "tailnet down: local only"
		default:
			if tl, err := d.h.Sockets.ListenTCP(d.self.IP, lease.HostPort); err == nil {
				s.listeners = append(s.listeners, tl)
				go d.carry(ctx, s, tl)
			} else {
				note = "local only"
			}
		}
		if note != "" {
			s.detail = joinDetail(s.detail, note)
		}
	}
	if httpsNote != "" {
		s.detail = joinDetail(s.detail, httpsNote)
	}
}

func joinDetail(a, b string) string {
	if a == "" {
		return b
	}
	return a + " · " + b
}

func (d *Daemon) unpublish(ctx context.Context, s *svc) {
	for _, l := range s.listeners {
		_ = l.Close()
	}
	s.listeners = nil
	if s.served {
		if i := d.leaseIndex(s.vm, s.port); i >= 0 {
			if args := services.ServeArgs(d.leases[i], d.policy().HTTPS, false); args != nil {
				_ = d.tn.Serve(ctx, args...)
			}
		}
		s.served = false
	}
}

func (d *Daemon) clientOf(vm string) system.SSHClient {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.clientLocked(vm)
}

func (d *Daemon) clientLocked(vm string) system.SSHClient {
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
	phase := services.Waiting
	v := services.ViewOf(l, d.self.DNSName, httpsMode, phase)
	if s != nil {
		switch s.st.Phase {
		case services.Published, services.Closing:
			v.State = services.Published
		case services.Refused:
			v.State = services.Refused
		case services.None, services.Waiting, services.Gone, services.Unknown:
		}
		v.Process, v.Detail = s.process, s.detail
		if s.httpsDown {
			v.URLs.HTTPS = ""
		}
		if s.refused {
			v.State = services.Refused
		}
	}
	return v
}

func (d *Daemon) hostView(vm string) services.HostView {
	p := d.policy()
	d.mu.Lock()
	defer d.mu.Unlock()
	out := services.HostView{
		Name: d.m.HereName(), Here: true, Reachable: true, Daemon: services.DaemonInfo{Running: true, Version: d.o.Version},
		PublicHost: d.self.DNSName, HTTPS: p.HTTPS, Services: []services.View{},
	}
	for _, l := range d.leases {
		if vm == "" || l.VM == vm {
			out.Services = append(out.Services, d.view(l, d.svcs[key(l.VM, l.GuestPort)], p.HTTPS))
		}
	}
	for _, s := range d.svcs {
		if s.st.Phase == services.Refused && d.leaseIndex(s.vm, s.port) < 0 && (vm == "" || s.vm == vm) {
			out.Services = append(out.Services, services.View{VM: s.vm, GuestPort: s.port, State: services.Refused, Process: s.process, Detail: s.detail})
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
	l, err := services.Allocate(p, d.leases, vm, port, d.busy)
	if err != nil {
		return services.View{}, asOp(err)
	}
	if i := d.leaseIndex(vm, port); i < 0 {
		l.ReservedAt = d.h.Clock.Now().UTC().Truncate(time.Second)
		l.Pinned, l.Profile = pinned, profile
		if pr, ok := services.ProfileOf(port); ok && profile != "" {
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
	return d.view(l, s, p.HTTPS), nil
}

func (d *Daemon) release(ctx context.Context, vm string, port int) (services.Lease, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	i := d.leaseIndex(vm, port)
	if i < 0 {
		return services.Lease{}, fault("not_running", (&services.NoLeaseError{VM: vm, Port: port}).Error(), "station services")
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

// invariant: station owns only the serve entries of its range (port mode) or of
// a path /<vm>/ of this host (path mode), and removes those no lease justifies;
// it never touches another entry.
func (d *Daemon) reconcileServe(ctx context.Context) {
	cfg, err := d.tn.ServeStatus(ctx)
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
	for _, e := range cfg.Entries {
		owned, justified := false, false
		switch {
		case p.InRange(e.Port):
			owned = true
			for _, l := range leases {
				justified = justified || l.HostPort == e.Port
			}
			if !justified {
				_ = d.tn.Serve(ctx, "--https="+strconv.Itoa(e.Port), "off")
			}
		case e.Port == 443 && strings.Count(e.Path, "/") >= 2:
			parts := strings.Split(strings.Trim(e.Path, "/"), "/")
			owned = vms[parts[0]]
			for _, l := range leases {
				justified = justified || (l.VM == parts[0] && strconv.Itoa(l.GuestPort) == parts[len(parts)-1])
			}
			if owned && !justified {
				_ = d.tn.Serve(ctx, "--https=443", "--set-path="+e.Path, "off")
			}
		}
		_ = owned
	}
}
