package machinescreate

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
	"github.com/jterrazz/studio-station/src/machines"
	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/page"
	pagemachines "github.com/jterrazz/studio-station/src/pages/machines"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the flags' sizes, parsed as the hypervisor takes them.
	Resources func(cpus int, memory, disk string) (machines.Resources, error)
	// value: every refusal a question cannot answer; unbuilt when the only
	// one is the image, not built yet.
	Check func(ctx context.Context, r machines.CreateRequest) (unbuilt bool, err error)
	// value: the refusal of an image the owner chose not to build.
	NotBuilt func(image string) error
	// value: refuses this machine when the call's rule says no, before any
	// check of the request.
	Admit func() error
	// value: the fleet, for the host question; read only.
	Fleet func(ctx context.Context) (machines.Fleet, error)
	Here  page.Context
	Ask   kit.Asker
}

type Input struct {
	Image, Name, Memory, Disk, Network string
	Permanent                          bool
	CPUs                               int
	// value: v2's --lifetime, kept hidden for one release (SPEC §4.3).
	Lifetime   string
	request    machines.CreateRequest
	buildFirst bool
	// value: another Mac chosen on a terminal, the create sent there.
	host string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:   "create [<name>]",
		Short: "Make a workspace on this Mac: throwaway, or kept with --permanent",
		Example: "  station machines create\n" +
			"  station machines create api --permanent --memory 12G",
		Args: cobra.MaximumNArgs(1),
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.BoolVar(&in.Permanent, "permanent", false, "kept until you delete it; without it, deleted when it powers off from inside")
			f.StringVar(&in.Image, "image", "linux", "the image the workspace is made from")
			f.IntVar(&in.CPUs, "cpus", 0, "virtual CPUs (default 4)")
			f.StringVar(&in.Memory, "memory", "", "memory with its unit, e.g. 8G or 512M (default 4G)")
			f.StringVar(&in.Disk, "disk", "", "disk size with its unit, e.g. 50G (default 20G)")
			f.StringVar(&in.Network, "network", "", "bridged: its own address on your LAN (macOS images only); default behind its Mac")
			f.StringVar(&in.Name, "name", "", "the workspace's name")
			f.StringVar(&in.Lifetime, "lifetime", "", "throwaway or permanent (--permanent)")
			_ = f.MarkHidden("name")
			_ = f.MarkHidden("lifetime")
		},
		Read: func(*Input) bool { return false },
		Bind: func(in *Input, args []string) error {
			if len(args) == 1 {
				in.Name = args[0]
			}
			r, err := d.Resources(in.CPUs, in.Memory, in.Disk)
			if err != nil {
				return err
			}
			lifetime := machines.Throwaway
			if in.Permanent {
				lifetime = machines.Permanent
			}
			if in.Lifetime != "" {
				var old machines.Lifetime
				_ = old.UnmarshalText([]byte(in.Lifetime))
				lifetime = old
				if old == machines.Permanent {
					env.Out.Now(nowWords(in))
				}
			}
			in.request = machines.CreateRequest{Image: in.Image, Name: in.Name, Lifetime: lifetime, Network: in.Network, Resources: r}
			return nil
		},
		Validate: func(ctx context.Context, in *Input) error {
			if !env.Far() {
				if err := d.Admit(); err != nil {
					return err
				}
			}
			unbuilt, err := d.Check(ctx, in.request)
			if unbuilt && d.Ask != nil {
				in.buildFirst = true
				return nil
			}
			return err
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			if d.Ask != nil {
				if err := ask(ctx, d, in); err != nil {
					return err
				}
			}
			target := env.Target()
			if in.host != "" {
				target = machine.Ref{Name: in.host}
			}
			if in.buildFirst && in.host == "" {
				if _, err := call.Invoke(ctx, env.Calls, env.Target(), imagescalls.Build, imagescalls.BuildIn{Names: []string{in.request.Image}}, nil); err != nil {
					return err
				}
			}
			steps := kit.Steps(io)
			created, err := call.Invoke(ctx, env.Calls, target, machinescalls.CreateWorkspace, machinescalls.CreateIn{Request: in.request}, steps)
			steps.Stop()
			if err != nil {
				return err
			}
			return io.Page(pagemachines.Created(created, d.Here), pagemachines.CreatedContract(created))
		},
	}, env)
}

// value: the v3 words of a create written with v2's --lifetime lasting.
func nowWords(in *Input) string {
	words := "station machines create"
	if in.Name != "" {
		words += " " + in.Name
	}
	return words + " --permanent"
}

func ask(ctx context.Context, d Deps, in *Input) error {
	var hosts []pagemachines.HostChoice
	if fleet, fleetErr := d.Fleet(ctx); fleetErr == nil {
		for _, h := range fleet.Hosts {
			if h.Reachable {
				hosts = append(hosts, pagemachines.HostChoice{Name: h.Name, Here: h.Here, Detail: vmCount(len(h.VMs))})
			}
		}
	}
	answers := map[string]string{}
	if form := pagemachines.CreateForm(string(in.request.Lifetime), hosts); len(form.Fields) > 0 {
		asked, err := d.Ask.Form(form)
		if err != nil {
			return err
		}
		answers = asked
	}
	for _, h := range hosts {
		if h.Name == answers["host"] && !h.Here {
			in.host = h.Name
			return nil
		}
	}
	if in.buildFirst {
		build, err := d.Ask.Confirm(pagemachines.BuildConfirm(in.request.Image))
		if err != nil {
			return err
		}
		if !build {
			return d.NotBuilt(in.request.Image)
		}
	}
	return nil
}

func vmCount(n int) string {
	if n == 1 {
		return "1 VM"
	}
	return strconv.Itoa(n) + " VMs"
}
