package kit

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/domain/remote"
	"github.com/jterrazz/studio-station/src/fail"
)

type On struct {
	Machine string
	Here    string
	Args    []string
	Remote  *remote.Service
	Refresh func(ctx context.Context) error
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
	if o == nil || o.Machine == "" || o.Machine == o.Here {
		return "", false
	}
	return o.Machine, true
}

func (o *On) Delegate(ctx context.Context, machine string, interactive bool) error {
	code, err := o.Remote.Run(ctx, machine, interactive, remote.Words(o.Args))
	if err != nil {
		return err
	}
	if code != 0 {
		return fail.Silent(fail.Code(code))
	}
	return nil
}

func Delegates(env Env, c *cobra.Command) (bool, error) {
	machine, ok := env.On.Target()
	if !ok {
		return false, nil
	}
	if env.Screen {
		return true, env.On.Delegate(c.Context(), machine, true)
	}
	return true, Menu(env, c)
}

func refusesOn(path string) bool {
	words := strings.Fields(path)
	if len(words) == 0 {
		return false
	}
	switch words[0] {
	case "shell", "completion", "help", "daemon", "guest":
		return true
	}
	return len(words) > 1 && words[0] == "machines" && (words[1] == "supervise" || words[1] == "pipe")
}

func RefuseOn(env Env) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, _ []string) error {
		if env.On == nil || env.On.Machine == "" {
			return nil
		}
		if p := path(c); refusesOn(p) {
			return fail.Usage("--on does not apply to \""+p+"\"", "run it on "+env.On.Machine+" itself: ssh "+env.On.Machine)
		}
		return nil
	}
}
