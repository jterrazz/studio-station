package brew

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

const (
	command = "brew"
	bundle  = command + " bundle"
)

var manager = contract.Manager{Name: command, App: "homebrew", Needs: "homebrew"}

type Brew struct {
	run local.Runner
	env func() []string
}

func New(run local.Runner, env func() []string) *Brew {
	// invariant: env is asked at each bundle, so a manager installed by an
	// earlier step of the same run is on the bundle's PATH.
	return &Brew{run: run, env: env}
}

func (b *Brew) Formulae() contract.Installer {
	return &kind{brew: b, method: catalog.MethodBrew, list: local.Command(command, "list", "--formula", "--versions")}
}

func (b *Brew) Casks() contract.Installer {
	return &kind{brew: b, method: catalog.MethodCask, list: local.Command(command, "list", "--cask", "--versions")}
}

func (b *Brew) Name() string { return bundle }

func (b *Brew) Bundle(ctx context.Context, lines []string, update bool, _ call.Progress) error {
	// hazard: brew's own prompts (a cask's sudo) read /dev/tty, not stdin, so
	// feeding the Brewfile on stdin (--file=-) does not steal them.
	mode := "--no-upgrade"
	if update {
		mode = "--upgrade"
	}
	c := local.Command(command, "bundle", "install", mode, "--file=-")
	c.Stdin = strings.Join(lines, "\n") + "\n"
	if b.env != nil {
		c.Env = b.env()
	}
	ran, err := b.run.Attach(ctx, c, local.ToStderr)
	switch {
	case err != nil:
		return errors.New(bundle + ": " + err.Error())
	case ran.ExitCode != 0:
		failure := bundle + " exited " + strconv.Itoa(ran.ExitCode)
		if ran.LastStderr != "" {
			failure += ": " + ran.LastStderr
		}
		return errors.New(failure)
	}
	return nil
}

type kind struct {
	brew   *Brew
	method catalog.Method
	list   local.Cmd
}

func (k *kind) Method() catalog.Method    { return k.method }
func (k *kind) Manager() contract.Manager { return manager }

func (k *kind) Can() contract.Abilities {
	return contract.Abilities{Install: true, Update: true, Remove: true, UpdateAll: true}
}

func (k *kind) Ready(context.Context) error {
	if _, ok := k.brew.run.LookPath(command); !ok {
		return contract.ErrMissing
	}
	return nil
}

func (k *kind) Inventory(ctx context.Context) (map[string]string, error) {
	if err := k.Ready(ctx); err != nil {
		return nil, err
	}
	out, err := k.brew.run.Output(ctx, k.list)
	if err != nil {
		return nil, err
	}
	if out.ExitCode != 0 {
		return nil, errors.New("brew list exited " + strconv.Itoa(out.ExitCode))
	}
	return parseList(out.Stdout), nil
}

func (k *kind) Outdated(context.Context, []string) (map[string]apps.Currency, error) {
	return nil, nil
}

func (k *kind) Install(ctx context.Context, installs []catalog.Install, p call.Progress) error {
	return k.brew.Bundle(ctx, k.lines(installs), false, p)
}

func (k *kind) Update(ctx context.Context, installs []catalog.Install, p call.Progress) error {
	return k.brew.Bundle(ctx, k.lines(installs), true, p)
}

func (k *kind) Remove(ctx context.Context, installs []catalog.Install, _ call.Progress) error {
	for _, i := range installs {
		var c local.Cmd
		switch v := i.(type) {
		case catalog.BrewInstall:
			c = local.Command(command, "uninstall", v.Formula)
		case catalog.CaskInstall:
			c = local.Command(command, "uninstall", "--cask", v.Token)
		default:
			return errors.New(i.Key() + " is not installed by brew")
		}
		if err := local.Attach(ctx, k.brew.run, c); err != nil {
			return err
		}
	}
	return nil
}

func (k *kind) lines(installs []catalog.Install) []string {
	out := make([]string, 0, len(installs))
	for _, i := range installs {
		out = append(out, k.Line(i))
	}
	return out
}

func (k *kind) Line(i catalog.Install) string {
	trusted := func(line string, t bool) string {
		if t {
			return line + ", trusted: true"
		}
		return line
	}
	switch v := i.(type) {
	case catalog.BrewInstall:
		return trusted(`brew "`+v.Formula+`"`, v.Trusted)
	case catalog.CaskInstall:
		return trusted(`cask "`+v.Token+`"`, v.Trusted)
	}
	return ""
}

func parseList(stdout string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		out[fields[0]] = fields[len(fields)-1]
		if len(fields) == 1 {
			out[fields[0]] = ""
		}
	}
	return out
}

func (b *Brew) Manager() string { return command }

func (b *Brew) Behind(ctx context.Context) (int, error) {
	// invariant: how many installed formulae and casks have a newer version;
	// contract.ErrMissing where brew is not installed.
	if _, ok := b.run.LookPath(command); !ok {
		return 0, contract.ErrMissing
	}
	out, err := b.run.Output(ctx, local.Command(command, "outdated", "--quiet"))
	switch {
	case err != nil:
		return 0, err
	case out.ExitCode != 0:
		return 0, errors.New("brew outdated exited " + strconv.Itoa(out.ExitCode))
	}
	return len(strings.Fields(out.Stdout)), nil
}
