package fake

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/jterrazz/studio-station/src/machines/contract"
)

type SSHConfig struct {
	mu      sync.Mutex
	blocks  map[string]contract.HostBlock
	foreign map[string]bool
}

func NewSSHConfig() *SSHConfig {
	return &SSHConfig{blocks: map[string]contract.HostBlock{}, foreign: map[string]bool{}}
}

func (c *SSHConfig) Foreign(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.foreign[name] = true
}

func (c *SSHConfig) Managed() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	names := make([]string, 0, len(c.blocks))
	for name := range c.blocks {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func (c *SSHConfig) put(blocks []contract.HostBlock) error {
	var refused error
	for _, b := range blocks {
		if c.foreign[b.Name] {
			if refused == nil {
				refused = &contract.SSHConfigError{Path: "~/.ssh/config", Cause: errors.New("Host " + b.Name + " already defined outside any managed block")}
			}
			continue
		}
		c.blocks[b.Name] = b
	}
	return refused
}

func (c *SSHConfig) Put(_ context.Context, blocks []contract.HostBlock) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.put(blocks)
}

func (c *SSHConfig) Replace(_ context.Context, blocks []contract.HostBlock) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	refused := c.put(blocks)
	for name := range c.blocks {
		if !slices.ContainsFunc(blocks, func(b contract.HostBlock) bool { return b.Name == name }) {
			delete(c.blocks, name)
		}
	}
	return refused
}

func (c *SSHConfig) Remove(_ context.Context, names ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, name := range names {
		delete(c.blocks, name)
	}
	return nil
}
