package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/jterrazz/studio-station/src/domain/config"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/domain/services"
	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/domain/tailnet"
	"github.com/jterrazz/studio-station/src/domain/tools"
	"github.com/jterrazz/studio-station/src/page"
	configscreen "github.com/jterrazz/studio-station/src/tui/screens/config"
	fleetscreen "github.com/jterrazz/studio-station/src/tui/screens/fleet"
	homescreen "github.com/jterrazz/studio-station/src/tui/screens/home"
	servicesscreen "github.com/jterrazz/studio-station/src/tui/screens/services"
	storagescreen "github.com/jterrazz/studio-station/src/tui/screens/storage"
	"github.com/jterrazz/studio-station/src/tui/screens/tab"
	toolsscreen "github.com/jterrazz/studio-station/src/tui/screens/tools"
	ui "github.com/jterrazz/studio-station/src/ui/render/tui"
)

type Deps struct {
	Here     page.Context
	Status   *status.Service
	Tools    *tools.Service
	Config   *config.Service
	Storage  *storage.Service
	Machines *machines.Service
	Images   *images.Service
	Secrets  *secrets.Service
	Services *services.Service
	Tailnet  *tailnet.Service
	Remote   func(ctx context.Context, machine string, words []string) error
	Station  func(ctx context.Context, words []string) error
	Describe func(error) (message, hint string)
	Prefer   func(prefix string)
}

type Tab struct {
	ui.Tab
	// value: the probe prefix the shown tab's reads run first under.
	Probes string
}

func Tabs() []Tab {
	return []Tab{
		{Tab: ui.Tab{Label: "Home"}, Probes: "status/"},
		{Tab: ui.Tab{Label: "Tools"}, Probes: "tools/"},
		{Tab: ui.Tab{Label: "Config"}},
		{Tab: ui.Tab{Label: "Storage"}, Probes: "storage/"},
		{Tab: ui.Tab{Label: "Fleet", Fleet: true}},
		{Tab: ui.Tab{Label: "Services", Fleet: true}},
	}
}

func Run(ctx context.Context, d Deps) error {
	describe := tab.Describe(d.Describe)
	app := newApp(d.Here, Tabs(), []tab.Screen{
		homescreen.New(ctx, d.Status, d.Station, d.Here, describe),
		toolsscreen.New(ctx, d.Tools, d.Here, describe),
		configscreen.New(ctx, d.Config, d.Here, describe),
		storagescreen.New(ctx, d.Storage, d.Here, describe),
		fleetscreen.New(ctx, fleetscreen.Deps{
			Machines: d.Machines, Images: d.Images, Secrets: d.Secrets, Services: d.Services, Tailnet: d.Tailnet,
			Remote: d.Remote, Describe: describe,
		}, d.Here),
		servicesscreen.New(ctx, d.Services, d.Here, describe),
	}, d.Prefer)
	_, err := tea.NewProgram(app, tea.WithContext(ctx)).Run()
	return err
}
