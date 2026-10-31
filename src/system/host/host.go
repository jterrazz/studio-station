package host

import (
	"io"
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
	return system.Host{
		Run:   &runner{env: env, dirs: dirs, stdin: stdin, stdout: stdout, stderr: stderr},
		FS:    disk{},
		Env:   identity{vars: vars},
		Net:   network{},
		Clock: clock{},
	}
}
