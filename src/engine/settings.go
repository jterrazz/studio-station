package engine

import (
	"maps"

	"github.com/jterrazz/studio-station/src/settings"
)

type SettingsKept map[string]settings.LastRead

func (d *Daemon) KeepSettings(vm string, read settings.LastRead) {
	// invariant: the last settings read of each workspace, held in this
	// engine's memory alone, for the matrix to show while it is stopped.
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.settings == nil {
		d.settings = SettingsKept{}
	}
	d.settings[vm] = read
}

func (d *Daemon) LastSettings(vm string) (settings.LastRead, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	read, ok := d.settings[vm]
	return read, ok
}

func (d *Daemon) SettingsKept() SettingsKept {
	d.mu.Lock()
	defer d.mu.Unlock()
	return maps.Clone(d.settings)
}

func (d *Daemon) RememberSettings(k SettingsKept) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.settings = maps.Clone(k)
}
