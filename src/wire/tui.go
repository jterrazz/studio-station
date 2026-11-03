package wire

import (
	"context"
	"path"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/domain/secrets"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/tui"
)

func tuiOpener(env kit.Env, m machine) func(context.Context) error {
	return func(ctx context.Context) error {
		return tui.Run(ctx, tui.Deps{
			Here:     m.where,
			Status:   statusService(env, m),
			Tools:    toolsService(env, m),
			Config:   configService(m),
			Storage:  m.storage(env),
			Machines: m.machines(true),
			Images:   m.images(),
			Secrets:  m.secrets(),
			Services: m.services(env),
			Tailnet:  m.tailnet(),
			Accounts: m.accounts(),
			Power: tuiPower{Service: m.power(), m: m, describe: func(err error) (string, string) {
				f := kit.Translate(err, "")
				return f.Message, f.Hint
			}},
			Remote: func(ctx context.Context, machine string, words []string) error {
				code, err := env.On.Remote.Run(ctx, machine, true, words)
				if err == nil && code != 0 {
					return &remoteExit{machine: machine, code: code}
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

type remoteExit struct {
	machine string
	code    int
}

func (e *remoteExit) Error() string {
	return "station on " + e.machine + " exited " + strconv.Itoa(e.code)
}

func (m machine) secrets() *secrets.Service {
	return secrets.New(m.host, m.store, vmRecords{fs: m.host.FS, dir: path.Join(m.paths.State, "vms")}, secrets.Options{Here: m.kind})
}
