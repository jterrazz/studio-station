package local

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/jterrazz/studio-station/src/system"
)

const defaultSystemDirs = "/usr/sbin:/sbin:/usr/libexec:/usr/libexec/ApplicationFirewall"

func New(env []string, stdin io.Reader, stdout, stderr io.Writer) system.Host {
	vars := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	systemDirs, set := vars["STATION_SYSTEM_DIRS"]
	if !set {
		systemDirs = defaultSystemDirs
	}
	var dirs []string
	for _, d := range strings.Split(vars["PATH"]+":"+systemDirs, ":") {
		if d != "" {
			dirs = append(dirs, d)
		}
	}
	power := newPower()
	if dir := vars["STATION_POWER_STUB"]; filepath.IsAbs(dir) {
		power = &stubPower{dir: dir, calls: filepath.Join(identity{vars: vars}.Home(), ".calls")}
	}
	return system.Host{
		Run:     &runner{env: env, dirs: dirs, stdin: stdin, stdout: stdout, stderr: stderr},
		FS:      disk{},
		Env:     identity{vars: vars},
		Net:     network{},
		Sockets: sockets{},
		Clock:   clock{},
		Power:   power,
	}
}
