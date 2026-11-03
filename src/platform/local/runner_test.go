package local

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

func TestReapKillsOnlyTheProcessThatWasRecorded(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command("/bin/sleep", "30")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); _ = cmd.Wait() })
	file := filepath.Join(dir, strconv.Itoa(cmd.Process.Pid))
	r := &runner{}

	recycled := time.Now().Add(-time.Hour).Unix()
	if err := os.WriteFile(file, []byte(strconv.FormatInt(recycled, 10)), 0o600); err != nil {
		t.Fatal(err)
	}
	if n := r.Reap(dir); n != 0 || syscall.Kill(cmd.Process.Pid, 0) != nil {
		t.Fatalf("a pid whose start does not match was killed (%d)", n)
	}

	if err := os.WriteFile(file, []byte(strconv.FormatInt(time.Now().Unix(), 10)), 0o600); err != nil {
		t.Fatal(err)
	}
	if n := r.Reap(dir); n != 1 {
		t.Fatalf("the recorded process was not killed (%d)", n)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("the record outlives its reap")
	}
}
