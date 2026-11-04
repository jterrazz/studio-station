package wire

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"time"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/engine"
	enginecmd "github.com/jterrazz/studio-station/src/surface/cli/engine"
	enginestart "github.com/jterrazz/studio-station/src/surface/cli/engine/start"
	enginestop "github.com/jterrazz/studio-station/src/surface/cli/engine/stop"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
	"github.com/jterrazz/studio-station/src/system"
)

func (m machine) control() *engine.Control {
	return engine.NewControl(m.host, m.engineAPI(), m.machines(false), engineHypervisor{m.hypervisor()}, m.engineJob(), engine.ControlOptions{
		Power: m.powerPort, Here: m.kind, HereName: m.where.Machine, State: m.paths.State,
		Version: Version(), Marks: m.stopMarks(),
	})
}

func (m machine) stopMarks() engine.StopMarks {
	if m.world != nil {
		return m.world.stopMarks()
	}
	return fileMarks{fs: m.host.FS, file: path.Join(m.paths.State, "engine-stopped.json")}
}

// invariant: the last stop the owner asked, kept beside the engine's socket
// while it is stopped; a file that cannot be read is no stop kept.
type fileMarks struct {
	fs   system.FS
	file string
}

func (f fileMarks) Read() (engine.StopMark, bool) {
	var m engine.StopMark
	data, err := f.fs.ReadFile(f.file)
	if err != nil || json.Unmarshal(data, &m) != nil {
		return engine.StopMark{}, false
	}
	return m, true
}

func (f fileMarks) Write(m engine.StopMark) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := f.fs.MkdirAll(path.Dir(f.file), 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	return f.fs.WriteFile(f.file, append(data, '\n'), 0o600)
}

func engineCommands(env kit.Env, m machine) []*cobra.Command {
	return []*cobra.Command{
		enginestart.Command(enginestart.Deps{Here: m.where}, env),
		enginestop.Command(enginestop.Deps{Here: m.where}, env),
	}
}

func engineNoun(env kit.Env, m machine, plumbing ...*cobra.Command) *cobra.Command {
	return enginecmd.Command(enginecmd.Deps{Here: m.where, Zone: time.Local}, env, append(engineCommands(env, m), plumbing...)...)
}

// value: station stop and station start, the words v2 gave them.
func engineAliases(env kit.Env, m machine) []*cobra.Command {
	return []*cobra.Command{
		kit.Alias("stop", "engine stop", enginestop.Command(enginestop.Deps{Here: m.where}, env), env),
		kit.Alias("start", "engine start", enginestart.Command(enginestart.Deps{Here: m.where}, env), env),
	}
}
