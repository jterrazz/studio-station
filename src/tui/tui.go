package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/tui/kit"
	configscreen "github.com/jterrazz/studio-station/src/tui/screens/config"
	fleetscreen "github.com/jterrazz/studio-station/src/tui/screens/fleet"
	statusscreen "github.com/jterrazz/studio-station/src/tui/screens/status"
	storagescreen "github.com/jterrazz/studio-station/src/tui/screens/storage"
	toolsscreen "github.com/jterrazz/studio-station/src/tui/screens/tools"
)

type Deps struct {
	Context  string
	Status   *status.Service
	Tools    *tools.Service
	Config   *config.Service
	Storage  *storage.Service
	Machines *machines.Service
	Images   *images.Service
	Secrets  *secrets.Service
	Tailnet  *tailnet.Service
	Remote   func(ctx context.Context, machine string, words []string) error
	Describe func(error) (message, hint string)
}

func Tabs() []kit.Tab {
	return []kit.Tab{{Label: "Status"}, {Label: "Tools"}, {Label: "Config"}, {Label: "Storage"}, {Label: "Fleet", Fleet: true}}
}

func Run(ctx context.Context, d Deps) error {
	t := theme.New(true)
	app := newApp(d.Context, Tabs(), []kit.Screen{
		statusscreen.New(ctx, d.Status, d.Describe, t),
		toolsscreen.New(ctx, d.Tools, d.Describe, t),
		configscreen.New(ctx, d.Config, d.Describe, t),
		storagescreen.New(ctx, d.Storage, d.Describe, t),
		fleetscreen.New(ctx, fleetscreen.Deps{
			Machines: d.Machines, Images: d.Images, Secrets: d.Secrets, Tailnet: d.Tailnet,
			Remote: d.Remote, Describe: d.Describe,
		}, t),
	})
	_, err := tea.NewProgram(app, tea.WithContext(ctx)).Run()
	return err
}
