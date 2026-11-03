package mise

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/platform/local"
)

const command = "mise"

type Mise struct{ run local.Runner }

func New(run local.Runner) *Mise { return &Mise{run: run} }

func (m *Mise) Method() catalog.Method { return catalog.MethodMise }

func (m *Mise) Manager() contract.Manager {
	return contract.Manager{Name: command, App: command, Needs: command}
}

func (m *Mise) Can() contract.Abilities {
	return contract.Abilities{Install: true, Update: true, Remove: true, UpdateAll: true}
}

func (m *Mise) Ready(context.Context) error {
	if _, ok := m.run.LookPath(command); !ok {
		return contract.ErrMissing
	}
	return nil
}

func (m *Mise) Inventory(ctx context.Context) (map[string]string, error) {
	if err := m.Ready(ctx); err != nil {
		return nil, err
	}
	out, err := m.run.Output(ctx, local.Command(command, "ls", "--global", "--json"))
	if err != nil {
		return nil, err
	}
	if out.ExitCode != 0 {
		return nil, errors.New("mise ls exited " + strconv.Itoa(out.ExitCode))
	}
	var doc map[string][]struct {
		Version   string `json:"version"`
		Installed bool   `json:"installed"`
	}
	if err := json.Unmarshal([]byte(out.Stdout), &doc); err != nil {
		return nil, err
	}
	list := map[string]string{}
	for name, versions := range doc {
		for _, v := range versions {
			if v.Installed {
				list[name] = v.Version
				break
			}
		}
	}
	return list, nil
}

func (m *Mise) Outdated(context.Context, []string) (map[string]apps.Currency, error) {
	return nil, nil
}

func (m *Mise) Install(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	return m.each(ctx, installs, func(v catalog.MiseInstall) local.Cmd {
		return local.Command(command, "use", "--global", v.Tool+"@"+v.Version)
	})
}

func (m *Mise) Update(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	return m.each(ctx, installs, func(v catalog.MiseInstall) local.Cmd { return local.Command(command, "upgrade", v.Tool) })
}

func (m *Mise) Remove(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	return m.each(ctx, installs, func(v catalog.MiseInstall) local.Cmd { return local.Command(command, "unuse", "--global", v.Tool) })
}

func (m *Mise) each(ctx context.Context, installs []catalog.Install, argv func(catalog.MiseInstall) local.Cmd) error {
	for _, i := range installs {
		v, ok := i.(catalog.MiseInstall)
		if !ok {
			return errors.New(i.Key() + " is not a mise runtime")
		}
		if err := local.Attach(ctx, m.run, argv(v)); err != nil {
			return err
		}
	}
	return nil
}
