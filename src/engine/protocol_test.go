package engine

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAnUnknownOpIsWordedForTheClient(t *testing.T) {
	d := &Daemon{}
	resp := d.answer(t.Context(), []byte(`{"v":1,"id":"2","op":"nope"}`))
	if resp.OK || resp.Error == nil || resp.Error.Code != "unknown_op" || resp.Error.Hint == "" || resp.ID != "2" {
		t.Fatalf("response %+v", resp)
	}
	if other := d.answer(t.Context(), []byte(`{"v":2,"id":"3","op":"ping"}`)); other.Error == nil || other.Error.Code != "unknown_op" {
		t.Fatalf("another protocol version answered %+v", other)
	}
	if broken := d.answer(t.Context(), []byte(`{`)); broken.OK {
		t.Fatalf("a line that is not JSON answered ok")
	}
}

func TestTheRunWrapperWritesTheExitCodeOfTheRun(t *testing.T) {
	c := RunCmd([]string{"/bin/tart", "run", "vm"}, "/s/runs/vm.exit")
	data, _ := json.Marshal(c.Args)
	if c.Name != "sh" || c.Args[0] != "-c" || !strings.Contains(c.Args[1], `"$@"; echo "$?" >"/s/runs/vm.exit"`) || c.Args[2] != "sh" || c.Args[3] != "/bin/tart" {
		t.Fatalf("wrapper %s %s", c.Name, data)
	}
}
