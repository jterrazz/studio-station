package machinescreate

import (
	"context"
	"errors"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/machines"
	presentmachines "github.com/jterrazz/studio-station/src/present/machines"
	"github.com/jterrazz/studio-station/src/prompt"
)

type Deps struct {
	Machines *machines.Service
	Context  string
	Ask      prompt.CreateAsker
	Build    func(ctx context.Context, image string) error
	On       func(ctx context.Context, host string, lifetime string) error
}

type Input struct {
	Lifetime, Image, Name, Memory, Disk string
	CPUs                                int
	request                             machines.CreateRequest
	buildFirst                          bool
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
		},
		Read: func(*Input) bool { return false },
		Bind: func(in *Input, _ []string) error {
			r, err := machines.ParseResources(in.CPUs, in.Memory, in.Disk)
			if err != nil {
				return err
			}
			in.request = machines.CreateRequest{Image: in.Image, Name: in.Name, Lifetime: machines.Lifetime(in.Lifetime), Resources: r}
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
			_, err := d.Machines.CheckCreate(ctx, check)
			var unbuilt *machines.NoImageError
			if errors.As(err, &unbuilt) && asking && d.Build != nil {
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
				if err := d.Build(ctx, in.request.Image); err != nil {
					return err
				}
			}
			stop := func() {}
			vm, err := d.Machines.Create(ctx, in.request, func(s machines.Step) {
				stop()
				stop = io.Wait(s.Label)
			})
			stop()
			if err != nil {
				return err
			}
			return io.Answer(presentmachines.Created(vm, d.Context))
		},
	}, env)
}

func ask(ctx context.Context, d Deps, in *Input) (done bool, err error) {
	var hosts []prompt.HostChoice
	if fleet, fleetErr := d.Machines.Fleet(ctx, false); fleetErr == nil {
		for _, h := range fleet.Hosts {
			if h.Reachable {
				hosts = append(hosts, prompt.HostChoice{Name: h.Name, Here: h.Here, Detail: vmCount(len(h.VMs))})
			}
		}
	}
	answers, err := d.Ask.AskCreate(in.Lifetime, hosts)
	if err != nil {
		return false, err
	}
	in.request.Lifetime = machines.Lifetime(answers.Lifetime)
	for _, h := range hosts {
		if h.Name == answers.Host && !h.Here {
			return true, d.On(ctx, h.Name, answers.Lifetime)
		}
	}
	if in.buildFirst {
		build, err := d.Ask.AskBuild(in.request.Image)
		if err != nil {
			return false, err
		}
		if !build {
			return false, &machines.NoImageError{Image: in.request.Image}
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
