package secretslend

import "github.com/jterrazz/studio-station/src/cli/kit"

type Input struct {
	To    string
	Names []string
}

func bind(in *Input, args []string) error {
	if in.To == "" {
		return kit.Refuse("lending needs a target: --to <vm>", "station secrets lend --to <vm>")
	}
	in.Names = args
	return nil
}
