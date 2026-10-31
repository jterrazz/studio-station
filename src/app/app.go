package app

import (
	"context"
	"io"
	"os"
	"strings"

	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/cobra"

	"github.com/jterrazz/studio-station/src/cli"
	"github.com/jterrazz/studio-station/src/cli/kit"
	"github.com/jterrazz/studio-station/src/fail"
	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/prompt"
)

func Run(args, env []string, stdin, stdout, stderr *os.File) int {
	vars := map[string]string{}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok {
			vars[k] = v
		}
	}
	inTTY, outTTY, errTTY := term.IsTerminal(stdin.Fd()), term.IsTerminal(stdout.Fd()), term.IsTerminal(stderr.Fd())
	dark := true
	if inTTY && outTTY {
		// hazard: the background query writes an OSC request and reads the reply
		// on stdin, so it runs only when both ends are a terminal that answers.
		dark = lipgloss.HasDarkBackground(stdin, stdout)
	}
	writer := output.NewWriter(
		output.Stream{W: stdout, IsTerminal: outTTY, Width: widthOf(stdout, outTTY), Dark: dark},
		output.Stream{W: stderr, IsTerminal: errTTY, Width: widthOf(stderr, errTTY), Dark: dark},
		vars,
	)
	kenv := kit.Env{
		Out:         writer,
		Prompt:      prompt.New(stdin, stderr),
		Interactive: inTTY && errTTY,
		Screen:      inTTY && outTTY,
	}

	var root *cobra.Command
	root = cli.NewRoot(kenv, tuiOpener(kenv, func() *cobra.Command { return root }),
		statusCommand(kenv),
		toolsCommand(kenv),
		configCommand(kenv),
		cleanCommand(kenv),
		machinesCommand(kenv),
		imagesCommand(kenv),
		secretsCommand(kenv),
		tailnetCommand(kenv),
		runCommand(kenv),
		setupCommand(kenv),
		shellCommand(kenv),
	)
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := fang.Execute(context.Background(), root,
		fang.WithVersion(Version()),
		fang.WithCommit(Commit()),
		fang.WithErrorHandler(func(_ io.Writer, _ fang.Styles, err error) {
			writer.Failure(kit.Translate(err, ""))
		}),
		fang.WithColorSchemeFunc(fangScheme),
		fang.WithNotifySignal(os.Interrupt),
	)
	if err == nil {
		return int(fail.CodeOK)
	}
	return int(fail.CodeOf(kit.Translate(err, "")))
}

func widthOf(f *os.File, isTerminal bool) int {
	if !isTerminal {
		return 0
	}
	w, _, err := term.GetSize(f.Fd())
	if err != nil {
		return 0
	}
	return w
}
