package contracttest

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/agents/contract"
)

type SessionSubject struct {
	Session contract.Session
	// value: the programs the session runs, each "<folder> <argv>", in the
	// order opened.
	Opened func() []string
	// value: the texts typed into the session, each sent with Enter.
	Typed func() []string
	// value: every command line the adapter ran.
	Lines func() []string
}

func Session(t *testing.T, open func(t *testing.T) SessionSubject) {
	ctx := context.Background()

	t.Run("a program opened runs in its folder, and a second one beside it", func(t *testing.T) {
		s := open(t)
		for _, folder := range []string{"/Users/admin/app", "/Users/admin/api"} {
			if err := s.Session.Open(ctx, folder, []string{"agent"}); err != nil {
				t.Fatalf("Open: %v", err)
			}
		}
		if got := s.Opened(); !slices.Equal(got, []string{"/Users/admin/app agent", "/Users/admin/api agent"}) {
			t.Errorf("opened %q, want both, in their folders", got)
		}
	})

	t.Run("a text sent is typed once with Enter, and never on a command line", func(t *testing.T) {
		s := open(t)
		if err := s.Session.Open(ctx, "/Users/admin/app", []string{"agent"}); err != nil {
			t.Fatalf("Open: %v", err)
		}
		text := "yes, merge it; then run `rm -rf build` && say \"done\" $HOME"
		if err := s.Session.Send(ctx, text); err != nil {
			t.Fatalf("Send: %v", err)
		}
		if got := s.Typed(); !slices.Equal(got, []string{text}) {
			t.Errorf("typed %q, want the text once", got)
		}
		for _, line := range s.Lines() {
			if strings.Contains(line, "merge it") {
				t.Errorf("the text went on a command line: %q", line)
			}
		}
	})
}
