package wire

import (
	"context"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/adapter/keychain"
	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
)

// value: the vault a machine keeps station's secrets in: a host's keychain;
// a VM keeps none.
func (m machine) vault() contract.Vault {
	if m.kind != catalog.Host {
		return noVault{}
	}
	return keychain.New(m.host.Run)
}

type noVault struct{}

func (noVault) Get(context.Context, string) ([]byte, error) { return nil, accounts.ErrNoVault }
func (noVault) Put(context.Context, string, []byte) error   { return accounts.ErrNoVault }
func (noVault) Forget(context.Context, string) error        { return accounts.ErrNoVault }
func (noVault) Names(context.Context) ([]string, error)     { return nil, accounts.ErrNoVault }
