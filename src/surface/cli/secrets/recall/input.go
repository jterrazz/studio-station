package secretsrecall

import "github.com/jterrazz/studio-station/src/surface/cli/kit"

type Input struct {
	From  string
	Names []string
}

func bind(in *Input, args []string) error {
	if in.From == "" {
		return kit.Refuse("recalling needs a source: --from <vm>", "station secrets recall --from <vm>")
	}
	in.Names = args
	return nil
}
