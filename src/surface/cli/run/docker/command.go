package rundocker

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/shortcuts"
	"github.com/jterrazz/studio-station/src/page"
	pageshortcuts "github.com/jterrazz/studio-station/src/pages/shortcuts"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Deps struct {
	Shortcuts *shortcuts.Service
	Here      page.Context
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:         "docker <" + strings.Join(shortcutNames(), "|") + ">",
		Short:       "Delete every docker container, every image, or both",
		Example:     "  station run docker rm --yes\n  station run docker reset --yes",
		Args:        cobra.ArbitraryArgs,
		ValidArgs:   shortcutNames(),
		Bind:        bind,
		Destructive: func(*Input) bool { return true },
		Confirm:     func(in *Input) string { return "run docker " + in.Name + "?" },
		Verb:        verb,
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			outcome, err := d.Shortcuts.Docker(ctx, in.Name)
			if err != nil {
				return err
			}
			return io.Page(pageshortcuts.Docker(outcome, d.Here), nil)
		},
	}, env)
}
