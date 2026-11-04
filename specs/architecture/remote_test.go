package architecture_test

import (
	"fmt"
	"go/ast"
	"regexp"
	"testing"
)

// invariant: the only command lines that start station elsewhere, each the
// plumbing its owner serves or hops by: another Mac's engine API, a VM's
// guest API and its ports reporter, and the owner's own ssh to a VM of
// another Mac. No other code builds `station <words>` to run elsewhere.
func stationLaunchers() map[string]string {
	return map[string]string{
		"remote/adapter/sshstdio":  "station engine attach",
		"remote/adapter/unix":      "station guest serve --stdio",
		"engine":                   "station guest",
		"machines/adapter/openssh": "station machines pipe, the owner's ProxyCommand",
	}
}

var launchesStation = regexp.MustCompile(`exec station\b|/station [a-z]`)

// invariant: rule no-remote-reexec, ARCHITECTURE 13.3 rule 12: --on is a
// Router target, the fleet a fan-out of calls; nothing relays station's
// text from another machine.
func ruleNoRemoteReexec(w *world) []finding {
	var out []finding
	for _, f := range w.files {
		if f.test || stationLaunchers()[f.pkg] != "" {
			continue
		}
		literals(f, func(lit *ast.BasicLit, value string) {
			if launchesStation.MatchString(value) {
				out = append(out, finding{"no-remote-reexec", f.at(lit.Pos()), fmt.Sprintf("%q starts station elsewhere: a call reaches another machine through the Router", f.pkg)})
			}
		})
	}
	return out
}

func TestNoRemoteReexec(t *testing.T) { judge(t, "NoRemoteReexec") }
