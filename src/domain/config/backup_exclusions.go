package config

import (
	"context"
	"errors"
	"path"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

func (s *Service) excluded() []string {
	return []string{s.o.Paths.Roots.Cache, path.Join(s.o.Paths.Data, "vms")}
}

func (s *Service) installBackupExclusions(ctx context.Context) (Change, error) {
	paths := s.excluded()
	for _, p := range paths {
		if err := s.h.FS.MkdirAll(p, 0o755); err != nil {
			return Change{}, err
		}
	}
	if _, err := s.must(ctx, "tmutil", append([]string{"addexclusion"}, paths...)...); err != nil {
		return Change{}, err
	}
	return Change{Steps: []Step{okStep("Time Machine", "excludes "+strings.Join(paths, ", "))}}, nil
}

func (s *Service) uninstallBackupExclusions(ctx context.Context) (Change, error) {
	paths := s.excluded()
	if _, err := s.must(ctx, "tmutil", append([]string{"removeexclusion"}, paths...)...); err != nil {
		return Change{}, err
	}
	return Change{Steps: []Step{okStep("Time Machine", "backs up "+strings.Join(paths, ", ")+" again")}}, nil
}

func (s *Service) checkBackupExclusions(ctx context.Context) (state catalog.State, detail string) {
	paths := s.excluded()
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "tmutil", Args: append([]string{"isexcluded"}, paths...), Timeout: probeBudget})
	if errors.Is(err, system.ErrNotFound) {
		return catalog.StateUnknown, "tmutil not found"
	}
	if err != nil {
		return catalog.StateUnknown, err.Error()
	}
	n := 0
	for _, line := range strings.Split(out.Stdout, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "[Excluded]") {
			n++
		}
	}
	switch {
	case n >= len(paths):
		return catalog.StateOK, "excluded from Time Machine"
	case n > 0:
		return catalog.StateDegraded, "one of the two is backed up"
	}
	return catalog.StateAbsent, ""
}
