package settings

import (
	"context"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

func DetectKind(ctx context.Context, h system.Host, override string) catalog.MachineKind {
	switch k := catalog.MachineKind(override); k {
	case catalog.Host, catalog.Linux, catalog.MacOS:
		return k
	}
	if h.Env.GOOS() == "linux" {
		return catalog.Linux
	}
	// invariant: anything but a clear "1" is a host, a missing or failing sysctl
	// included, so a sandbox with no system tool still answers a kind.
	out, err := h.Run.Output(ctx, system.Cmd{Name: "sysctl", Args: []string{"-n", "kern.hv_vmm_present"}, Timeout: 2 * time.Second})
	if err == nil && out.ExitCode == 0 && strings.TrimSpace(out.Stdout) == "1" {
		return catalog.MacOS
	}
	return catalog.Host
}
