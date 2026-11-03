package script

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/platform/local"
)

// hazard: the installer must keep the terminal as its stdin: Homebrew's install.sh
// turns NONINTERACTIVE and `sudo -n` when stdin is not a TTY, which is why its
// documented form is `bash -c "$(curl ...)"` rather than `curl | bash`.
const curlInstaller = `script="$(curl -fsSL "$0")" && exec bash -c "$script" bash "$@"`

type Script struct{ run local.Runner }

func New(run local.Runner) *Script { return &Script{run: run} }

func (s *Script) Method() catalog.Method    { return catalog.MethodScript }
func (s *Script) Manager() contract.Manager { return contract.Manager{} }

func (s *Script) Can() contract.Abilities {
	return contract.Abilities{Install: true, Update: true, UpdateAll: true}
}

func (s *Script) Ready(context.Context) error { return nil }

func (s *Script) Inventory(context.Context) (map[string]string, error) {
	return nil, contract.ErrUnlisted
}

func (s *Script) Outdated(context.Context, []string) (map[string]apps.Currency, error) {
	return nil, nil
}

func (s *Script) Install(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	for _, i := range installs {
		v, ok := i.(catalog.ScriptInstall)
		if !ok {
			return errors.New(i.Key() + " is not an install script")
		}
		c := local.Command("bash", append([]string{"-c", curlInstaller, v.URL}, v.Args...)...)
		c.Env = v.Env
		if err := local.Attach(ctx, s.run, c); err != nil {
			return err
		}
	}
	return nil
}

func (s *Script) Update(ctx context.Context, installs []catalog.Install, p call.Progress) error {
	return s.Install(ctx, installs, p)
}

func (s *Script) Remove(context.Context, []catalog.Install, call.Progress) error {
	return contract.ErrCannot
}
