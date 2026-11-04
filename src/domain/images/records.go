package images

import (
	"context"
	"strconv"

	model "github.com/jterrazz/studio-station/src/images"
)

type (
	Record = model.Record
	Update = model.Update
)

const (
	imagePrefix = "station-image-"
	buildPrefix = "station-build-"
)

func DiskName(image, version string) string { return imagePrefix + image + "-" + version }

func (s *Service) records(image string) ([]Record, error) {
	return s.store.List(context.Background(), image)
}

func (s *Service) writeRecord(r Record) error { return s.store.Put(context.Background(), r) }

func (s *Service) nextVersion(image string) (string, error) {
	existing, err := s.records(image)
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(existing))
	for _, r := range existing {
		taken[r.Version] = true
	}
	day := s.h.Clock.Now().UTC().Format("2006.01.02")
	if !taken[day] {
		return day, nil
	}
	for n := 2; ; n++ {
		if candidate := day + "." + strconv.Itoa(n); !taken[candidate] {
			return candidate, nil
		}
	}
}
