package machinescreate

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
)

type Input struct {
	Lifetime, Image, Name, Memory, Disk string
	CPUs                                int
}

func Command(env kit.Env) *cobra.Command {
	cmd := kit.Command(kit.Leaf[Input]{
		Use:   "create",
		Short: "Make a VM on a host, from an image",
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.StringVar(&in.Lifetime, "lifetime", "", "temporary or lasting")
			f.StringVar(&in.Image, "image", "linux", "the image the VM is made from")
			f.StringVar(&in.Name, "name", "", "the VM's name")
			f.IntVar(&in.CPUs, "cpus", 0, "virtual CPUs")
			f.StringVar(&in.Memory, "memory", "", "memory, e.g. 4G")
			f.StringVar(&in.Disk, "disk", "", "disk size, e.g. 50G")
		},
		Bind: func(*Input, []string) error { return nil },
		Run:  func(context.Context, *Input, kit.IO) error { return kit.NotBuilt() },
	}, env)
	return cmd
}
