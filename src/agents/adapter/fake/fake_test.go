package fake_test

import (
	"testing"

	"github.com/jterrazz/studio-station/src/agents"
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

func TestSession(t *testing.T) {
	contracttest.Session(t, func(*testing.T) contracttest.SessionSubject {
		s := fake.NewSession()
		return contracttest.SessionSubject{Session: s, Opened: s.Opened, Typed: s.Typed, Lines: func() []string { return nil }}
	})
}

func TestForge(t *testing.T) {
	contracttest.Forge(t, func(*testing.T) contracttest.ForgeSubject {
		f := fake.NewForge(42)
		return contracttest.ForgeSubject{Forge: f, Folder: "/Users/admin/app", Open: 42, Merged: f.Merged}
	})
}

func TestRepo(t *testing.T) {
	contracttest.Repo(t, func(*testing.T) contracttest.RepoSubject {
		want := agents.Change{Files: 3, Plus: 86, Minus: 12}
		return contracttest.RepoSubject{Repo: fake.NewRepo("/Users/admin/app", want), Folder: "/Users/admin/app", Unchanged: "/Users/admin/api", Base: "main", Want: want}
	})
}
