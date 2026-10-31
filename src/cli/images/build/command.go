package imagesbuild

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/images"
	presentimages "github.com/jterrazz/studio-station/src/present/images"
)

type Deps struct {
	Images  *images.Service
	Context string
}

type Input struct {
	Names  []string
	Binary string
}

func Command(d Deps, env kit.Env) *cobra.Command {
	return kit.Command(kit.Leaf[Input]{
		Use:       "build [<image>...]",
		Short:     "Build images; bare, every image that is missing or whose recipe changed",
		Example:   "  station images build\n  station images build linux --binary .artifacts/go/station-linux-arm64",
		Args:      cobra.ArbitraryArgs,
		ValidArgs: imageNames(),
		Declare: func(f *pflag.FlagSet, in *Input) {
			f.StringVar(&in.Binary, "binary", "", "the linux station binary to put in the image (a dev build needs one: make build-linux)")
		},
		Bind: func(in *Input, args []string) error {
			in.Names = args
			return nil
		},
		Run: func(ctx context.Context, in *Input, io kit.IO) error {
			stop := func() {}
			records, err := d.Images.Build(ctx, images.BuildRequest{Images: in.Names, Binary: in.Binary}, func(s images.Step) {
				stop()
				stop = io.Wait(s.Label)
			})
			stop()
			if err != nil {
				return err
			}
			return io.Answer(presentimages.Built(records, d.Context))
		},
	}, env)
}

func imageNames() []string {
	names := make([]string, 0, len(catalog.Images))
	for _, img := range catalog.Images {
		names = append(names, img.Name)
	}
	return names
}
