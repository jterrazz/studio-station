package mas

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/apps/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/platform/local"
)

const command = "mas"

type Mas struct {
	run    local.Runner
	bundle contract.Bundler
}

func New(run local.Runner, bundle contract.Bundler) *Mas { return &Mas{run: run, bundle: bundle} }

func (m *Mas) Method() catalog.Method { return catalog.MethodMas }

func (m *Mas) Manager() contract.Manager {
	return contract.Manager{Name: command, App: command, Needs: "homebrew"}
}

func (m *Mas) Can() contract.Abilities {
	// invariant: the App Store updates its own apps, so a bare update leaves
	// them; one is updated by name.
	return contract.Abilities{Install: true, Update: true, Remove: true}
}

func (m *Mas) Ready(context.Context) error {
	if _, ok := m.run.LookPath(command); !ok {
		return contract.ErrMissing
	}
	return nil
}

func (m *Mas) Inventory(ctx context.Context) (map[string]string, error) {
	if err := m.Ready(ctx); err != nil {
		return nil, err
	}
	out, err := m.run.Output(ctx, local.Command(command, "list"))
	if err != nil {
		return nil, err
	}
	if out.ExitCode != 0 {
		return nil, errors.New("mas list exited " + strconv.Itoa(out.ExitCode))
	}
	return parseList(out.Stdout), nil
}

func (m *Mas) Outdated(context.Context, []string) (map[string]apps.Currency, error) {
	return nil, nil
}

func (m *Mas) Install(ctx context.Context, installs []catalog.Install, p call.Progress) error {
	return m.bundle.Bundle(ctx, m.lines(installs), false, p)
}

func (m *Mas) Update(ctx context.Context, installs []catalog.Install, p call.Progress) error {
	return m.bundle.Bundle(ctx, m.lines(installs), true, p)
}

func (m *Mas) Remove(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	for _, i := range installs {
		v, ok := i.(catalog.MasInstall)
		if !ok {
			return errors.New(i.Key() + " is not an App Store app")
		}
		if err := local.Attach(ctx, m.run, local.Command(command, "uninstall", strconv.Itoa(v.ID))); err != nil {
			return err
		}
	}
	return nil
}

func (m *Mas) lines(installs []catalog.Install) []string {
	out := make([]string, 0, len(installs))
	for _, i := range installs {
		out = append(out, m.Line(i))
	}
	return out
}

func (m *Mas) Line(i catalog.Install) string {
	v, ok := i.(catalog.MasInstall)
	if !ok {
		return ""
	}
	return `mas "` + v.Name + `", id: ` + strconv.Itoa(v.ID)
}

func parseList(stdout string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		version := ""
		if open := strings.LastIndex(line, "("); open >= 0 {
			version = strings.TrimSuffix(strings.TrimSpace(line[open+1:]), ")")
		}
		out[fields[0]] = version
	}
	return out
}
