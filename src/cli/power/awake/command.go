package powerawake

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/power"
	"github.com/jterrazz/studio-station/src/page"
	pagepower "github.com/jterrazz/studio-station/src/pages/power"
)

type Deps struct {
	Power *power.Service
	Here  page.Context
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
			f.DurationVar(&in.For, "for", power.Default, "how long, from 1m to 24h")
		},
		Validate: func(_ context.Context, in *Input) error {
			in.forSet = flags.Changed("for")
			if err := power.CheckFor(in.For); err != nil {
				return err
			}
			return d.Power.Check()
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			a, err := d.Power.Awake(ctx, power.Request{Lid: in.Lid, Battery: in.Battery, For: in.For, By: by(in)})
			if err != nil {
				return err
			}
			return io.Page(pagepower.Awake(a, d.Power.Zone(), d.Here), a)
		},
	}, env)
}
