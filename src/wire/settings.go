package wire

import (
	"context"

	"github.com/jterrazz/studio-station/src/core/call"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/engine/process"
	"github.com/jterrazz/studio-station/src/machines"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/remote/each"
	"github.com/jterrazz/studio-station/src/settings"
	settingscalls "github.com/jterrazz/studio-station/src/settings/calls"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

// value: the matrix's reach: the fleet as This Mac lists it, each machine's
// settings through this run's Router, the last reads in This Mac's engine.
type settingsAllMachines struct{ m machine }

func (f settingsAllMachines) Machines(ctx context.Context) (machines.AllMachines, error) {
	out, err := call.Read(ctx, f.m.far.router, coremachine.Ref{}, machinescalls.List, machinescalls.ListIn{Unwritten: true}, nil)
	return out.Fleet, err
}

func (f settingsAllMachines) Read(ctx context.Context, refs []coremachine.Ref) map[coremachine.Ref]settingscalls.Answer {
	r := f.m.far.router
	names := make([]string, len(refs))
	by := make(map[string]coremachine.Ref, len(refs))
	for i, ref := range refs {
		names[i] = ref.Name + "@" + ref.On
		by[names[i]] = ref
	}
	got := each.All(ctx, names, func(ctx context.Context, name string) ([]settings.Row, error) {
		rows, err := call.Read(ctx, r, by[name], settingscalls.List, settingscalls.ListIn{}, nil)
		if err == nil && rows == nil {
			rows = []settings.Row{}
		}
		return rows, err
	})
	out := make(map[coremachine.Ref]settingscalls.Answer, len(refs))
	for name, a := range got {
		if a.Err != nil {
			out[by[name]] = settingscalls.Answer{Why: kit.Translate(a.Err, "settings").Message}
			continue
		}
		out[by[name]] = settingscalls.Answer{Rows: a.Value}
	}
	return out
}

func (f settingsAllMachines) Keep(ctx context.Context, vm string, read settings.LastRead) {
	_ = f.m.engineAPI().do(ctx, process.KeepSettings, process.KeepSettingsIn{VM: vm, Read: read}, nil)
}

func (f settingsAllMachines) Last(ctx context.Context, vm string) (settings.LastRead, bool) {
	var out process.LastSettingsOut
	if err := f.m.engineAPI().do(ctx, process.LastSettings, process.VMIn{VM: vm}, &out); err != nil {
		return settings.LastRead{}, false
	}
	return out.Read, out.Kept
}
