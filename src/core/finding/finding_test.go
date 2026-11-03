package finding_test

import (
	"slices"
	"testing"

	"github.com/jterrazz/studio-station/src/core/finding"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

func TestNeedsYouListsBrokenFirstThenBySectionThenMacBeforeItsVMs(t *testing.T) {
	this := machine.Ref{Name: "jterrazz-macbook-pro"}
	mini := machine.Ref{Name: "jterrazz-mac-mini"}
	infra := machine.Ref{Name: "infra", On: mini.Name}
	linux1 := machine.Ref{Name: "linux-1", On: this.Name}
	line := func(m machine.Ref, s section.ID, sev finding.Severity, subject string) finding.Finding {
		return finding.Finding{Machine: m, Section: s, Severity: sev, Subject: subject}
	}
	read := []finding.Finding{
		line(this, section.Settings, finding.NeedsALook, "starship"),
		line(mini, section.Ports, finding.Broken, "airplay-5000"),
		line(linux1, section.Apps, finding.NeedsALook, "jq"),
		line(this, section.Apps, finding.NeedsALook, "node"),
		line(infra, section.Overview, finding.Broken, "health"),
		line(this, section.Accounts, finding.NeedsALook, "npm"),
		line(mini, section.Overview, finding.Broken, "engine"),
		line(this, section.Settings, finding.NeedsALook, "firewall"),
	}

	got := make([]string, 0, len(read))
	for _, f := range finding.Rank(read) {
		got = append(got, f.Machine.Name+" "+f.Subject)
	}

	want := []string{
		"jterrazz-mac-mini engine",
		"infra health",
		"jterrazz-mac-mini airplay-5000",
		"jterrazz-macbook-pro node",
		"linux-1 jq",
		"jterrazz-macbook-pro starship",
		"jterrazz-macbook-pro firewall",
		"jterrazz-macbook-pro npm",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Needs you ranks\n%v\nwant\n%v", got, want)
	}
	if read[0].Subject != "starship" {
		t.Error("ranking reordered the read it was given")
	}
}
