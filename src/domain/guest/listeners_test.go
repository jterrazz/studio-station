package guest

import (
	"testing"

	"github.com/jterrazz/studio-station/src/ports"
)

func TestEachPortIsOneListenerAndDockerNamesItsOwn(t *testing.T) {
	sockets := Sockets([]ports.Listener{
		{Port: 8081, Bind: "::", Process: "node", PID: 412}, {Port: 8081, Bind: "0.0.0.0", Process: "node", PID: 412},
		{Port: 8081, Bind: "::1", Process: "node", PID: 412}, {Port: 5432, Bind: "127.0.0.1", Process: "postgres", PID: 90},
	})
	if len(sockets) != 2 || sockets[0] != (Listener{Port: 5432, IP: "127.0.0.1", Process: "postgres", PID: 90, Source: SourceSocket}) ||
		sockets[1].IP != "0.0.0.0" || sockets[1].Process != "node" {
		t.Fatalf("sockets %+v, want one per port, IPv4 first", sockets)
	}
	docker := ParseDockerPS([]byte(`{"Names":"web-1","Ports":"0.0.0.0:3000->80/tcp, [::]:3000->80/tcp, 5432/tcp"}` + "\n"))
	if len(docker) != 1 || docker[0] != (Listener{Port: 3000, IP: "0.0.0.0", Process: "web-1", Source: SourceDocker}) {
		t.Fatalf("docker %+v", docker)
	}
	merged := Merge([]Listener{{Port: 3000, IP: "0.0.0.0", Process: "docker-proxy", Source: SourceSocket}}, docker)
	if merged[0].Process != "web-1" {
		t.Fatalf("the docker entry does not win its port: %+v", merged)
	}
}

func TestTheDiffOfTwoSnapshotsIsListenAndUnlisten(t *testing.T) {
	before := []Listener{{Port: 3000, IP: "0.0.0.0"}, {Port: 8081, IP: "0.0.0.0"}}
	after := []Listener{{Port: 3000, IP: "0.0.0.0"}, {Port: 5173, IP: "127.0.0.1"}}
	got := Diff(before, after)
	if len(got) != 2 || got[0].Listen == nil || got[0].Listen.Port != 5173 || got[1].Unlisten != 8081 {
		t.Fatalf("diff %+v", got)
	}
	if kept := Filter([]Listener{{Port: 22}, {Port: 631}, {Port: 3000}}, 1024, []int{22}); len(kept) != 1 || kept[0].Port != 3000 {
		t.Fatalf("filter %+v", kept)
	}
}
