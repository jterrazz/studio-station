package tools_test

import (
	"context"
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/apps/adapter/fake"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/probe"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

type stillClock struct{}

func (stillClock) Now() time.Time { return time.Unix(0, 0) }

func (stillClock) Sleep(ctx context.Context, _ time.Duration) error {
	<-ctx.Done()
	return ctx.Err()
}

func reportOf(t *testing.T, tool catalog.Tool, manager *fake.State, run *systemtest.Runner, files *systemtest.FS) tools.Report {
	h := systemtest.Host(run, files, systemtest.Env{HomeDir: "/home/u", OS: "darwin"}, systemtest.NewNet(), &systemtest.Clock{})
	m := tool.Installs[0].Install.Method()
	installers := contract.Installers{m: fake.New(m, manager)}
	reports, err := tools.New(h, []catalog.Tool{tool}, nil, tools.Options{Here: catalog.Host, Installers: installers, Pool: probe.NewPool(context.Background(), stillClock{}, 16, 1)}).Inventory(context.Background())
	if err != nil || len(reports) != 1 {
		t.Fatalf("inventory: %v %v", reports, err)
	}
	return reports[0]
}

func TestStateAnAppPresentWithoutItsManagerIsDegraded(t *testing.T) {
	files := systemtest.NewFS()
	_ = files.MkdirAll("/Applications/Keynote.app", 0o755)
	tool := catalog.Tool{Name: "keynote", Installs: catalog.HostOnly(catalog.Cask("keynote")), Bundle: "Keynote"}

	got := reportOf(t, tool, &fake.State{Absent: true}, systemtest.NewRunner(t), files)

	if got.State != catalog.StateDegraded || got.Detail != "installed, cask not installed" || got.Managed {
		t.Fatalf("report %+v", got)
	}
}

func TestStateAStockShimWhoseProbeFailsIsAbsent(t *testing.T) {
	run := systemtest.NewRunner(t)
	run.Paths["java"] = "/usr/bin/java"
	run.On("java", "-version").Reply(system.Output{ExitCode: 1, Stderr: "The operation couldn't be completed. Unable to locate a Java Runtime."})
	tool := catalog.Tool{Name: "java", Installs: catalog.Anywhere(catalog.Cask("temurin")), Provides: []string{"java"}, Version: catalog.Probe{Args: []string{"java", "-version"}, Pattern: `version "([^"]+)"`}}

	got := reportOf(t, tool, &fake.State{}, run, systemtest.NewFS())

	if got.State != catalog.StateAbsent {
		t.Fatalf("report %+v", got)
	}
}

func TestStateAShimThatAnswersIsDegraded(t *testing.T) {
	run := systemtest.NewRunner(t)
	run.Paths["java"] = "/usr/bin/java"
	run.On("java", "-version").Reply(system.Output{Stderr: `openjdk version "21.0.5" 2024-10-15`})
	tool := catalog.Tool{Name: "java", Installs: catalog.Anywhere(catalog.Cask("temurin")), Provides: []string{"java"}, Version: catalog.Probe{Args: []string{"java", "-version"}, Pattern: `version "([^"]+)"`}}

	got := reportOf(t, tool, &fake.State{}, run, systemtest.NewFS())

	if got.State != catalog.StateDegraded || got.Version != "21.0.5" || got.Detail != "on PATH, not managed by cask" {
		t.Fatalf("report %+v", got)
	}
}
