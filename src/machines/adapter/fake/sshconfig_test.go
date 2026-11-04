package fake

import (
	"testing"

	"github.com/jterrazz/studio-station/src/machines/contract/contracttest"
)

func TestTheFakeSSHConfigKeepsTheContract(t *testing.T) {
	contracttest.SSHConfig(t, func(*testing.T) contracttest.SSHConfigSubject {
		c := NewSSHConfig()
		return contracttest.SSHConfigSubject{
			Config:  c,
			Managed: func(*testing.T) []string { return c.Managed() },
			Foreign: func(_ *testing.T, name string) { c.Foreign(name) },
		}
	})
}
