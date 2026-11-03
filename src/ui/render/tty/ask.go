package tty

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/huh/v2"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

type Terminal struct {
	in  io.Reader
	out io.Writer
}

func NewTerminal(in io.Reader, out io.Writer) *Terminal { return &Terminal{in: in, out: out} }

func FormStyles(isDark bool) *huh.Styles {
	st := token.New(isDark)
	s := huh.ThemeBase(isDark)
	s.Focused.Title = st.Title
	s.Focused.Description = st.Muted
	s.Focused.ErrorMessage = st.Bad
	s.Focused.ErrorIndicator = st.Bad
	s.Focused.SelectSelector = st.Accent
	s.Focused.Option = st.Text
	s.Focused.SelectedOption = st.Accent
	s.Focused.FocusedButton = st.Button
	s.Focused.BlurredButton = st.Idle
	s.Blurred.Title = st.Muted
	s.Blurred.Description = st.Subtle
	s.Blurred.FocusedButton = st.Button
	s.Blurred.BlurredButton = st.Idle
	return s
}

// invariant: a Danger question defaults to No and Esc declines it, so a
// destructive step only ever runs on a deliberate yes; any other question
// treats Esc and ctrl-c as an interrupt.
func (t *Terminal) Confirm(c page.Confirm) (bool, error) {
	answer := false
	field := ConfirmField(c).Value(&answer)
	form := huh.NewForm(huh.NewGroup(field)).
		WithTheme(huh.ThemeFunc(FormStyles)).
		WithInput(t.in).
		WithOutput(t.out).
		WithShowHelp(false)
	if c.Danger {
		keys := huh.NewDefaultKeyMap()
		keys.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
		form = form.WithKeyMap(keys)
	}
	err := form.Run()
	switch {
	case errors.Is(err, huh.ErrUserAborted) && c.Danger:
		return false, nil
	case errors.Is(err, huh.ErrUserAborted):
		return false, context.Canceled
	}
	return answer, err
}

func ConfirmField(c page.Confirm) *huh.Confirm {
	yes, no := c.Yes, c.No
	if yes == "" {
		yes = "Yes"
	} else {
		yes = strings.ToUpper(yes[:1]) + yes[1:]
	}
	if no == "" {
		no = "No"
	}
	f := huh.NewConfirm().Title(c.Question).Affirmative(yes).Negative(no)
	if len(c.Lines) > 0 {
		lines := make([]string, 0, len(c.Lines))
		for _, l := range c.Lines {
			lines = append(lines, atom.Show(l))
		}
		f = f.Description(strings.Join(lines, "\n"))
	}
	return f
}

// invariant: a secret field is masked as it is typed and its value is returned
// to the caller alone: never echoed, logged or kept by the form.
func (t *Terminal) Form(f page.Form) (map[string]string, error) {
	fields, read := Fields(f)
	group := huh.NewGroup(fields...)
	if f.Title != "" {
		group = group.Title(f.Title)
	}
	err := huh.NewForm(group).
		WithTheme(huh.ThemeFunc(FormStyles)).
		WithInput(t.in).
		WithOutput(t.out).
		WithShowHelp(false).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return nil, context.Canceled
	}
	if err != nil {
		return nil, err
	}
	return read(), nil
}

func Fields(f page.Form) (fields []huh.Field, read func() map[string]string) {
	fields = make([]huh.Field, 0, len(f.Fields))
	getters := make(map[string]func() string, len(f.Fields))
	for _, fd := range f.Fields {
		field, get := Field(fd)
		fields = append(fields, field)
		getters[fd.Key] = get
	}
	return fields, func() map[string]string {
		values := make(map[string]string, len(getters))
		for k, get := range getters {
			values[k] = get()
		}
		return values
	}
}

func Field(f page.Field) (huh.Field, func() string) {
	value := f.Default
	switch f.Kind {
	case page.FieldSelect:
		if value == "" && len(f.Options) > 0 {
			value = f.Options[0].Value
		}
		opts := make([]huh.Option[string], 0, len(f.Options))
		for _, o := range f.Options {
			label := o.Label
			if label == "" {
				label = o.Value
			}
			if o.Note != "" {
				label += atom.Spaces(token.Gap) + o.Note
			}
			opts = append(opts, huh.NewOption(label, o.Value))
		}
		s := huh.NewSelect[string]().Title(f.Label).Options(opts...).Value(&value)
		if f.Note != "" {
			s = s.Description(f.Note)
		}
		return s, func() string { return value }
	case page.FieldConfirm:
		answer := value == strconv.FormatBool(true)
		c := huh.NewConfirm().Title(f.Label).Value(&answer)
		if f.Note != "" {
			c = c.Description(f.Note)
		}
		return c, func() string { return strconv.FormatBool(answer) }
	case page.FieldSecret:
		return input(f, &value).EchoMode(huh.EchoModePassword), func() string { return value }
	case page.FieldText:
	}
	return input(f, &value), func() string { return value }
}

func input(f page.Field, value *string) *huh.Input {
	in := huh.NewInput().Title(f.Label).Value(value)
	if f.Note != "" {
		in = in.Description(f.Note)
	}
	if f.Required {
		in = in.Validate(func(v string) error {
			if v == "" {
				return errors.New(f.Label + " is required")
			}
			return nil
		})
	}
	return in
}
