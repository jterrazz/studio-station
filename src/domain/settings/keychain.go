package settings

import (
	"context"
	"errors"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

var ErrNoKeychain = errors.New("kept secrets live only in a host's keychain")

const (
	service = "station"
	// outside: `security` exits 44 when the item is not in the keychain.
	notFound = 44
)

type Keychain struct {
	run system.Runner
	off bool
}

func NewKeychain(r system.Runner) Keychain { return Keychain{run: r} }

func (k Keychain) On(kind catalog.MachineKind) Keychain {
	k.off = kind != catalog.Host
	return k
}

func (k Keychain) Read(ctx context.Context, account string) (value string, found bool, err error) {
	if k.off {
		return "", false, ErrNoKeychain
	}
	out, err := k.run.Output(ctx, system.Cmd{Name: "security", Args: []string{"find-generic-password", "-s", service, "-a", account, "-w"}})
	switch {
	case err != nil:
		return "", false, &KeychainError{Account: account, Cause: err}
	case out.ExitCode == notFound:
		return "", false, nil
	case out.ExitCode != 0:
		return "", false, &KeychainError{Account: account, Cause: errors.New(strings.TrimSpace(out.Stderr))}
	}
	return strings.TrimRight(out.Stdout, "\n"), true, nil
}

func (k Keychain) Remove(ctx context.Context, account string) (removed bool, err error) {
	if k.off {
		return false, ErrNoKeychain
	}
	out, err := k.run.Output(ctx, system.Cmd{Name: "security", Args: []string{"delete-generic-password", "-s", service, "-a", account}})
	if err == nil && out.ExitCode != 0 && out.ExitCode != notFound {
		err = errors.New(strings.TrimSpace(out.Stderr))
	}
	if err != nil {
		return false, &KeychainError{Account: account, Cause: err}
	}
	return out.ExitCode == 0, nil
}

type KeychainError struct {
	Account string
	Cause   error
}

func (e *KeychainError) Error() string {
	return "keychain account " + e.Account + ": " + e.Cause.Error()
}
func (e *KeychainError) Unwrap() error { return e.Cause }
