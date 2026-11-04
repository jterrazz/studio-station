package calls

import (
	"context"
	"strconv"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/confirm"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/section"
	"github.com/jterrazz/studio-station/src/ports"
)

type ListIn struct {
	// value: only this host, not the fleet.
	Here bool
}

var List = call.Call[Deps, ListIn, ports.Fleet]{
	Name: "services.list", Section: section.Ports, Where: call.AtTarget, Applies: ports.Applies,
	Run: func(ctx context.Context, d Deps, in ListIn, _ call.Progress) (ports.Fleet, error) {
		if err := d.Services.CheckList(in.Here); err != nil {
			return ports.Fleet{}, err
		}
		return d.Services.List(ctx, in.Here)
	},
}

type URLIn struct{ Port string }

var URL = call.Call[Deps, URLIn, ports.ServiceView]{
	Name: "services.url", Section: section.Ports, Where: call.InEngine, Applies: ports.Applies,
	Run: func(ctx context.Context, d Deps, in URLIn, _ call.Progress) (ports.ServiceView, error) {
		port, err := d.Services.CheckURL(in.Port)
		if err != nil {
			return ports.ServiceView{}, err
		}
		return d.Services.URL(ctx, port)
	},
}

type ReleaseIn struct {
	// value: <vm>:<port>, or a port inside a VM.
	Target  string
	Confirm string
}

type ReleaseOut struct {
	Lease ports.Reservation
	// value: the host that released it when it is not this one; it printed
	// its own answer.
	Host string
}

var Release = call.Call[Deps, ReleaseIn, ReleaseOut]{
	Name: "services.release", Section: section.Ports, Where: call.InEngine, Applies: ports.Applies, Confirm: confirm.StopSharing,
	Errors: []fail.Code{confirm.YesNeeded},
	Run: func(ctx context.Context, d Deps, in ReleaseIn, _ call.Progress) (ReleaseOut, error) {
		// invariant: a pinned port's URL is in its VM's environment, so its
		// release asks first; a release on another host is answered there.
		t, err := d.Services.ParseTarget(in.Target)
		if err != nil {
			return ReleaseOut{}, err
		}
		host, err := d.Services.CheckRelease(ctx, t)
		if err != nil {
			return ReleaseOut{}, err
		}
		if d.Services.Pinned(t) {
			if err := confirm.Require(confirm.For(confirm.StopSharing, d.Kind), t.VM+":"+strconv.Itoa(t.Port), in.Confirm); err != nil {
				return ReleaseOut{}, err
			}
		}
		l, err := d.Services.Release(ctx, t, host, in.Confirm == confirm.Yes)
		return ReleaseOut{Lease: l, Host: host}, err
	},
}
