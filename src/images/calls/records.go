package calls

import (
	"context"
	"strconv"

	"github.com/jterrazz/studio-station/src/images"
)

const (
	imagePrefix = "station-image-"
	buildPrefix = "station-build-"
)

func DiskName(image, version string) string { return imagePrefix + image + "-" + version }

func (s *Builder) records(image string) ([]images.Record, error) {
	return s.store.List(context.Background(), image)
}

func (s *Builder) writeRecord(r images.Record) error { return s.store.Put(context.Background(), r) }

func (s *Builder) nextVersion(image string) (string, error) {
	existing, err := s.records(image)
	if err != nil {
		return "", err
	}
	taken := make(map[string]bool, len(existing))
	for _, r := range existing {
		taken[r.Version] = true
	}
	day := s.m.Now().UTC().Format("2006.01.02")
	if !taken[day] {
		return day, nil
	}
	for n := 2; ; n++ {
		if candidate := day + "." + strconv.Itoa(n); !taken[candidate] {
			return candidate, nil
		}
	}
}
