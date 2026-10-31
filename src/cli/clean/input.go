package cleancmd

import "github.com/jterrazz/studio-station/src/cli/kit"

type Input struct {
	Names []string
	All   bool
}

func (in *Input) acts() bool { return len(in.Names) > 0 || in.All }

func bind(in *Input, args []string) error {
	in.Names = args
	if in.All && len(args) > 0 {
		return kit.Refuse("name cleanables or give --all, not both", "station clean --all --yes")
	}
	return nil
}
