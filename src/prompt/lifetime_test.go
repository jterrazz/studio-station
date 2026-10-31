package prompt

import (
	"slices"
	"testing"
)

func TestCreateQuestionsAskTheLifetimeThenAHostOnlyWhenThereIsAChoice(t *testing.T) {
	here := HostChoice{Name: "macbook", Here: true}
	other := HostChoice{Name: "mac-mini"}
	for _, c := range []struct {
		name     string
		lifetime string
		hosts    []HostChoice
		want     []string
	}{
		{name: "no lifetime and one host: the lifetime only", hosts: []HostChoice{here}, want: []string{"lifetime"}},
		{name: "no lifetime and two hosts: both", hosts: []HostChoice{here, other}, want: []string{"lifetime", "host"}},
		{name: "a lifetime given and two hosts: the host only", lifetime: "lasting", hosts: []HostChoice{here, other}, want: []string{"host"}},
		{name: "a lifetime given and one host: nothing", lifetime: "temporary", hosts: []HostChoice{here}},
	} {
		t.Run(c.name, func(t *testing.T) {
			keys := make([]string, 0, 2)
			for _, q := range createQuestions(c.lifetime, c.hosts) {
				keys = append(keys, q.key)
			}
			if !slices.Equal(keys, c.want) {
				t.Errorf("questions = %v, want %v", keys, c.want)
			}
		})
	}
}

func TestTheLifetimeQuestionOffersExactlyTheTwoLifetimes(t *testing.T) {
	qs := createQuestions("", nil)
	values := make([]string, 0, len(qs[0].options))
	for _, o := range qs[0].options {
		values = append(values, o.Value)
	}
	if !slices.Equal(values, []string{"temporary", "lasting"}) || qs[0].initial != "" {
		t.Errorf("lifetime options = %v (initial %q), want temporary and lasting with no default", values, qs[0].initial)
	}
}

func TestTheHostQuestionDefaultsToHere(t *testing.T) {
	qs := createQuestions("lasting", []HostChoice{{Name: "mac-mini"}, {Name: "macbook", Here: true}})
	if qs[0].initial != "macbook" {
		t.Errorf("host default = %q, want macbook", qs[0].initial)
	}
}
