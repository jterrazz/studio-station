//go:build darwin && cgo

package darwin

import (
	"os/exec"
	"testing"
	"time"
)

func TestAProcessReadByItsPidSaysWhenItStarted(t *testing.T) {
	before := time.Now().Add(-time.Second)
	child := exec.Command("/bin/sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	got, err := Kernel{}.Process(child.Process.Pid)
	if err != nil || len(got.List) != 1 {
		t.Fatalf("Process(child) = %+v, %v; want the child's process", got, err)
	}
	if started := got.List[0].Started; started.Before(before) || started.After(time.Now()) {
		t.Errorf("Started = %v; want the child's start, after %v", started, before)
	}
}
