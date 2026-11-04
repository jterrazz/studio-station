package awakecmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type powerAwakeIn struct {
	Input
	For string
}

func PowerAwake(d Deps, env kit.Env) *cobra.Command {
	// value: v2's power awake [--for d] [--lid] [--battery], a keep-awake.
	return kit.Alias("awake", kit.Command(kit.Leaf[powerAwakeIn]{
		Use:   "awake",
		Short: "Keep this Mac awake for a while",
		Declare: func(f *pflag.FlagSet, in *powerAwakeIn) {
			f.BoolVar(&in.Lid, "lid", false, "stay awake with the lid closed too")
			f.BoolVar(&in.Battery, "battery", false, "stay awake when unplugged")
			f.StringVar(&in.For, "for", "4h", "how long, from 1m to 24h")
			kit.AliasPositional(f, "for")
		},
		Run: func(ctx context.Context, in *powerAwakeIn, io kit.IO) error {
			in.Words = in.For
			return keep(ctx, d, env, &in.Input, io)
		},
	}, env), "")
}
