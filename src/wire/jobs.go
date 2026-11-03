package wire

import (
	"time"

	"github.com/jterrazz/studio-station/src/background/adapter/launchd"
)

func (m machine) jobs(budget time.Duration) *launchd.Jobs {
	return launchd.New(m.host.Run, m.host.FS, launchd.Options{
		Home: m.host.Env.Home(), Root: m.systemRoot(), UID: m.host.Env.Euid(), Budget: budget,
	})
}
