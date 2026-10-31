package prompt

import "charm.land/huh/v2"

type HostChoice struct {
	Name, Detail string
	Here         bool
}

type CreateAnswers struct{ Lifetime, Host string }

type CreateAsker interface {
	AskCreate(lifetime string, hosts []HostChoice) (CreateAnswers, error)
	AskBuild(image string) (bool, error)
}

type question struct {
	key, title string
	options    []huh.Option[string]
	initial    string
}

// invariant: the lifetime is asked, never defaulted, and offers exactly the two
// lifetimes; the host is a question only when there is a choice, two reachable
// hosts or more, and it defaults to here.
func createQuestions(lifetime string, hosts []HostChoice) []question {
	var qs []question
	if lifetime == "" {
		qs = append(qs, question{key: "lifetime", title: "Will it last?", options: []huh.Option[string]{
			huh.NewOption("temporary: a poweroff inside it deletes it", "temporary"),
			huh.NewOption("lasting: it stays until deleted", "lasting"),
		}})
	}
	if len(hosts) >= 2 {
		q := question{key: "host", title: "On which host?"}
		for _, h := range hosts {
			label := h.Name
			if h.Detail != "" {
				label += "  " + h.Detail
			}
			q.options = append(q.options, huh.NewOption(label, h.Name))
			if h.Here {
				q.initial = h.Name
			}
		}
		qs = append(qs, q)
	}
	return qs
}

func (t *Terminal) AskCreate(lifetime string, hosts []HostChoice) (CreateAnswers, error) {
	answers := CreateAnswers{Lifetime: lifetime}
	qs := createQuestions(lifetime, hosts)
	if len(qs) == 0 {
		return answers, nil
	}
	values := make([]string, len(qs))
	fields := make([]huh.Field, 0, len(qs))
	for i, q := range qs {
		values[i] = q.initial
		fields = append(fields, huh.NewSelect[string]().Title(q.title).Options(q.options...).Value(&values[i]))
	}
	if err := t.run(huh.NewGroup(fields...)); err != nil {
		return answers, err
	}
	for i, q := range qs {
		switch q.key {
		case "lifetime":
			answers.Lifetime = values[i]
		case "host":
			answers.Host = values[i]
		}
	}
	return answers, nil
}

func (t *Terminal) AskBuild(image string) (bool, error) {
	return t.Confirm("Image " + image + " is not built here. Build " + image + " now? (~3 min)")
}
