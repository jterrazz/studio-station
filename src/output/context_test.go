package output_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/output"
)

func TestNoColorDegradesATerminalToThePipeForm(t *testing.T) {
	terminal := output.Stream{IsTerminal: true, Width: 120}
	if got := output.Detect(terminal, map[string]string{}, false).Mode; got != output.TTY {
		t.Fatalf("a terminal without NO_COLOR is mode %v, want TTY", got)
	}
	if got := output.Detect(terminal, map[string]string{"NO_COLOR": "1"}, false).Mode; got != output.Pipe {
		t.Fatalf("a terminal with NO_COLOR=1 is mode %v, want Pipe", got)
	}
}
