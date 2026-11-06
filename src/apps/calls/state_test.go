package calls_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/apps/adapter/fake"
	"github.com/jterrazz/studio-station/src/apps/calls"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	probe "github.com/jterrazz/studio-station/src/core/call/lanes"
)

type probed struct {
	output string
	code   int
}

type machine struct {
	files  map[string]bool
	paths  map[string]string
	probes map[string]probed
}

func (machine) Now() time.Time { return time.Unix(0, 0) }

func (machine) Sleep(ctx context.Context, _ time.Duration) error {
	<-ctx.Done()
	return ctx.Err()
}

func (machine) Home() string              { return "/home/u" }
func (machine) Glob(string) []string      { return nil }
func (m machine) Exists(path string) bool { return m.files[path] }
func (machine) Keep(string, string) error { return nil }
func (m machine) LookPath(name string) (string, bool) {
	where, ok := m.paths[name]
	return where, ok
}

func (m machine) Probe(_ context.Context, name string, args []string, _ time.Duration) (output string, code int, ran bool) {
	p, ok := m.probes[strings.Join(append([]string{name}, args...), " ")]
	return p.output, p.code, ok
}

func reportOf(t *testing.T, tool catalog.Tool, manager *fake.State, m machine) apps.Report {
	method := tool.Installs[0].Install.Method()
	installers := contract.Installers{method: fake.New(method, manager)}
	reports, err := calls.NewService(m, []catalog.Tool{tool}, nil, calls.ServiceOptions{Here: catalog.Host, Root: "/", Installers: installers, Pool: probe.NewPool(context.Background(), m, 16, 1)}).Inventory(context.Background())
	if err != nil || len(reports) != 1 {
		t.Fatalf("inventory: %v %v", reports, err)
	}
	return reports[0]
}

func java(output string, code int) machine {
	return machine{paths: map[string]string{"java": "/usr/bin/java"}, probes: map[string]probed{"java -version": {output, code}}}
}

var javaTool = catalog.Tool{Name: "java", Installs: catalog.Anywhere(catalog.Cask("temurin")), Provides: []string{"java"}, Version: catalog.Probe{Args: []string{"java", "-version"}, Pattern: `version "([^"]+)"`}}

func TestStateAnAppPresentWithoutItsManagerIsDegraded(t *testing.T) {
	tool := catalog.Tool{Name: "keynote", Installs: catalog.HostOnly(catalog.Cask("keynote")), Bundle: "Keynote"}

	got := reportOf(t, tool, &fake.State{Absent: true}, machine{files: map[string]bool{"/Applications/Keynote.app": true}})

	if got.State != catalog.StateDegraded || got.Detail != "installed, cask not installed" || got.Managed {
		t.Fatalf("report %+v", got)
	}
}

func TestStateAStockShimWhoseProbeFailsIsAbsent(t *testing.T) {
	got := reportOf(t, javaTool, &fake.State{}, java("The operation couldn't be completed. Unable to locate a Java Runtime.", 1))

	if got.State != catalog.StateAbsent {
		t.Fatalf("report %+v", got)
	}
}

func TestStateAShimThatAnswersIsDegraded(t *testing.T) {
	got := reportOf(t, javaTool, &fake.State{}, java(`openjdk version "21.0.5" 2024-10-15`, 0))

	if got.State != catalog.StateDegraded || got.Version != "21.0.5" || got.Detail != "on PATH, not managed by cask" {
		t.Fatalf("report %+v", got)
	}
}
