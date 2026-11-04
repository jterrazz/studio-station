package world

import "github.com/jterrazz/studio-station/src/background"

type World struct {
	Macs map[string]*Mac
}

type Mac struct {
	Jobs map[string]*Job
	// value: the job manager's overrides by label: enabled or not.
	Overrides map[string]bool
}

type Job struct {
	background.Job
	// value: the definition as it was written, so writing it again is seen.
	Definition string
}
