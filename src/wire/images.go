package wire

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/domain/images"
	model "github.com/jterrazz/studio-station/src/images"
	"github.com/jterrazz/studio-station/src/images/adapter/oci"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
	imagescontract "github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/remote/each"
	imagescmd "github.com/jterrazz/studio-station/src/surface/cli/images"
	imagesbuild "github.com/jterrazz/studio-station/src/surface/cli/images/build"
	imagesdelete "github.com/jterrazz/studio-station/src/surface/cli/images/delete"
	imagesupdate "github.com/jterrazz/studio-station/src/surface/cli/images/update"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func (m machine) images() *images.Service {
	user := m.sys.Env.User()
	if current, err := m.stores.vmUser().Get(); err == nil && current != "" {
		user = current
	}
	hypervisor := m.hypervisor()
	return images.New(m.sys, images.Contracts{
		Hypervisor: hypervisor, Guests: m.guestDialer(), Store: m.stores.images,
		Source: m.imageSource(hypervisor.Name()), Machines: m.stores.machines,
	}, catalog.Images, images.Options{
		Cache: m.paths.Cache, Key: filepath.Join(m.paths.State, "images", "key"),
		Version:   Version(),
		User:      user,
		GuestPort: m.guestPort(),
		Mac:       m.profile.Ref.Name,
	})
}

// value: where station's images are published: ghcr, the owner's packages.
func (m machine) imageSource(hypervisor string) imagescontract.ImageSource {
	if m.world != nil {
		return m.world.imageSource(hypervisor)
	}
	return oci.New(m.sys.Net, "https://ghcr.io", "jterrazz", hypervisor)
}

func imagesCommand(env kit.Env, m machine) *cobra.Command {
	return imagescmd.Command(imagescmd.Deps{ThisMachine: m.where, Every: func(ctx context.Context) ([]model.Report, error) { return m.everyMacImages(ctx, env) }}, env,
		imagesbuild.Command(imagesbuild.Deps{ThisMachine: m.where}, env),
		imagesupdate.Command(imagesupdate.Deps{ThisMachine: m.where}, env),
		imagesdelete.Command(imagesdelete.Deps{ThisMachine: m.where}, env),
	)
}

type imageBuilder struct{ *images.Service }

func (s imageBuilder) Build(ctx context.Context, names []string, binary string, step func(label string)) ([]images.Record, error) {
	return s.Service.Build(ctx, images.BuildRequest{Images: names, Binary: binary}, func(st images.Step) { step(st.Label) })
}

func (s imageBuilder) Update(ctx context.Context, names []string, step func(label string)) ([]model.Update, error) {
	return s.Service.Update(ctx, names, func(st images.Step) { step(st.Label) })
}

// invariant: the images of This Mac, then of every other Mac that answers,
// each Mac's own; a Mac that does not answer has no row.
func (m machine) everyMacImages(ctx context.Context, env kit.Env) ([]model.Report, error) {
	thisMac, err := call.Invoke(ctx, env.Calls, env.Self, imagescalls.List, imagescalls.ListIn{}, nil)
	if err != nil {
		return nil, err
	}
	others := m.machines(false).OtherHosts()
	slices.Sort(others)
	got := each.All(ctx, others, func(ctx context.Context, mac string) ([]model.Report, error) {
		return call.Invoke(ctx, env.Calls, coremachine.Ref{Name: mac}, imagescalls.List, imagescalls.ListIn{}, nil)
	})
	for _, mac := range others {
		if got[mac].Err == nil {
			thisMac = append(thisMac, got[mac].Value...)
		}
	}
	return thisMac, nil
}
