package wire

import (
	"path/filepath"

	"github.com/jterrazz/studio-station/src/accounts/adapter/studio"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/shell"
)

func (m machine) shell() *shell.Service {
	return shell.New(m.host, catalog.DotfilesFS, studio.New(m.host.Run, studio.Options{FileBackend: true, Budget: shell.TokenBudget}), shell.Options{Here: m.kind, Services: filepath.Join(m.paths.Config, "services.env")})
}
