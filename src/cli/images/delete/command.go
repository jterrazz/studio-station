package imagesdelete

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/images"
	presentimages "github.com/jterrazz/studio-station/src/present/images"
)

type Deps struct {
	Images  *images.Service
	Context string
}

type Input struct{ Image, Version string }

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:     "delete <image>[@<version>]",
		Short:   "Delete one version of an image, or every version; the VMs made from it stay",
		Example: "  station images delete linux@2026.11.01 --yes\n  station images delete linux --yes",
		Args:    cobra.ExactArgs(1),
		Bind: func(in *Input, args []string) error {
			in.Image, in.Version, _ = strings.Cut(args[0], "@")
			return nil
		},
		Complete: func([]string, string) []string {
			names := make([]string, 0, len(catalog.Images))
			for _, img := range catalog.Images {
				names = append(names, img.Name)
			}
			return names
		},
		Validate:    func(_ context.Context, in *Input) error { return d.Images.CheckDelete(in.Image, in.Version) },
		Destructive: func(*Input) bool { return true },
		Confirm: func(in *Input) string {
			if in.Version == "" {
				return "Delete every version of " + in.Image + "?"
			}
			return "Delete " + in.Image + "@" + in.Version + "?"
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			removed, err := d.Images.Delete(ctx, in.Image, in.Version)
			if err != nil {
				return err
			}
			return io.Answer(presentimages.Deleted(removed, d.Context))
		},
	}, env)
}
