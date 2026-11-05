package activitycmd

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/activity"
	activitycalls "github.com/jterrazz/studio-station/src/activity/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/machines"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/page"
	pageactivity "github.com/jterrazz/studio-station/src/pages/activity"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Here page.Context
	Now  func() time.Time
	// value: the short name a Mac is said by (mac-mini).
	Label func(mac string) string
}

type Input struct{ AllMachines, History bool }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "activity",
		Short:   "What a machine is doing now: CPU, memory, disk, network, thermal, battery, the busiest processes",
		Example: "  station activity\n  station activity --on linux-1\n  station activity --all-machines\n  station activity --json --history",
		Group:   kit.OnAMachine,
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.BoolVar(&in.AllMachines, "all-machines", false, "one row per machine")
			f.BoolVar(&in.History, "history", false, "with --json: the 24 h series the engine keeps")
		},
		Read:  func(*Input) bool { return true },
		Pages: true,
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			if in.History && !io.JSON {
				return kit.Refuse("--history is for scripts: it needs --json", "station activity --json --history")
			}
			if in.AllMachines {
				views, err := every(ctx, d, env)
				if err != nil {
					return err
				}
				return io.Page(pageactivity.AllMachines(views, d.Here), pageactivity.AllMachinesContract(views, d.Now()))
			}
			v, err := one(ctx, d, env, env.Target(), in.History)
			if err != nil {
				return err
			}
			return io.Page(pageactivity.Page(v, d.Here), pageactivity.Contract(v, d.Now()))
		},
	}, env)
}

func mac(ctx context.Context, env kit.Env, ref machine.Ref, label string, history bool) (pageactivity.View, error) {
	v := pageactivity.View{Name: ref.Name, Label: label, Kind: string(machine.Mac), ThisMac: ref == env.Self}
	if v.ThisMac {
		v.Label = "This Mac"
	}
	r, err := call.Read(ctx, env.Calls, ref, activitycalls.Read, activitycalls.ReadIn{History: history}, nil)
	v.Report = r
	return v, err
}

func one(ctx context.Context, d Deps, env kit.Env, target machine.Ref, history bool) (pageactivity.View, error) {
	if target == env.Self {
		return mac(ctx, env, env.Self, env.Self.Name, history)
	}
	held, ok := kit.Holder(ctx, env, target.Name)
	if !ok {
		return mac(ctx, env, target, target.Name, history)
	}
	holder := machine.Ref{Name: held.Mac}
	if held.MacIsHere {
		holder = env.Self
	}
	return vm(ctx, d, env, holder, held.VM, held.Kind, held.Running, history)
}

func vm(ctx context.Context, d Deps, env kit.Env, holder machine.Ref, name string, kind machine.Kind, running, history bool) (pageactivity.View, error) {
	// invariant: a VM's history is its Mac's engine's, and so is the word
	// that its guest does not answer (the relay into it is that engine's);
	// one it holds no last 10 s of is read inside the VM, twice 1 s apart,
	// its series still that engine's.
	v := pageactivity.View{Name: name, Label: name, Kind: string(kind), Mac: holder.Name, On: d.Label(holder.Name), Stopped: !running}
	if holder == env.Self {
		v.On = "This Mac"
	}
	if !running {
		return v, nil
	}
	in := activitycalls.ReadIn{Of: name, Machine: machine.Ref{Name: name, On: holder.Name}, Kind: kind, History: history}
	r, err := call.Read(ctx, env.Calls, holder, activitycalls.Read, in, nil)
	if err == nil && r.Source != activity.NotKept {
		v.Report = r
		return v, nil
	}
	kept := r
	in.Of, in.History = "", false
	r, err = call.Read(ctx, env.Calls, in.Machine, activitycalls.Read, in, nil)
	if kept.History != nil {
		r.Since, r.History = kept.Since, kept.History
	}
	r.Findings = activity.Lasting(r.Findings, kept.Findings)
	v.Report = r
	return v, err
}

func every(ctx context.Context, d Deps, env kit.Env) ([]pageactivity.View, error) {
	// invariant: This Mac then each of its VMs, then every other Mac and
	// its VMs; a Mac that does not answer is its own row and the others are
	// filled.
	list, err := call.Read(ctx, env.Calls, env.Self, machinescalls.List, machinescalls.ListIn{Unwritten: true}, nil)
	if err != nil {
		return nil, err
	}
	var views []pageactivity.View
	for _, h := range list.Fleet.Hosts {
		ref := machine.Ref{Name: h.Name}
		if h.Here {
			ref = env.Self
		}
		if !h.Reachable && !h.Here {
			views = append(views, pageactivity.View{Name: h.Name, Label: h.Name, Kind: string(machine.Mac), Silent: "does not answer"})
			continue
		}
		v, err := mac(ctx, env, ref, h.Name, false)
		if err != nil {
			v.Silent = kit.Translate(err, "").Message
		}
		views = append(views, v)
		for _, m := range h.VMs {
			v, err := vm(ctx, d, env, ref, m.Name, m.Lifetime.Kind(), m.State == machines.Running, false)
			if err != nil {
				v.Silent = kit.Translate(err, "").Message
			}
			views = append(views, v)
		}
	}
	return views, ctx.Err()
}
