package wire

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/machines/adapter/tart"
	imagescmd "github.com/jterrazz/studio-station/src/surface/cli/images"
	imagesbuild "github.com/jterrazz/studio-station/src/surface/cli/images/build"
	imagesdelete "github.com/jterrazz/studio-station/src/surface/cli/images/delete"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func (m machine) images() *images.Service {
	user := m.host.Env.User()
	if current, err := m.store.Load(); err == nil && current.VMUser != "" {
		user = current.VMUser
	}
	return images.New(m.host, tart.New(m.host.Run), catalog.Images, images.Options{
		Paths:   m.paths,
		Version: Version(),
		User:    user,
		Here:    m.kind,
	})
}

func imagesCommand(env kit.Env, m machine) *cobra.Command {
	service := m.images()
	return imagescmd.Command(imagescmd.Deps{Images: service, Here: m.where}, env,
		imagesbuild.Command(imagesbuild.Deps{Images: service, Here: m.where}, env),
		imagesdelete.Command(imagesdelete.Deps{Images: service, Here: m.where}, env),
	)
}
