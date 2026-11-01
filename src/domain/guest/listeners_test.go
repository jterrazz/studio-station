package guest

import (
	"slices"
	"testing"
)

const procTCP = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:1F91 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 101 1 0000000000000000 100 0 0 10 0
   1: 0100007F:1538 00000000:0000 0A 00000000:00000000 00:00000000 00000000   999        0 102 1 0000000000000000 100 0 0 10 0
   2: 0100007F:2328 0100007F:A000 01 00000000:00000000 00:00000000 00000000  1000        0 103 1 0000000000000000 100 0 0 10 0
`

const procTCP6 = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:0BB8 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 104 1 0000000000000000 100 0 0 10 0
   1: 0000000000000000FFFF00000100007F:1F91 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 105 1 0000000000000000 100 0 0 10 0
   2: 00000000000000000000000001000000:0050 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 106 1 0000000000000000 100 0 0 10 0
`

func TestProcNetReadsListenersInHostByteOrder(t *testing.T) {
	got := ParseProcNet([]byte(procTCP), []byte(procTCP6))
	want := []Listener{
		{Port: 80, IP: "::1", Source: SourceSocket, inode: "106"},
		{Port: 3000, IP: "::", Source: SourceSocket, inode: "104"},
		{Port: 5432, IP: "127.0.0.1", Source: SourceSocket, inode: "102"},
		{Port: 8081, IP: "0.0.0.0", Source: SourceSocket, inode: "101"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("listeners\n%+v\nwant\n%+v", got, want)
	}
}

func TestLsofAndDockerListenersAreParsed(t *testing.T) {
	lsof := ParseLsof([]byte("p412\ncnode\nn*:8081\nn[::1]:8081\np90\ncpostgres\nn127.0.0.1:5432\n"))
	if len(lsof) != 2 || lsof[0] != (Listener{Port: 5432, IP: "127.0.0.1", Process: "postgres", PID: 90, Source: SourceSocket}) ||
		lsof[1].IP != "0.0.0.0" || lsof[1].Process != "node" {
		t.Fatalf("lsof %+v", lsof)
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
