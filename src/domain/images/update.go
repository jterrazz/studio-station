package images

import (
	"context"
	"errors"
	"slices"
	"time"

	model "github.com/jterrazz/studio-station/src/images"
	imagescontract "github.com/jterrazz/studio-station/src/images/contract"
)

// value: bare, every image this Mac holds a version of; named, those.
func (s *Service) updating(names []string) ([]string, error) {
	if len(names) > 0 {
		var out []string
		for _, name := range names {
			if _, err := s.image(name); err != nil {
				return nil, err
			}
			if !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
		return out, nil
	}
	var out []string
	for _, img := range s.list {
		held, err := s.records(img.Name)
		if err != nil {
			return nil, err
		}
		if len(held) > 0 {
			out = append(out, img.Name)
		}
	}
	return out, nil
}

func (s *Service) Update(ctx context.Context, names []string, progress func(Step)) ([]model.Update, error) {
	wanted, err := s.updating(names)
	if err != nil || len(wanted) == 0 {
		return nil, err
	}
	if _, err := s.e.Version(ctx); err != nil {
		return nil, &NoEngineError{Hypervisor: s.e.Name(), Cause: err}
	}
	if progress == nil {
		progress = func(Step) {}
	}
	var out []model.Update
	for _, name := range wanted {
		u, err := s.updateOne(ctx, name, progress)
		if err != nil {
			return out, err
		}
		out = append(out, u)
	}
	return out, nil
}

// invariant: an update fetches only a version newer than every one held;
// it lands whole as an image disk or not at all, so a failed fetch leaves
// the image as it was.
func (s *Service) updateOne(ctx context.Context, name string, progress func(Step)) (model.Update, error) {
	latest, err := s.source.Latest(ctx, name)
	switch {
	case errors.Is(err, imagescontract.ErrNotPublished):
		return model.Update{}, &NotPublishedError{Image: name}
	case err != nil:
		return model.Update{}, &RegistryError{Image: name, Cause: err}
	}
	held, err := s.records(name)
	if err != nil {
		return model.Update{}, err
	}
	if len(held) > 0 && model.CompareVersions(held[0].Version, latest.Version) >= 0 {
		return model.Update{Ref: held[0].Ref(), Newest: true}, nil
	}
	at, err := s.source.Locate(ctx, latest, s.e.Name())
	if errors.Is(err, imagescontract.ErrNotForHypervisor) {
		return model.Update{}, &NotPublishedError{Image: name}
	}
	if err != nil {
		return model.Update{}, &RegistryError{Image: name, Cause: err}
	}
	failed := func(step string, cause error) (model.Update, error) {
		return model.Update{}, &FetchFailedError{Image: name, Version: latest.Version, Step: step, Cause: cause}
	}
	progress(Step{Label: "fetching " + name + " " + latest.Version})
	if err := s.e.Fetch(ctx, at, nil); err != nil {
		return failed("fetching", err)
	}
	disk := DiskName(name, latest.Version)
	progress(Step{Label: "recording " + name + " " + latest.Version})
	if err := s.e.Clone(ctx, at, disk); err != nil {
		_ = s.e.Discard(context.WithoutCancel(ctx), at)
		return failed("keeping the fetched image", err)
	}
	_ = s.e.Discard(ctx, at)
	rec := Record{Name: name, Version: latest.Version, BuiltAt: s.h.Clock.Now().UTC().Truncate(time.Second), Published: true}
	if img, err := s.image(name); err == nil {
		rec.Base = img.Base
	}
	if disks, err := s.e.Disks(ctx); err == nil {
		for _, d := range disks {
			if d.VM == disk {
				rec.SizeGB = int(d.Bytes / 1e9)
			}
		}
	}
	if err := s.writeRecord(rec); err != nil {
		_ = s.e.Delete(context.WithoutCancel(ctx), disk)
		return failed("writing the image record", err)
	}
	return model.Update{Ref: rec.Ref()}, nil
}
