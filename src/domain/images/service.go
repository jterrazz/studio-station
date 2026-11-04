package images

import (
	"context"
	"slices"

	imagescontract "github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/machines/contract"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

type Options struct {
	// value: where the release binary is cached, and the build key.
	Cache, Key string
	Version    string
	User       string
	Here       catalog.MachineKind
	GuestPort  int
}

type Service struct {
	h      system.Host
	e      contract.Hypervisor
	guests contract.GuestDialer
	store  imagescontract.ImageStore
	vms    contract.MachineStore
	list   []catalog.Image
	o      Options
}

func New(h system.Host, e contract.Hypervisor, guests contract.GuestDialer, store imagescontract.ImageStore, vms contract.MachineStore, list []catalog.Image, o Options) *Service {
	return &Service{h: h, e: e, guests: guests, store: store, vms: vms, list: list, o: o}
}

type Report struct {
	Record Record
	Newest bool
	VMs    []string
}

func (s *Service) checkHost() error {
	if s.o.Here != catalog.Host {
		return &NotAHostError{Here: s.o.Here}
	}
	return nil
}

func (s *Service) image(name string) (catalog.Image, error) {
	for _, img := range s.list {
		if img.Name == name {
			return img, nil
		}
	}
	return catalog.Image{}, &UnknownImageError{Image: name}
}

func (s *Service) List(ctx context.Context) ([]Report, error) {
	if err := s.checkHost(); err != nil {
		return nil, err
	}
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
	var out []Report
	for _, img := range s.list {
		all, err := s.records(img.Name)
		if err != nil {
			return nil, err
		}
		for i, r := range all {
			if size, ok := sizes[DiskName(r.Name, r.Version)]; ok {
				r.SizeGB = size
			}
			out = append(out, Report{Record: r, Newest: i == 0, VMs: users[r.Name+"@"+r.Version]})
		}
	}
	return out, nil
}

func (s *Service) users() (map[string][]string, error) {
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

func (s *Service) CheckDelete(image, version string) error {
	if err := s.checkHost(); err != nil {
		return err
	}
	if _, err := s.image(image); err != nil {
		return err
	}
	all, err := s.records(image)
	if err != nil {
		return err
	}
	if len(all) == 0 {
		return &NotBuiltError{Image: image}
	}
	if version != "" && !slices.ContainsFunc(all, func(r Record) bool { return r.Version == version }) {
		return &UnknownVersionError{Image: image, Version: version}
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, image, version string) ([]Record, error) {
	if err := s.CheckDelete(image, version); err != nil {
		return nil, err
	}
	if _, err := s.e.Version(ctx); err != nil {
		return nil, &NoEngineError{Hypervisor: s.e.Name(), Cause: err}
	}
	all, err := s.records(image)
	if err != nil {
		return nil, err
	}
	present, err := s.e.Disks(ctx)
	if err != nil {
		return nil, err
	}
	var removed []Record
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

func (s *Service) location(ref string) imagescontract.ImageLocation {
	return imagescontract.ImageLocation{Hypervisor: s.e.Name(), Ref: ref}
}
