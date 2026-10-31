package kit_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/fail"
	"github.com/jterrazz/studio-station/src/output"
)

type answer struct {
	yes bool
	err error
}

func (a answer) Confirm(string) (bool, error) { return a.yes, a.err }

func TestTheConfirmGateOnATerminal(t *testing.T) {
	for _, c := range []struct {
		name   string
		answer answer
		ran    bool
		code   fail.Code
		stderr string
	}{
		{"yes runs the leaf", answer{yes: true}, true, fail.CodeOK, ""},
		{"no is cancelled", answer{yes: false}, false, fail.CodeFailure,
			"Error: uninstall cancelled\nHint: re-run with --yes to skip the question\n"},
		{"ctrl-c is interrupted, with no block", answer{err: context.Canceled}, false, fail.CodeInterrupted, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			var stderr bytes.Buffer
			out := output.NewWriter(output.Stream{W: &bytes.Buffer{}}, output.Stream{W: &stderr}, map[string]string{})
			ran := false
			uninstall := func(context.Context, *struct{}, kit.IO) error {
				ran = true
				return nil
			}
			cmd := kit.Command(kit.Leaf[struct{}]{
				Use:         "uninstall",
				Destructive: func(*struct{}) bool { return true },
				Confirm:     func(*struct{}) string { return "Uninstall node?" },
				Run:         uninstall,
			}, kit.Env{Out: out, Prompt: c.answer, Interactive: true})
			cmd.SetArgs(nil)
			cmd.SilenceErrors, cmd.SilenceUsage = true, true
			err := cmd.Execute()
			out.Failure(err)
			if ran != c.ran {
				t.Errorf("the leaf ran: %v, want %v", ran, c.ran)
			}
			if got := fail.CodeOf(err); got != c.code {
				t.Errorf("exit code %d, want %d", got, c.code)
			}
			if stderr.String() != c.stderr {
				t.Errorf("stderr %q, want %q", stderr.String(), c.stderr)
			}
		})
	}
}
