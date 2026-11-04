package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/settings"
)

type Deps struct {
	Config Config
	// value: the values an item asks, from the owner at a terminal; nil
	// where nobody is there to answer.
	Ask       Ask
	Shortcuts Shortcuts
	// value: the script the ~/.zshrc block evaluates.
	Shell Shell
	Kind  machine.Kind
}

type Shell interface {
	Zsh(ctx context.Context, completion string) (string, error)
}

type Config interface {
	Watch(ctx context.Context) (<-chan []settings.Report, error)
	Find(names []string) ([]catalog.ConfigItem, error)
	CheckInstall(names []string, interactive bool) error
	Install(ctx context.Context, name string, values map[string]string, interactive bool) (settings.Change, error)
	CheckUninstall(names []string) error
	Uninstall(ctx context.Context, name string) (settings.Change, error)
}

type Ask interface {
	Values(item catalog.ConfigItem) (map[string]string, error)
}

func All(d Deps) []call.Handler {
	return []call.Handler{call.BindWatch(List, d), call.Bind(Install, d), call.Bind(Uninstall, d), call.Bind(Git, d), call.Bind(Containers, d), call.Bind(Zsh, d)}
}
