package prompt

import (
	"errors"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"

	"github.com/jterrazz/studio-station/src/theme/forms"
)

type ResetLine struct{ Level, Path, Size string }

type ResetAsker interface {
	ConfirmReset(tools []string, lines []ResetLine) (bool, error)
}

func resetDescription(lines []ResetLine) string {
	out := make([]string, 0, len(lines)+1)
	for _, l := range lines {
		if l.Level != "cache" && l.Level != "data" {
			continue
		}
		out = append(out, l.Level+"  "+l.Path+"  "+l.Size)
	}
	out = append(out, "installed files and kept files stay")
	return strings.Join(out, "\n")
}

// invariant: Keep is the default and Esc declines, so a reset only ever runs on a
// deliberate Reset.
func resetForm(in io.Reader, out io.Writer, tools []string, lines []ResetLine, answer *bool) *huh.Form {
	*answer = false
	keys := huh.NewDefaultKeyMap()
	keys.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
	field := huh.NewConfirm().
		Title("Reset " + strings.Join(tools, ", ") + "?").
		Description(resetDescription(lines)).
		Affirmative("Reset").
		Negative("Keep").
		Value(answer)
	return huh.NewForm(huh.NewGroup(field)).
		WithTheme(huh.ThemeFunc(forms.Styles)).
		WithKeyMap(keys).
		WithInput(in).
		WithOutput(out).
		WithShowHelp(false)
}

func (t *Terminal) ConfirmReset(tools []string, lines []ResetLine) (bool, error) {
	var answer bool
	if err := resetForm(t.in, t.out, tools, lines, &answer).Run(); err != nil && !errors.Is(err, huh.ErrUserAborted) {
		return false, err
	}
	return answer, nil
}
