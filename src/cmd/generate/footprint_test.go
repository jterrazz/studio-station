package main

import (
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
)

func TestAPathIsMarkedWithTheMachinesItLivesOn(t *testing.T) {
	for _, c := range []struct {
		kinds []catalog.MachineKind
		want  string
	}{
		{nil, ""},
		{[]catalog.MachineKind{catalog.Host, catalog.Linux, catalog.MacOS}, ""},
		{[]catalog.MachineKind{catalog.Host, catalog.MacOS}, "macOS"},
		{[]catalog.MachineKind{catalog.Linux, catalog.MacOS}, "vm"},
		{[]catalog.MachineKind{catalog.Linux}, "linux"},
		{[]catalog.MachineKind{catalog.Host}, "Mac"},
	} {
		if got := on(c.kinds); got != c.want {
			t.Errorf("on(%v) = %q, want %q", c.kinds, got, c.want)
		}
	}
}
