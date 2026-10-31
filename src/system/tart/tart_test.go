package tart

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

func recorded(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func newEngine(t *testing.T) (system.Engine, *systemtest.Runner) {
	run := systemtest.NewRunner(t)
	run.Paths["tart"] = "/home/o/.local/bin/tart"
	return New(run), run
}

func TestListKeepsLocalVMsAndMapsTheirShape(t *testing.T) {
	eng, run := newEngine(t)
	run.On("/home/o/.local/bin/tart", "list", "--source", "local", "--format", "json").Reply(system.Output{Stdout: recorded(t, "list.json")})
	vms, err := eng.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	disk20, disk50 := 20, 50
	want := []system.VMInfo{
		{Name: "station-image-ubuntu-1", State: system.VMStopped, DiskGB: &disk20, SizeGB: 7},
		{Name: "dev", State: system.VMRunning, DiskGB: &disk50, SizeGB: 12},
		{Name: "paused", State: system.VMSuspended, DiskGB: nil, SizeGB: 9},
	}
	if len(vms) != len(want) {
		t.Fatalf("got %d VMs (%+v), want the OCI entry dropped", len(vms), vms)
	}
	for i, w := range want {
		g := vms[i]
		if g.Name != w.Name || g.State != w.State || g.SizeGB != w.SizeGB || (g.DiskGB == nil) != (w.DiskGB == nil) || (w.DiskGB != nil && *g.DiskGB != *w.DiskGB) {
			t.Errorf("vm %d = %+v, want %+v", i, g, w)
		}
	}
}

func TestVersionIsTheTrimmedOutput(t *testing.T) {
	eng, run := newEngine(t)
	run.On("/home/o/.local/bin/tart", "--version").Reply(system.Output{Stdout: recorded(t, "version.txt")})
	v, err := eng.Version(context.Background())
	if err != nil || v != "2.41.1" {
		t.Fatalf("version = %q, %v", v, err)
	}
}

func TestIPTimeoutIsErrTimeout(t *testing.T) {
	eng, run := newEngine(t)
	run.On("/home/o/.local/bin/tart", "ip", "dev", "--wait", "30").Reply(system.Output{ExitCode: 1, Stderr: "Failed to get IP"})
	_, err := eng.IP(context.Background(), "dev", 30*time.Second)
	if !errors.Is(err, system.ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
}

func TestIPIsTheTrimmedAddress(t *testing.T) {
	eng, run := newEngine(t)
	run.On("/home/o/.local/bin/tart", "ip", "dev", "--wait", "5").Reply(system.Output{Stdout: "192.168.64.7\n"})
	ip, err := eng.IP(context.Background(), "dev", 5*time.Second)
	if err != nil || ip != "192.168.64.7" {
		t.Fatalf("ip = %q, %v", ip, err)
	}
}

func TestRunArgvCarriesTheAbsoluteTartAndTheConsole(t *testing.T) {
	eng, _ := newEngine(t)
	got := eng.RunArgv("dev", "/v/dev.console")
	want := []string{"/home/o/.local/bin/tart", "run", "--no-graphics", "--no-audio", "--serial-path", "/v/dev.console", "dev"}
	if !slices.Equal(got, want) {
		t.Fatalf("argv = %v, want %v", got, want)
	}
}

func TestResizeSendsOnlyTheAskedResources(t *testing.T) {
	eng, run := newEngine(t)
	run.On("/home/o/.local/bin/tart", "set", "dev", "--cpu", "4", "--memory", "8192", "--disk-size", "60").Reply(system.Output{})
	err := eng.Resize(context.Background(), "dev", system.Resources{CPUs: 4, MemoryBytes: 8 << 30, DiskBytes: 60_000_000_000})
	if err != nil {
		t.Fatal(err)
	}
}

func TestAFailedCommandCarriesItsStderr(t *testing.T) {
	eng, run := newEngine(t)
	run.On("/home/o/.local/bin/tart", "delete", "dev").Reply(system.Output{ExitCode: 1, Stderr: "no such VM"})
	err := eng.Delete(context.Background(), "dev")
	if err == nil || !strings.Contains(err.Error(), "no such VM") {
		t.Fatalf("err = %v", err)
	}
}
