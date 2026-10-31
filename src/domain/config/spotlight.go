package config

import (
	"context"
	"errors"
	"io/fs"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

func (s *Service) checkSpotlight(ctx context.Context) (state catalog.State, detail string) {
	if s.spotlightExcluded(ctx, s.home("~/Developer")) {
		return catalog.StateOK, "~/Developer in the Spotlight Privacy list"
	}
	return catalog.StateAbsent, ""
}

func (s *Service) installSpotlight(ctx context.Context) (Change, error) {
	dev := s.home("~/Developer")
	if !s.exists(dev) {
		return Change{}, errors.New("~/Developer does not exist")
	}
	if s.spotlightExcluded(ctx, dev) {
		return Change{Steps: []Step{okStep("~/Developer", "already excluded")}}, nil
	}
	// outside: a .metadata_never_index marker never worked inside a sub-folder, only at
	// volume root, so a stale one is removed.
	_ = s.h.FS.Remove(dev + "/.metadata_never_index")
	if _, err := s.must(ctx, "open", "x-apple.systempreferences:com.apple.Spotlight-Settings.extension"); err != nil {
		return Change{}, errors.New("opening System Settings failed: open Spotlight > Search Privacy by hand")
	}
	return Change{Steps: []Step{
		okStep("System Settings", "opened at Spotlight > Search Privacy"),
		skipStep("next", "click +, pick ~/Developer, then run station config to confirm"),
	}}, nil
}

// outside: on Tahoe (26+) the Privacy list lives in a root-only plist and
// `defaults read com.apple.Spotlight Exclusions` says "does not exist", so the check
// probes instead: mdfind finds a known file inside dir unless dir is excluded.
func (s *Service) spotlightExcluded(ctx context.Context, dir string) bool {
	probe := s.spotlightProbe(dir)
	if probe == "" {
		return false
	}
	slash := strings.LastIndex(probe, "/")
	out, ok := s.probe(ctx, "mdfind", "-onlyin", probe[:slash], "kMDItemFSName == "+strconv.Quote(probe[slash+1:]))
	return ok && out == ""
}

// hazard: hidden files are skipped: Spotlight ignores them by default, so they would read as excluded.
func (s *Service) spotlightProbe(dir string) string {
	var probe string
	_ = s.h.FS.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			//nolint:nilerr // an unreadable entry is skipped; the probe is the first file that IS readable
			return nil
		}
		if p != dir && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			probe = p
			return fs.SkipAll
		}
		return nil
	})
	return probe
}
