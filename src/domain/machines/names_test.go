package machines

import "testing"

func TestAllocateNameSkipsEveryHeldName(t *testing.T) {
	for _, c := range []struct {
		name    string
		records []string
		nodes   map[string]Node
		want    string
	}{
		{name: "the smallest free integer fills a gap", records: []string{"linux-1", "linux-3"}, want: "linux-2"},
		{name: "an online station node holds its name", nodes: map[string]Node{"linux-1": {Online: true, StationVM: true}}, want: "linux-2"},
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
