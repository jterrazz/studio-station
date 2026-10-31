package present_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/present"
)

func TestBytesHumanisesAtTheUnitBoundaries(t *testing.T) {
	for _, c := range []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{999, "999 B"},
		{1000, "1.0 KB"},
		{1499, "1.5 KB"},
		{999_949, "999.9 KB"},
		{999_950, "1.0 MB"},
		{18_400_000_000, "18.4 GB"},
		{2_000_000_000_000, "2.0 TB"},
		{9_000_000_000_000_000_000, "9.0 EB"},
	} {
		if got := present.Bytes(c.in); got != c.want {
			t.Errorf("Bytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
