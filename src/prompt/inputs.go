package prompt

import (
	"errors"
	"strconv"

	"charm.land/huh/v2"

	"github.com/jterrazz/studio-station/src/catalog"
)

type Asker interface {
	Inputs(title string, inputs []catalog.Input, defaults map[string]string) (map[string]string, error)
}

func (t *Terminal) Inputs(title string, inputs []catalog.Input, defaults map[string]string) (map[string]string, error) {
	// invariant: a password input is masked as it is typed and its value is returned
	// to the caller alone: it is never echoed, logged or kept by the form.
	fields := make([]huh.Field, 0, len(inputs))
	read := make(map[string]func() string, len(inputs))
	for _, in := range inputs {
		f, get := field(in, defaults[in.Name])
		fields = append(fields, f)
		read[in.Name] = get
	}
	if err := t.run(huh.NewGroup(fields...).Title(title)); err != nil {
		return nil, err
	}
	values := make(map[string]string, len(inputs))
	for name, get := range read {
		values[name] = get()
	}
	return values, nil
}

func field(in catalog.Input, initial string) (input huh.Field, read func() string) {
	switch in.Kind {
	case catalog.InputSelect:
		value := initial
		if value == "" && len(in.Options) > 0 {
			value = in.Options[0]
		}
		f := huh.NewSelect[string]().Title(in.Label).Description(in.Help).Options(huh.NewOptions(in.Options...)...).Value(&value)
		return f, func() string { return value }
	case catalog.InputConfirm:
		answer := initial == strconv.FormatBool(true)
		f := huh.NewConfirm().Title(in.Label).Description(in.Help).Value(&answer)
		return f, func() string { return strconv.FormatBool(answer) }
	case catalog.InputPassword:
		value := initial
		return text(in, &value).EchoMode(huh.EchoModePassword), func() string { return value }
	case catalog.InputText:
		value := initial
		return text(in, &value), func() string { return value }
	}
	value := initial
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
