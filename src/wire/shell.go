package wire

import (
	"path/filepath"

	"github.com/jterrazz/studio-station/src/accounts/adapter/studio"
	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings/adapter/zshblock"
)

func (m machine) secretSource(o studio.Options) accountscontract.SecretSource {
	if m.world != nil {
		return m.world.secretSource()
	}
	return studio.New(m.sys.Run, o)
}

func (m machine) shell() *zshblock.Block {
	return zshblock.NewBlock(m.sys.FS, catalog.DotfilesFS, m.secretSource(studio.Options{FileBackend: true, Budget: zshblock.TokenBudget}), zshblock.BlockOptions{Here: m.kind, Services: filepath.Join(m.paths.Config, "services.env"), Functions: m.zshFunctions()})
}

// value: the script of every setting that defines shell functions, where
// its Applier writes it.
func (m machine) zshFunctions() []string {
	var out []string
	for _, it := range catalog.Settings {
		if _, ok := it.Effect.(catalog.ZshFunctions); ok {
			out = append(out, zshblock.Script(m.zshDir(), it.Name))
		}
	}
	return out
}

func (m machine) zshDir() string { return filepath.Join(m.paths.Config, "zsh") }
