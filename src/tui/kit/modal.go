package kit

import (
	"errors"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/theme/forms"
)

const modalWidth = 64

type Modal struct {
	form   *huh.Form
	submit func() tea.Cmd
}

func newModal(submit func(f *huh.Form) tea.Cmd, fields ...huh.Field) *Modal {
	form := huh.NewForm(huh.NewGroup(fields...)).
		WithTheme(huh.ThemeFunc(forms.Styles)).
		WithShowHelp(false).
		WithWidth(modalWidth - 4)
	return &Modal{form: form, submit: func() tea.Cmd { return submit(form) }}
}

func Confirm(question string, onYes tea.Cmd) *Modal {
	return newModal(func(f *huh.Form) tea.Cmd {
		if f.GetBool("ok") {
			return onYes
		}
		return nil
	}, huh.NewConfirm().Key("ok").Title(question).Affirmative("Yes").Negative("No"))
}

func ConfirmReset(question, description string, onYes tea.Cmd) *Modal {
	return newModal(func(f *huh.Form) tea.Cmd {
		if f.GetBool("ok") {
			return onYes
		}
		return nil
	}, huh.NewConfirm().Key("ok").Title(question).Description(description).Affirmative("Reset").Negative("Keep"))
}

func TypeToConfirm(question, name string, onYes tea.Cmd) *Modal {
	return newModal(func(f *huh.Form) tea.Cmd {
		if f.GetString("name") == name {
			return onYes
		}
		return nil
	}, huh.NewInput().Key("name").Title(question).Description("type "+name+" to confirm").
		Validate(func(v string) error {
			if v != name {
				return errors.New("type " + name + " exactly")
			}
			return nil
		}))
}

func Form(title string, inputs []catalog.Input, defaults map[string]string, submit func(values map[string]string) tea.Cmd) *Modal {
	// invariant: a password input is masked as it is typed and its value goes to
	// the submit alone: never echoed in the status bar, never kept by the screen.
	fields := make([]huh.Field, 0, len(inputs)+1)
	fields = append(fields, huh.NewNote().Title(title))
	read := make(map[string]func() string, len(inputs))
	for _, in := range inputs {
		f, get := field(in, defaults[in.Name])
		fields = append(fields, f)
		read[in.Name] = get
	}
	return newModal(func(*huh.Form) tea.Cmd {
		values := make(map[string]string, len(read))
		for name, get := range read {
			values[name] = get()
		}
		return submit(values)
	}, fields...)
}

func field(in catalog.Input, initial string) (input huh.Field, read func() string) {
	value := initial
	switch in.Kind {
	case catalog.InputSelect:
		if value == "" && len(in.Options) > 0 {
			value = in.Options[0]
		}
		return huh.NewSelect[string]().Title(in.Label).Description(in.Help).Options(huh.NewOptions(in.Options...)...).Value(&value),
			func() string { return value }
	case catalog.InputConfirm:
		answer := initial == strconv.FormatBool(true)
		return huh.NewConfirm().Title(in.Label).Description(in.Help).Value(&answer),
			func() string { return strconv.FormatBool(answer) }
	case catalog.InputPassword:
		return text(in, &value).EchoMode(huh.EchoModePassword), func() string { return value }
	case catalog.InputText:
	}
	return text(in, &value), func() string { return value }
}

func text(in catalog.Input, value *string) *huh.Input {
	return huh.NewInput().Title(in.Label).Description(in.Help).Value(value).
		Validate(func(v string) error {
			if in.Required && v == "" {
				return errors.New(in.Label + " is required")
			}
			return nil
		})
}

func (m *Modal) init() tea.Cmd { return m.form.Init() }

func (m *Modal) update(msg tea.Msg) (done bool, cmd tea.Cmd) {
	model, cmd := m.form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		m.form = f
	}
	return m.form.State != huh.StateNormal, cmd
}

func (m *Modal) result() tea.Cmd {
	if m.form.State != huh.StateCompleted {
		return nil
	}
	return m.submit()
}

func (m *Modal) view(t theme.Theme, width int) string {
	return Frame(t, "", m.form.View(), min(width-1, modalWidth))
}
