package wire

import (
	"slices"
	"testing"
)

func TestAGuestStartedWithTheSystemPathFindsWhatStationInstalledThere(t *testing.T) {
	env := withAppBins([]string{"HOME=/home/e2e", "PATH=/usr/bin:/bin"}, true)
	want := "PATH=/usr/bin:/bin:/home/e2e/.local/bin:/home/e2e/.local/share/mise/shims:/home/e2e/.local/share/bun/bin:/home/e2e/.local/share/go/bin"
	if !slices.Contains(env, want) {
		t.Errorf("the guest's PATH: %v, want %s", env, want)
	}
	moved := withAppBins([]string{"HOME=/home/e2e", "MISE_DATA_DIR=/data/mise", "PATH=/home/e2e/.local/bin:/usr/bin"}, true)
	want = "PATH=/home/e2e/.local/bin:/usr/bin:/data/mise/shims:/home/e2e/.local/share/bun/bin:/home/e2e/.local/share/go/bin"
	if !slices.Contains(moved, want) {
		t.Errorf("with mise's data moved and ~/.local/bin already first: %v, want %s", moved, want)
	}
	mac := withAppBins([]string{"HOME=/Users/o", "PATH=/usr/bin"}, false)
	if !slices.Contains(mac, "PATH=/usr/bin:/Users/o/.local/share/bun/bin:/Users/o/.local/share/go/bin") {
		t.Errorf("on a Mac the shell block owns ~/.local/bin and mise: %v", mac)
	}
}
