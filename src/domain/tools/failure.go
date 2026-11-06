package tools

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

// invariant: an installer's own words (its stderr, its exit) go to the log
// and only there (SPEC §12.3); the outcome says, in station's words, what
// failed and where the tool's output is kept.
func (s *Service) failed(verb string, list []catalog.App, cause error) string {
	words := "the " + verb + " failed"
	if s.o.Log == "" || s.keep(verb, list, cause) != nil {
		return words
	}
	shown := s.o.Log
	if home := s.h.Env.Home(); home != "" && strings.HasPrefix(shown, home+"/") {
		shown = "~" + strings.TrimPrefix(shown, home)
	}
	return words + ": its output is in " + shown
}

func (s *Service) keep(verb string, list []catalog.App, cause error) error {
	names := make([]string, 0, len(list))
	for _, t := range list {
		names = append(names, t.Name)
	}
	before, err := s.h.FS.ReadFile(s.o.Log)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := s.h.FS.MkdirAll(path.Dir(s.o.Log), 0o700); err != nil {
		return err
	}
	line := s.h.Clock.Now().UTC().Format(time.RFC3339) + " " + verb + " " + strings.Join(names, " ") + ": " + cause.Error() + "\n"
	return s.h.FS.WriteFile(s.o.Log, append(before, line...), 0o600)
}
