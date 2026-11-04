package mas_test

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/apps/adapter/mas"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/apps/contract/contracttest"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/platform/local"
)

var line = regexp.MustCompile(`^mas "([^"]*)", id: (\d+)$`)

// value: the App Store as these tests play it, and the bundle that installs
// its lines.
type store struct {
	apps  map[string]string
	fails bool
}

func (s *store) play(c local.Cmd) (local.Output, error) {
	args := strings.Join(c.Args, " ")
	switch {
	case args == "list":
		out := make([]string, 0, len(s.apps))
		for id, version := range s.apps {
			out = append(out, id+"  App  ("+version+")")
		}
		return local.Output{Stdout: strings.Join(out, "\n")}, nil
	case strings.HasPrefix(args, "uninstall "):
		delete(s.apps, strings.TrimPrefix(args, "uninstall "))
		return local.Output{}, nil
	}
	return local.Output{ExitCode: 97}, nil
}

func (s *store) Name() string { return "the bundle" }

func (s *store) Bundle(_ context.Context, lines []string, _ bool, _ call.Progress) error {
	if s.fails {
		return errors.New("the bundle exited 1")
	}
	for _, l := range lines {
		if m := line.FindStringSubmatch(l); m != nil {
			s.apps[m[2]] = "1.0"
		}
	}
	return nil
}

func TestInstaller(t *testing.T) {
	contracttest.Installer(t, catalog.Mas("Xcode", 497799835), func(_ *testing.T, seed contracttest.Seed) contract.Installer {
		s := &store{apps: map[string]string{}, fails: seed.Fails}
		for k, v := range seed.Installed {
			s.apps[k] = v
		}
		names := []string{"mas"}
		if seed.Absent {
			names = nil
		}
		return mas.New(local.Played(s.play, names...), s)
	})
}

func TestAnAppIsListedByItsIDWithTheVersionInParentheses(t *testing.T) {
	run := local.Played(func(local.Cmd) (local.Output, error) {
		return local.Output{Stdout: "497799835  Xcode  (26.1)\n409183694  Keynote 14  (14.4)\n"}, nil
	}, "mas")

	got, err := mas.New(run, nil).Inventory(context.Background())

	if err != nil || got["497799835"] != "26.1" || got["409183694"] != "14.4" {
		t.Fatalf("inventory %v %v", got, err)
	}
}

func TestABareUpdateLeavesTheAppStoreItsOwnApps(t *testing.T) {
	if mas.New(nil, nil).Can().UpdateAll {
		t.Error("a bare update reaches the App Store's apps")
	}
}
