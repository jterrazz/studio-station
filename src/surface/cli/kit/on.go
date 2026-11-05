package kit

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/core/machine"
)

type On struct {
	Machine string
	// value: the machine this terminal uses, when no --on names one.
	Used    string
	Here    string
	Refresh func(ctx context.Context) error
	// value: whether a machine of that name exists; an error when it cannot
	// be told (a Mac that does not answer).
	Known func(ctx context.Context, name string) (bool, error)
}

// invariant: the remote's answer is the outcome; a refresh that fails leaves the
// blocks the next bare `station machines` rewrites, so it is no failure.
func (o *On) refresh(ctx context.Context, path string) {
	if o.Refresh == nil {
		return
	}
	switch path {
	case "machines create", "machines delete", "machines add", "machines forget":
		_ = o.Refresh(ctx)
	}
}

func (o *On) Target() (string, bool) {
	if o == nil {
		return "", false
	}
	name := o.Machine
	if name == "" {
		name = o.Used
	}
	if name == "" || name == o.Here {
		return "", false
	}
	return name, true
}

func (e Env) Target() machine.Ref {
	// invariant: --on is a target of the Router: the leaf runs here, its call
	// runs on that machine and its answer is rendered by this binary.
	if name, ok := e.On.Target(); ok && name != e.Self.Name {
		return machine.Ref{Name: name}
	}
	return e.Self
}

func (e Env) Far() bool { return e.Target() != e.Self }

func Delegates(env Env, c *cobra.Command) (bool, error) {
	if !env.Far() || env.Screen {
		return false, nil
	}
	return true, Menu(env, c)
}

func refusesOn(path string) bool {
	words := strings.Fields(path)
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "shell", "completion", "help", "daemon", "guest", "use":
		return true
	}
	return len(words) > 1 && (words[0] == "machines" && (words[1] == "supervise" || words[1] == "pipe") ||
		words[0] == "engine" && (words[1] == "run" || words[1] == "attach"))
}

func RefuseOn(env Env) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		if env.On == nil {
			return nil
		}
		if env.On.Machine == "" && c.DisableFlagParsing {
			env.On.Machine = rawOn(args)
		}
		if env.On.Machine == "" {
			return nil
		}
		p := path(c)
		if refusesOn(p) {
			return usage("--on does not apply to \""+p+"\"", "run it on "+env.On.Machine+" itself: ssh "+env.On.Machine)
		}
		if env.Far() && c.Annotations[owned] == "" && c.HasParent() {
			return usage("--on does not apply to \""+p+"\"", "run it on "+env.On.Machine+" itself: ssh "+env.On.Machine)
		}
		return nil
	}
}

// invariant: a command kit built answers --on with its calls; one it did not
// (fang's man) has none to send, so --on is refused for it.
const owned = "station.kit"

func rawOn(args []string) string {
	for i, a := range args {
		if a == "--" {
			return ""
		}
		if v, ok := strings.CutPrefix(a, "--on="); ok {
			return v
		}
		if a == "--on" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
