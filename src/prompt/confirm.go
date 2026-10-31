package prompt

import (
	"context"
	"errors"
	"io"

	"charm.land/huh/v2"

	"github.com/jterrazz/studio-station/src/theme/forms"
)

type Prompter interface {
	Confirm(question string) (bool, error)
}

type Terminal struct {
	in  io.Reader
	out io.Writer
}

func New(in io.Reader, out io.Writer) *Terminal {
	return &Terminal{in: in, out: out}
}

func (t *Terminal) Confirm(question string) (bool, error) {
	answer := false
	field := huh.NewConfirm().Title(question).Affirmative("Yes").Negative("No").Value(&answer)
	return answer, t.run(huh.NewGroup(field))
}

func (t *Terminal) run(groups ...*huh.Group) error {
	err := huh.NewForm(groups...).
		WithTheme(huh.ThemeFunc(forms.Styles)).
		WithInput(t.in).
		WithOutput(t.out).
		WithShowHelp(false).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return context.Canceled
	}
	return err
}
