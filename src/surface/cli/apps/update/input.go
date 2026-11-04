package appsupdate

import (
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/surface/cli/kit"
)

type Input struct {
	Names       []string
	Method      []string
	AllMachines bool
}

func bind(in *Input, args []string) error {
	in.Names = args
	if len(in.Names) > 0 && len(in.Method) > 0 {
		return kit.Refuse("name apps or give --method, not both", "station apps update --method "+in.Method[0])
	}
	for _, v := range in.Method {
		if !slices.Contains(managerNames(), v) {
			return kit.Refuse("unknown method \""+v+"\"", "--method takes "+strings.Join(managerNames(), ", "))
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

func managers(method []string) []catalog.Manager {
	out := make([]catalog.Manager, 0, len(method))
	for _, v := range method {
		out = append(out, catalog.Manager(v))
	}
	return out
}
