package wire

import (
	"context"
	"path/filepath"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	coremachine "github.com/jterrazz/studio-station/src/core/machine"
	model "github.com/jterrazz/studio-station/src/images"
	"github.com/jterrazz/studio-station/src/images/adapter/oci"
	imagescalls "github.com/jterrazz/studio-station/src/images/calls"
	imagescontract "github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/machines/adapter/openssh"
	"github.com/jterrazz/studio-station/src/platform/local"
	"github.com/jterrazz/studio-station/src/remote/each"
	imagescmd "github.com/jterrazz/studio-station/src/surface/cli/images"
	imagesbuild "github.com/jterrazz/studio-station/src/surface/cli/images/build"
	imagesdelete "github.com/jterrazz/studio-station/src/surface/cli/images/delete"
	imagesupdate "github.com/jterrazz/studio-station/src/surface/cli/images/update"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

func (m machine) images() *imagescalls.Service {
	user := m.host.Env.User()
	if current, err := m.stores.vmUser().Get(); err == nil && current != "" {
		user = current
	}
	hypervisor := m.hypervisor()
	return imagescalls.NewService(imagesMachine{h: m.host, keys: openssh.NewKeyGen(m.host.Run, m.host.FS)}, imagescalls.Contracts{
		Hypervisor: hypervisor, Guests: m.guestDialer(), Store: m.stores.images,
		Source: m.imageSource(hypervisor.Name()), Machines: m.stores.machines,
	}, catalog.Images, imagescalls.ServiceOptions{
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
	return oci.New(m.host.Net, "https://ghcr.io", "jterrazz", hypervisor)
}

func imagesCommand(env kit.Env, m machine) *cobra.Command {
	return imagescmd.Command(imagescmd.Deps{Here: m.where, Every: func(ctx context.Context) ([]model.Report, error) { return m.everyMacImages(ctx, env) }}, env,
		imagesbuild.Command(imagesbuild.Deps{Here: m.where}, env),
		imagesupdate.Command(imagesupdate.Deps{Here: m.where}, env),
		imagesdelete.Command(imagesdelete.Deps{Here: m.where}, env),
	)
}

type imageBuilds struct{ *imagescalls.Service }

func (s imageBuilds) Build(ctx context.Context, names []string, binary string, step func(label string)) ([]model.Record, error) {
	return s.Service.Build(ctx, model.BuildRequest{Images: names, Binary: binary}, func(st model.Step) { step(st.Label) })
}

func (s imageBuilds) Update(ctx context.Context, names []string, step func(label string)) ([]model.Update, error) {
	return s.Service.Update(ctx, names, func(st model.Step) { step(st.Label) })
}

// invariant: the images of This Mac, then of every other Mac that answers,
// each Mac's own; a Mac that does not answer has no row.
func (m machine) everyMacImages(ctx context.Context, env kit.Env) ([]model.Report, error) {
	here, err := call.Invoke(ctx, env.Calls, env.Self, imagescalls.List, imagescalls.ListIn{}, nil)
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
			here = append(here, got[mac].Value...)
		}
	}
	return here, nil
}

type imagesMachine struct {
	h    local.Host
	keys openssh.KeyGen
}

func (i imagesMachine) Now() time.Time { return i.h.Clock.Now() }
func (i imagesMachine) Sleep(ctx context.Context, d time.Duration) error {
	return i.h.Clock.Sleep(ctx, d)
}
func (i imagesMachine) Exists(p string) bool {
	_, err := i.h.FS.Stat(p)
	return err == nil
}
func (i imagesMachine) ReadFile(p string) ([]byte, error) { return i.h.FS.ReadFile(p) }
func (i imagesMachine) Keep(p string, data []byte) error {
	if err := i.h.FS.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return i.h.FS.WriteFile(p, data, 0o755)
}
func (i imagesMachine) Get(ctx context.Context, url string) (status int, body []byte, err error) {
	return i.h.Net.Get(ctx, url, nil)
}
func (i imagesMachine) MakeKey(ctx context.Context, p string) error { return i.keys.MakeKey(ctx, p) }
