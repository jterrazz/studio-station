package fake_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/background/adapter/fake"
	"github.com/jterrazz/studio-station/src/background/contract/contracttest"
	"github.com/jterrazz/studio-station/src/world"
)

func TestJobs(t *testing.T) {
	contracttest.Jobs(t, func(*testing.T) contracttest.Subject {
		return contracttest.Subject{Jobs: fake.New(&world.World{}, "mac-mini"), UID: 501, Dir: "/Users/owner/Library/LaunchAgents"}
	})
}
