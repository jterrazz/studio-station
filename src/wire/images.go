package wire

import (
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/machines/adapter/ssh"
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
	return images.New(m.host, m.hypervisor(), ssh.New(), m.stores.images, m.stores.machines, catalog.Images, images.Options{
		Cache: m.paths.Cache, Key: filepath.Join(m.paths.State, "images", "key"),
		Version:   Version(),
		User:      user,
		Here:      m.kind,
		GuestPort: m.guestPort(),
	})
}

func imagesCommand(env kit.Env, m machine) *cobra.Command {
	service := m.images()
	return imagescmd.Command(imagescmd.Deps{Images: service, Here: m.where}, env,
		imagesbuild.Command(imagesbuild.Deps{Images: service, Here: m.where}, env),
		imagesdelete.Command(imagesdelete.Deps{Images: service, Here: m.where}, env),
	)
}
