package prompt

import (
	"strings"

	"charm.land/huh/v2"
)

func (t *Terminal) AskName(suggestion string) (string, error) {
	name := suggestion
	field := huh.NewInput().Title("Name this machine").Description("lowercase letters, digits and -, as in macbook or mac-mini").Value(&name)
	if err := t.run(huh.NewGroup(field)); err != nil {
		return "", err
	}
	return strings.TrimSpace(name), nil
}
