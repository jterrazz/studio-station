package app_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/apps/adapter/app"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/apps/contract/contracttest"
	"github.com/jterrazz/studio-station/src/catalog"
)

func TestInstaller(t *testing.T) {
	contracttest.Installer(t, catalog.App("download it from its website"), func(*testing.T, contracttest.Seed) contract.Installer {
		return app.New()
	})
}
