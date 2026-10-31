package tools_test

import (
	"context"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

func reportOf(t *testing.T, tool catalog.Tool, run *systemtest.Runner, files *systemtest.FS) tools.Report {
	h := systemtest.Host(run, files, systemtest.Env{HomeDir: "/home/u", OS: "darwin"}, systemtest.NewNet(), &systemtest.Clock{})
	reports, err := tools.New(h, []catalog.Tool{tool}, nil, tools.Options{Here: catalog.Host}).Inventory(context.Background())
	if err != nil || len(reports) != 1 {
		t.Fatalf("inventory: %v %v", reports, err)
	}
	return reports[0]
}

func TestStateAnAppStoreAppPresentWithoutMasIsDegraded(t *testing.T) {
	files := systemtest.NewFS()
	_ = files.MkdirAll("/Applications/Keynote.app", 0o755)
	tool := catalog.Tool{Name: "keynote", Installs: catalog.HostOnly(catalog.Mas("Keynote", 361285480)), Bundle: "Keynote"}

	got := reportOf(t, tool, systemtest.NewRunner(t), files)

	if got.State != catalog.StateDegraded || got.Detail != "installed, mas not installed" || got.Managed {
		t.Fatalf("report %+v", got)
	}
}

func TestStateAStockShimWhoseProbeFailsIsAbsent(t *testing.T) {
	run := systemtest.NewRunner(t)
	run.Paths["java"] = "/usr/bin/java"
	run.Paths["mise"] = "/opt/homebrew/bin/mise"
	run.On("mise", "ls", "--global", "--json").Reply(system.Output{Stdout: "{}"})
	run.On("java", "-version").Reply(system.Output{ExitCode: 1, Stderr: "The operation couldn't be completed. Unable to locate a Java Runtime."})
	tool := catalog.Tool{Name: "java", Installs: catalog.Anywhere(catalog.Mise("java", "temurin-21")), Provides: []string{"java"}, Version: catalog.Probe{Args: []string{"java", "-version"}, Pattern: `version "([^"]+)"`}}

	got := reportOf(t, tool, run, systemtest.NewFS())

	if got.State != catalog.StateAbsent {
		t.Fatalf("report %+v", got)
	}
}

func TestStateAShimThatAnswersIsDegraded(t *testing.T) {
	run := systemtest.NewRunner(t)
	run.Paths["java"] = "/usr/bin/java"
	run.Paths["mise"] = "/opt/homebrew/bin/mise"
	run.On("mise", "ls", "--global", "--json").Reply(system.Output{Stdout: "{}"})
	run.On("java", "-version").Reply(system.Output{Stderr: `openjdk version "21.0.5" 2024-10-15`})
	tool := catalog.Tool{Name: "java", Installs: catalog.Anywhere(catalog.Mise("java", "temurin-21")), Provides: []string{"java"}, Version: catalog.Probe{Args: []string{"java", "-version"}, Pattern: `version "([^"]+)"`}}

	got := reportOf(t, tool, run, systemtest.NewFS())

	if got.State != catalog.StateDegraded || got.Version != "21.0.5" || got.Detail != "on PATH, not managed by mise" {
		t.Fatalf("report %+v", got)
	}
}
