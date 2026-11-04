package fake_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/apps/adapter/fake"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/apps/contract/contracttest"
	"github.com/jterrazz/studio-station/src/catalog"
)

func TestInstaller(t *testing.T) {
	contracttest.Installer(t, catalog.Skill("anthropics/skills", "frontend-design"), func(_ *testing.T, s contracttest.Seed) contract.Installer {
		return fake.New(catalog.MethodSkill, &fake.State{Absent: s.Absent, Installed: s.Installed, Fails: s.Fails})
	})
}
