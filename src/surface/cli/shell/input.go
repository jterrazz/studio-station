package shellcmd

import "github.com/jterrazz/studio-station/src/surface/cli/kit"

type Input struct{ Shell string }

func bind(in *Input, args []string) error {
	if len(args) != 1 || args[0] != "zsh" {
		return kit.Refuse("station shell takes the one shell it serves: zsh", "station shell zsh")
	}
	in.Shell = args[0]
	return nil
}
