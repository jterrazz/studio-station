package rundocker

import (
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/cli/kit"
)

type Input struct{ Name string }

func shortcutNames() []string {
	out := make([]string, 0, len(catalog.DockerShortcuts))
	for _, s := range catalog.DockerShortcuts {
		out = append(out, s.Name)
	}
	return out
}

func bind(in *Input, args []string) error {
	names := strings.Join(shortcutNames(), ", ")
	switch {
	case len(args) == 0:
		return kit.Refuse("name a docker shortcut: "+names, "station run docker reset --yes")
	case len(args) > 1:
		return kit.Refuse("one docker shortcut at a time", "station run docker "+args[0]+" --yes")
	case !slices.Contains(shortcutNames(), args[0]):
		return kit.Refuse("unknown docker shortcut \""+args[0]+"\"", "run docker takes "+names)
	}
	in.Name = args[0]
	return nil
}

func verb(in *Input) string {
	for _, s := range catalog.DockerShortcuts {
		if s.Name == in.Name {
			return s.Verb
		}
	}
	return in.Name
}
