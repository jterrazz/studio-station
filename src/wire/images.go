package wire

import (
	"context"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/images"
	imagescmd "github.com/jterrazz/studio-station/src/surface/cli/images"
	imagesbuild "github.com/jterrazz/studio-station/src/surface/cli/images/build"
	imagesdelete "github.com/jterrazz/studio-station/src/surface/cli/images/delete"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func (m machine) images() *images.Service {
	user := m.host.Env.User()
	if current, err := m.stores.vmUser().Get(); err == nil && current != "" {
		user = current
	}
	return images.New(m.host, m.hypervisor(), m.guestDialer(), m.stores.images, m.stores.machines, catalog.Images, images.Options{
		Cache: m.paths.Cache, Key: filepath.Join(m.paths.State, "images", "key"),
		Version:   Version(),
		User:      user,
		Here:      m.kind,
		GuestPort: m.guestPort(),
	})
}

func imagesCommand(env kit.Env, m machine) *cobra.Command {
	return imagescmd.Command(imagescmd.Deps{Here: m.where}, env,
		imagesbuild.Command(imagesbuild.Deps{Here: m.where}, env),
		imagesdelete.Command(imagesdelete.Deps{Here: m.where}, env),
	)
}

type imageBuilds struct{ *images.Service }

func (s imageBuilds) Build(ctx context.Context, names []string, binary string, step func(label string)) ([]images.Record, error) {
	return s.Service.Build(ctx, images.BuildRequest{Images: names, Binary: binary}, func(st images.Step) { step(st.Label) })
}
