package world

import (
	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings"
)

type World struct {
	Macs map[string]*Mac
}

type Mac struct {
	Jobs map[string]*Job
	// value: the job manager's overrides by label: enabled or not.
	Overrides map[string]bool
	// value: what station keeps in this Mac's vault, by name.
	Vault map[string][]byte
	// value: the secrets another tool keeps on this Mac, by name.
	Secrets map[string]string
	// value: who each provider says is signed in, by provider; a provider
	// absent here has no row on this Mac.
	Identities map[string]accounts.Identity
	// value: the facts this Mac answers, by fact; a fact absent here is one
	// it cannot look at.
	Facts map[catalog.Fact]settings.Fact
}

type Job struct {
	background.Job
	// value: the definition as it was written, so writing it again is seen.
	Definition string
}
