package fake

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/world"
)

type SSHConfig struct {
	// invariant: the ssh client config of one Mac of the world: its managed
	// blocks, and the hosts the owner defined outside them.
	w   *world.World
	mac string
}

func SSHConfigOf(w *world.World, mac string) *SSHConfig { return &SSHConfig{w: w, mac: mac} }

func NewSSHConfig() *SSHConfig {
	w := &world.World{ThisMac: "here"}
	w.Mac("here")
	return SSHConfigOf(w, "here")
}

func (c *SSHConfig) config() *world.SSH {
	m := c.w.Mac(c.mac)
	if m.SSH == nil {
		m.SSH = &world.SSH{}
	}
	if m.SSH.Blocks == nil {
		m.SSH.Blocks = map[string]world.HostBlock{}
	}
	return m.SSH
}

func (c *SSHConfig) Foreign(name string) {
	c.w.Lock()
	defer c.w.Unlock()
	cfg := c.config()
	cfg.Foreign = append(cfg.Foreign, name)
}

func (c *SSHConfig) Managed() []string {
	c.w.Lock()
	defer c.w.Unlock()
	return slices.Sorted(maps.Keys(c.config().Blocks))
}

func (c *SSHConfig) put(blocks []contract.HostBlock) error {
	cfg := c.config()
	var refused error
	for _, b := range blocks {
		if slices.Contains(cfg.Foreign, b.Name) {
			if refused == nil {
				refused = &contract.SSHConfigError{Path: "~/.ssh/config", Cause: errors.New("Host " + b.Name + " already defined outside any managed block")}
			}
			continue
		}
		cfg.Blocks[b.Name] = world.HostBlock{User: b.User, Key: b.Key, Behind: b.Behind, ThisMac: b.Here}
	}
	return refused
}

func (c *SSHConfig) Put(_ context.Context, blocks []contract.HostBlock) error {
	c.w.Lock()
	defer c.w.Unlock()
	return c.put(blocks)
}

func (c *SSHConfig) Replace(_ context.Context, blocks []contract.HostBlock) error {
	c.w.Lock()
	defer c.w.Unlock()
	refused := c.put(blocks)
	maps.DeleteFunc(c.config().Blocks, func(name string, _ world.HostBlock) bool {
		return !slices.ContainsFunc(blocks, func(b contract.HostBlock) bool { return b.Name == name })
	})
	return refused
}

func (c *SSHConfig) Remove(_ context.Context, names ...string) error {
	c.w.Lock()
	defer c.w.Unlock()
	for _, name := range names {
		delete(c.config().Blocks, name)
	}
	return nil
}
