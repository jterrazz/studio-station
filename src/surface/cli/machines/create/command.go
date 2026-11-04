package machinescreate

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/core/call"
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
	// value: the fleet, for the host question; read only.
	Fleet func(ctx context.Context) (machines.Fleet, error)
	Here  page.Context
	Ask   kit.Asker
	On    func(ctx context.Context, host string, lifetime string) error
}

type Input struct {
	Lifetime, Image, Name, Memory, Disk, Network string
	CPUs                                         int
	request                                      machines.CreateRequest
	buildFirst                                   bool
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:   "create",
		Short: "Make a VM on this host, from an image",
		Example: "  station machines create --lifetime temporary\n" +
			"  station machines create --lifetime lasting --name api --memory 12G",
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.StringVar(&in.Lifetime, "lifetime", "", "temporary (a poweroff inside it deletes it) or lasting; asked on a terminal")
			f.StringVar(&in.Image, "image", "linux", "the image the VM is made from")
			f.StringVar(&in.Name, "name", "", "the VM's name (default <image>-<n>)")
			f.IntVar(&in.CPUs, "cpus", 0, "virtual CPUs (default the image's)")
			f.StringVar(&in.Memory, "memory", "", "memory, e.g. 8G or 512M; a bare number is GB")
			f.StringVar(&in.Disk, "disk", "", "disk size, e.g. 50G; a bare number is GB")
			f.StringVar(&in.Network, "network", "nat", "nat, or bridged (macOS VMs only: an iPhone on the Wi-Fi reaches it)")
		},
		Read: func(*Input) bool { return false },
		Bind: func(in *Input, _ []string) error {
			r, err := d.Resources(in.CPUs, in.Memory, in.Disk)
			if err != nil {
				return err
			}
			in.request = machines.CreateRequest{Image: in.Image, Name: in.Name, Lifetime: machines.Lifetime(in.Lifetime), Network: in.Network, Resources: r}
			return nil
		},
		Validate: func(ctx context.Context, in *Input) error {
			// invariant: on a terminal the questions come in Run; every refusal a
			// question cannot answer still comes first, with a lifetime stand-in.
			check := in.request
			asking := d.Ask != nil
			if check.Lifetime == "" && asking {
				check.Lifetime = machines.Temporary
			}
			unbuilt, err := d.Check(ctx, check)
			if unbuilt && asking {
				in.buildFirst = true
				return nil
			}
			return err
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			if d.Ask != nil {
				done, err := ask(ctx, d, in)
				if err != nil || done {
					return err
				}
			}
			if in.buildFirst {
				if _, err := call.Invoke(ctx, env.Calls, env.Target, imagescalls.Build, imagescalls.BuildIn{Names: []string{in.request.Image}}, nil); err != nil {
					return err
				}
			}
			steps := kit.Steps(io)
			created, err := call.Invoke(ctx, env.Calls, env.Target, machinescalls.Create, machinescalls.CreateIn{Request: in.request}, steps)
			steps.Stop()
			if err != nil {
				return err
			}
			return io.Page(pagemachines.Created(created, d.Here), pagemachines.CreatedContract(created))
		},
	}, env)
}

func ask(ctx context.Context, d Deps, in *Input) (done bool, err error) {
	var hosts []pagemachines.HostChoice
	if fleet, fleetErr := d.Fleet(ctx); fleetErr == nil {
		for _, h := range fleet.Hosts {
			if h.Reachable {
				hosts = append(hosts, pagemachines.HostChoice{Name: h.Name, Here: h.Here, Detail: vmCount(len(h.VMs))})
			}
		}
	}
	answers := map[string]string{"lifetime": in.Lifetime}
	if form := pagemachines.CreateForm(in.Lifetime, hosts); len(form.Fields) > 0 {
		asked, err := d.Ask.Form(form)
		if err != nil {
			return false, err
		}
		for k, v := range asked {
			answers[k] = v
		}
	}
	in.request.Lifetime = machines.Lifetime(answers["lifetime"])
	for _, h := range hosts {
		if h.Name == answers["host"] && !h.Here {
			return true, d.On(ctx, h.Name, answers["lifetime"])
		}
	}
	if in.buildFirst {
		build, err := d.Ask.Confirm(pagemachines.BuildConfirm(in.request.Image))
		if err != nil {
			return false, err
		}
		if !build {
			return false, d.NotBuilt(in.request.Image)
		}
	}
	return false, nil
}

func vmCount(n int) string {
	if n == 1 {
		return "1 VM"
	}
	return strconv.Itoa(n) + " VMs"
}
