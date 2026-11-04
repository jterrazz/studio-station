package wire

import (
	"path/filepath"

	"github.com/jterrazz/studio-station/src/accounts/adapter/studio"
	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/shell"
)

func (m machine) secretSource(o studio.Options) accountscontract.SecretSource {
	if m.world != nil {
		return m.world.secretSource()
	}
	return studio.New(m.host.Run, o)
}

func (m machine) shell() *shell.Service {
	return shell.New(m.host, catalog.DotfilesFS, m.secretSource(studio.Options{FileBackend: true, Budget: shell.TokenBudget}), shell.Options{Here: m.kind, Services: filepath.Join(m.paths.Config, "services.env")})
}
