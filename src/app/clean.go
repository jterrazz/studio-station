package app

import (
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/catalog"
	cleancmd "github.com/jterrazz/studio-station/src/cli/clean"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/domain/clean"
)

func cleanCommand(env kit.Env, m machine) *cobra.Command {
	service := clean.New(m.host, catalog.Cleanables, clean.Options{Here: m.kind})
	return cleancmd.Command(cleancmd.Deps{Clean: service, Context: m.context}, env)
}
