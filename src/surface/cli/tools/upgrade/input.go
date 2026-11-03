package toolsupgrade

import (
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Input struct {
	Names []string
	Via   []string
}

func bind(in *Input, args []string) error {
	in.Names = args
	if len(in.Names) > 0 && len(in.Via) > 0 {
		return kit.Refuse("name tools or give --via, not both", "station tools upgrade --via "+in.Via[0])
	}
	for _, v := range in.Via {
		if !slices.Contains(managerNames(), v) {
			return kit.Refuse("unknown manager \""+v+"\"", "--via takes "+strings.Join(managerNames(), ", "))
		}
	}
	return nil
}

func managerNames() []string {
	out := make([]string, 0, len(catalog.Managers))
	for _, m := range catalog.Managers {
		out = append(out, string(m.Manager))
	}
	return out
}

func managers(via []string) []catalog.Manager {
	out := make([]catalog.Manager, 0, len(via))
	for _, v := range via {
		out = append(out, catalog.Manager(v))
	}
	return out
}
