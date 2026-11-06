package calls

import (
	"context"
	"slices"
	"time"

	"github.com/jterrazz/studio-station/src/images"

	imagescontract "github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/machines/contract"

	"github.com/jterrazz/studio-station/src/catalog"
)

type BuilderOptions struct {
	// value: where the release binary is cached, and the build key.
	Cache, Key string
	Version    string
	User       string
	// value: the Mac this service holds the images of.
	Mac string
}

type Builder struct {
	m      Machine
	e      contract.Hypervisor
	builds BuildVMs
	store  imagescontract.ImageStore
	source imagescontract.ImageSource
	vms    contract.MachineStore
	list   []catalog.Image
	o      BuilderOptions
}

type Contracts struct {
	Hypervisor contract.Hypervisor
	Builds     BuildVMs
	Store      imagescontract.ImageStore
	Source     imagescontract.ImageSource
	Machines   contract.MachineStore
}

type BuildVMs interface {
	// value: the engine's build VMs: it boots a clone and holds the guest a
	// build speaks to; images/calls never reaches a VM but through it.
	Boot(ctx context.Context, vm string) error
	// value: the booted VM, its guest reached at the address it leased within
	// wait.
	Open(ctx context.Context, vm string, wait time.Duration) (BuildVM, error)
}

type BuildVM interface {
	Put(ctx context.Context, as Login, path string, data []byte, mode uint32) error
	Run(ctx context.Context, as Login, line, stdin string) (Ran, error)
	// invariant: lets go of every session the engine held for the build.
	Close() error
}

type Login struct {
	// value: who a build speaks to its VM as: the base's admin by its
	// password, or the VM's user by the image key.
	User, Key, Password string
}

type Ran struct {
	Code           int
	Stdout, Stderr string
}

func NewBuilder(m Machine, c Contracts, list []catalog.Image, o BuilderOptions) *Builder {
	return &Builder{m: m, e: c.Hypervisor, builds: c.Builds, store: c.Store, source: c.Source, vms: c.Machines, list: list, o: o}
}

func (s *Builder) image(name string) (catalog.Image, error) {
	for _, img := range s.list {
		if img.Name == name {
			return img, nil
		}
	}
	return catalog.Image{}, &images.UnknownError{Image: name}
}

func (s *Builder) List(ctx context.Context) ([]images.Report, error) {
	sizes := map[string]int{}
	if disks, err := s.e.Disks(ctx); err == nil {
		for _, d := range disks {
			sizes[d.VM] = int(d.Bytes / 1e9)
		}
	}
	users, err := s.users()
	if err != nil {
		return nil, err
	}
	var out []images.Report
	for _, img := range s.list {
		all, err := s.records(img.Name)
		if err != nil {
			return nil, err
		}
		for i, r := range all {
			if size, ok := sizes[DiskName(r.Name, r.Version)]; ok {
				r.SizeGB = size
			}
			out = append(out, images.Report{Record: r, Newest: i == 0, VMs: users[r.Name+"@"+r.Version], Mac: s.o.Mac})
		}
	}
	return out, nil
}

func (s *Builder) users() (map[string][]string, error) {
	records, err := s.vms.ListReadable(context.Background())
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, rec := range records {
		key := rec.Image + "@" + rec.Version
		out[key] = append(out[key], rec.Name)
	}
	for _, names := range out {
		slices.Sort(names)
	}
	return out, nil
}

func (s *Builder) CheckDelete(image, version string) error {
	if _, err := s.image(image); err != nil {
		return err
	}
	all, err := s.records(image)
	if err != nil {
		return err
	}
	if len(all) == 0 {
		return &images.NotBuiltError{Image: image}
	}
	if version != "" && !slices.ContainsFunc(all, func(r images.Record) bool { return r.Version == version }) {
		return &images.UnknownVersionError{Image: image, Version: version}
	}
	return nil
}

func (s *Builder) Delete(ctx context.Context, image, version string) ([]images.Record, error) {
	if err := s.CheckDelete(image, version); err != nil {
		return nil, err
	}
	if _, err := s.e.Version(ctx); err != nil {
		return nil, &images.NoEngineError{Hypervisor: s.e.Name(), Cause: err}
	}
	all, err := s.records(image)
	if err != nil {
		return nil, err
	}
	present, err := s.e.Disks(ctx)
	if err != nil {
		return nil, err
	}
	var removed []images.Record
	for _, r := range all {
		if version != "" && r.Version != version {
			continue
		}
		name := DiskName(r.Name, r.Version)
		if slices.ContainsFunc(present, func(d contract.Disk) bool { return d.VM == name }) {
			if err := s.e.Discard(ctx, s.location(name)); err != nil {
				return removed, err
			}
		}
		if err := s.store.Delete(ctx, image, r.Version); err != nil {
			return removed, err
		}
		removed = append(removed, r)
	}
	return removed, nil
}

func (s *Builder) location(ref string) imagescontract.ImageLocation {
	return imagescontract.ImageLocation{Hypervisor: s.e.Name(), Ref: ref}
}
