package powerawake

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/awake"
	awakecalls "github.com/jterrazz/studio-station/src/awake/calls"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	// value: the zone the page shows the session's end in.
	Zone *time.Location
	Here page.Context
}

type Input struct {
	Lid, Battery bool
	For          time.Duration
	forSet       bool
}

func by(in *Input) string {
	words := "station power awake"
	if in.Lid {
		words += " --lid"
	}
	if in.Battery {
		words += " --battery"
	}
	if in.forSet {
		words += " --for " + strings.TrimSuffix(strings.TrimSuffix(in.For.String(), "0s"), "0m")
	}
	return words
}

func Command(d Deps, env kit.Env) *cobra.Command {
	var flags *pflag.FlagSet
	return kit.Command(kit.Leaf[Input]{
		Use:     "awake",
		Short:   "Keep this host awake: 4h by default, at most 24h; --lid with the lid closed too",
		Example: "  station power awake\n  station power awake --lid --for 2h\n  station power awake --lid --battery",
		Declare: func(f *pflag.FlagSet, in *Input) {
			flags = f
			f.BoolVar(&in.Lid, "lid", false, "also stay awake with the lid closed (the display sleeps)")
			f.BoolVar(&in.Battery, "battery", false, "allow the lid closed on battery for this session")
			f.DurationVar(&in.For, "for", awake.DefaultFor, "how long, from 1m to 24h")
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			in.forSet = flags.Changed("for")
			a, err := call.Invoke(ctx, env.Calls, env.Target(), awakecalls.Keep, awake.Request{Lid: in.Lid, Battery: in.Battery, For: in.For, By: by(in)}, nil)
			if err != nil {
				return err
			}
			return io.Page(pagepower.Awake(a, d.Zone, d.Here), a)
		},
	}, env)
}
