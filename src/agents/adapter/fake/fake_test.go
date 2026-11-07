package fake_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/agents/adapter/fake"
	"github.com/jterrazz/studio-station/src/agents/contract/contracttest"
)

func TestJournal(t *testing.T) {
	contracttest.Journal(t, func(*testing.T) contracttest.JournalSubject {
		j := fake.NewJournal()
		j.World().Now = contracttest.Morning()
		return contracttest.JournalSubject{Journal: j, Now: &j.World().Now}
	})
}
