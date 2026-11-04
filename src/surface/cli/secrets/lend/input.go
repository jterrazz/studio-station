package secretslend

import (
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Input struct {
	To    string
	For   string
	Names []string
	Term  time.Duration
}

func bind(in *Input, args []string) error {
	if in.To == "" {
		return kit.Refuse("lending needs a target: --to <vm>", "station secrets lend --to <vm>")
	}
	in.Names = args
	in.Term = accounts.DefaultShareTerm
	if in.For == "" {
		return nil
	}
	term, err := time.ParseDuration(in.For)
	if err != nil || term < accounts.ShortestShare || term > accounts.LongestShare {
		return kit.Refuse("a lend lasts between 1m and 7d", "station secrets lend --to "+in.To+" --for 12h")
	}
	in.Term = term
	return nil
}
