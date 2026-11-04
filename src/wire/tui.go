package wire

import (
	"context"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/accounts/adapter/studio"
	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	"github.com/jterrazz/studio-station/src/surface/tui"
	accountsscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/accounts"
	configscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/config"
	powerscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/power"
	servicesscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/services"
	storagescreen "github.com/jterrazz/studio-station/src/surface/tui/screens/storage"
	toolsscreen "github.com/jterrazz/studio-station/src/surface/tui/screens/tools"
	"github.com/jterrazz/studio-station/src/system"
)

func tuiOpener(env kit.Env, m machine) func(context.Context) error {
	return func(ctx context.Context) error {
		storage, accounts := m.storage(env), m.accounts()
		return tui.Run(ctx, tui.Deps{
			Here:     m.where,
			Calls:    env.Calls,
			Target:   env.Target(),
			Tools:    toolsscreen.Deps{Calls: env.Calls, Target: env.Target(), Forget: toolsService(env, m).Forget},
			Config:   configscreen.Deps{Calls: env.Calls, Target: env.Target(), Defaults: configService(m).Defaults},
			Storage:  storagescreen.Deps{Calls: env.Calls, Target: env.Target(), Forget: storage.Forget, Plan: storage.Plan},
			Services: servicesscreen.Deps{Calls: env.Calls, Target: env.Target()},
			Accounts: accountsscreen.Deps{Calls: env.Calls, Target: env.Target(), Forget: accounts.Forget, LendList: accounts.LendList},
			Power: powerscreen.Deps{Calls: env.Calls, Target: env.Target(), Power: tuiPower{Service: m.power(), m: m, describe: func(err error) (string, string) {
				f := kit.Translate(err, "")
				return f.Message, f.Hint
			}}},
			Remote: func(ctx context.Context, machine string) error {
				words := []string{"--on", machine}
				r, err := m.host.Run.Attach(ctx, system.Cmd{Name: m.host.Env.Executable(), Args: words}, system.ToStdout)
				if err == nil && r.ExitCode != 0 {
					return &stationExit{words: words, code: r.ExitCode}
				}
				return err
			},
			Station: func(ctx context.Context, words []string) error {
				r, err := m.host.Run.Attach(ctx, system.Cmd{Name: m.host.Env.Executable(), Args: words}, system.ToStdout)
				if err == nil && r.ExitCode != 0 {
					return &stationExit{words: words, code: r.ExitCode}
				}
				return err
			},
			Prefer: m.pool.Prefer,
			Stop:   m.pool.Close,
			Describe: func(err error) (message, hint string) {
				f := kit.Translate(err, "")
				return f.Message, f.Hint
			},
		})
	}
}

type stationExit struct {
	words []string
	code  int
}

func (e *stationExit) Error() string {
	return "station " + strings.Join(e.words, " ") + " exited " + strconv.Itoa(e.code)
}

func (m machine) secrets() *secrets.Service { return m.secretsVia(m.hostDaemon()) }

func (m machine) secretsVia(guest secrets.Guest) *secrets.Service {
	return secrets.New(m.host, guest, m.secretStores(), m.secretSource(studio.Options{Budget: secrets.ExportBudget}), secrets.Options{Here: m.kind})
}
