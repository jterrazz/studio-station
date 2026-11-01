package prompt

import (
	"io"
	"strings"
	"testing"
)

var esc = string([]byte{27})

func run(t *testing.T, keys string) bool {
	t.Helper()
	answer := true
	form := resetForm(strings.NewReader(keys), io.Discard, []string{"zed"}, nil, &answer)
	_ = form.Run()
	return answer
}

func TestTheResetFormKeepsUnlessResetIsChosen(t *testing.T) {
	if run(t, "\r") {
		t.Error("enter on the default reset, want Keep")
	}
	if run(t, esc) {
		t.Error("esc reset, want a decline")
	}
	if !run(t, esc+"[D\r") {
		t.Error("left then enter kept, want Reset")
	}
}

func TestTheResetDescriptionListsWhatGoesAndNothingThatStays(t *testing.T) {
	got := resetDescription([]ResetLine{
		{Level: "cache", Path: "~/Library/Logs/Zed", Size: "1.0 MB"},
		{Level: "data", Path: "~/Library/Application Support/Zed/db", Size: "2.0 MB"},
		{Level: "config", Path: "~/.config/zed", Size: "4.0 KB"},
		{Level: "kept", Path: "~/.ssh", Size: "1.0 KB"},
	})
	want := "cache  ~/Library/Logs/Zed  1.0 MB\ndata  ~/Library/Application Support/Zed/db  2.0 MB\ninstalled files and kept files stay"
	if got != want {
		t.Errorf("description =\n%s\nwant\n%s", got, want)
	}
}
