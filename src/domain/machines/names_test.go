package machines

import "testing"

func TestAllocateNameSkipsEveryHeldName(t *testing.T) {
	for _, c := range []struct {
		name    string
		records []string
		nodes   map[string]Node
		want    string
	}{
		{name: "an empty fleet starts at 1", want: "linux-1"},
		{name: "a record on any host holds its name", records: []string{"linux-1"}, want: "linux-2"},
		{name: "the smallest free integer fills a gap", records: []string{"linux-1", "linux-3"}, want: "linux-2"},
		{name: "an online node of any kind holds its name", records: []string{"linux-1"}, nodes: map[string]Node{"linux-2": {Online: true}}, want: "linux-3"},
		{name: "an online station node holds its name", nodes: map[string]Node{"linux-1": {Online: true, StationVM: true}}, want: "linux-2"},
		{name: "an offline station node with no record is a leftover", nodes: map[string]Node{"linux-1": {StationVM: true}}, want: "linux-1"},
		{name: "an offline node station did not tag holds its name", nodes: map[string]Node{"linux-1": {}}, want: "linux-2"},
		{name: "a record holds its name even when its node is a leftover", records: []string{"linux-1"}, nodes: map[string]Node{"linux-1": {StationVM: true}}, want: "linux-2"},
	} {
		t.Run(c.name, func(t *testing.T) {
			records := map[string]bool{}
			for _, r := range c.records {
				records[r] = true
			}
			if got := allocateName("linux", records, c.nodes); got != c.want {
				t.Errorf("allocateName = %q, want %q", got, c.want)
			}
		})
	}
}
