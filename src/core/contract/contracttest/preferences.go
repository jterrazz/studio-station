package contracttest

import (
	"reflect"
	"testing"

	"github.com/jterrazz/studio-station/src/core/contract"
)

type example struct {
	Range    string `json:"range,omitempty"`
	MaxPerVM int    `json:"max_per_vm,omitempty"`
}

func Preferences(t *testing.T, open func(t *testing.T, groups []string) contract.Preferences) {
	groups := []string{"here", "services", "lend_list"}

	t.Run("a group never written reads as nothing, the value handed left as it was", func(t *testing.T) {
		got := example{Range: "kept"}
		if err := open(t, groups).Read("services", &got); err != nil {
			t.Fatalf("Read: %v", err)
		}
		if got.Range != "kept" {
			t.Errorf("Read changed the value to %+v", got)
		}
	})

	t.Run("a group written reads back as it was written", func(t *testing.T) {
		p := open(t, groups)
		want := example{Range: "41000-41999", MaxPerVM: 5}
		if err := p.Write("services", want); err != nil {
			t.Fatalf("Write: %v", err)
		}
		var got example
		if err := p.Read("services", &got); err != nil || got != want {
			t.Errorf("Read = %+v, %v; want %+v", got, err, want)
		}
	})

	t.Run("writing one group keeps every other as it was", func(t *testing.T) {
		p := open(t, groups)
		if err := p.Write("here", "mac-mini"); err != nil {
			t.Fatalf("Write here: %v", err)
		}
		if err := p.Write("lend_list", []string{"session:a"}); err != nil {
			t.Fatalf("Write lend_list: %v", err)
		}
		if err := p.Write("lend_list", []string{"session:a", "session:b"}); err != nil {
			t.Fatalf("Write lend_list again: %v", err)
		}
		var here string
		var list []string
		if err := p.Read("here", &here); err != nil || here != "mac-mini" {
			t.Errorf("here = %q, %v; want mac-mini", here, err)
		}
		if err := p.Read("lend_list", &list); err != nil || !reflect.DeepEqual(list, []string{"session:a", "session:b"}) {
			t.Errorf("lend_list = %v, %v; want the second list", list, err)
		}
	})

	t.Run("an empty value removes its group", func(t *testing.T) {
		p := open(t, groups)
		if err := p.Write("here", "mac-mini"); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if err := p.Write("here", ""); err != nil {
			t.Fatalf("Write empty: %v", err)
		}
		here := "untouched"
		if err := p.Read("here", &here); err != nil || here != "untouched" {
			t.Errorf("here = %q, %v; want the group gone", here, err)
		}
	})
}
