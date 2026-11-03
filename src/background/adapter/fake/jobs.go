package fake

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jterrazz/studio-station/src/background"
	"github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/world"
)

const command = "jobs"

type Jobs struct {
	w   *world.World
	mac string
	pid int
}

var _ contract.Jobs = (*Jobs)(nil)

func New(w *world.World, mac string) *Jobs { return &Jobs{w: w, mac: mac} }

func (j *Jobs) here() *world.Mac {
	if j.w.Macs == nil {
		j.w.Macs = map[string]*world.Mac{}
	}
	m := j.w.Macs[j.mac]
	if m == nil {
		m = &world.Mac{}
		j.w.Macs[j.mac] = m
	}
	if m.Jobs == nil {
		m.Jobs = map[string]*world.Job{}
	}
	return m
}

func (j *Jobs) Present() bool { return j.w.Macs[j.mac] != nil }

func (j *Jobs) Names() contract.Names {
	return contract.Names{Manager: "the job manager", Command: command}
}

func (j *Jobs) failed(verb string, r contract.Ref) error {
	return &contract.Failed{Argv: []string{command, verb, r.Label}, Exit: 1, Stderr: r.Label + " is not loaded", See: command + " read " + r.Label}
}

func (j *Jobs) List(context.Context) ([]background.Job, error) {
	jobs := make([]background.Job, 0, len(j.here().Jobs))
	for _, job := range j.here().Jobs {
		jobs = append(jobs, job.Job)
	}
	slices.SortFunc(jobs, func(a, b background.Job) int { return strings.Compare(a.Label, b.Label) })
	return jobs, nil
}

func (j *Jobs) Read(_ context.Context, r contract.Ref) contract.Status {
	job := j.here().Jobs[r.Label]
	switch {
	case job == nil || !job.Loaded:
		return contract.Status{}
	case job.Running:
		return contract.Status{Loaded: true, State: "running"}
	}
	return contract.Status{Loaded: true, State: "waiting"}
}

func (j *Jobs) Overrides(context.Context, contract.Domain) map[string]bool {
	return j.here().Overrides
}

func (j *Jobs) Install(_ context.Context, spec contract.JobSpec) (bool, error) {
	definition := fmt.Sprintf("%+v", spec)
	m := j.here()
	job := m.Jobs[spec.Label]
	if job != nil && job.Definition == definition {
		return false, nil
	}
	if job == nil {
		job = &world.Job{}
		m.Jobs[spec.Label] = job
	}
	job.Label, job.File, job.Scope, job.Definition = spec.Label, spec.File, background.ScopeUser, definition
	job.Program = ""
	if len(spec.Program) > 0 {
		job.Program = spec.Program[0]
	}
	job.ProgramExists, job.OwnerExists = true, true
	return true, nil
}

func (j *Jobs) Load(_ context.Context, r contract.Ref) error {
	job := j.here().Jobs[r.Label]
	if job == nil || job.File != r.File {
		return &contract.Failed{Argv: []string{command, "load", r.Label}, Exit: 1, Stderr: "no definition at " + r.File, See: command + " read " + r.Label}
	}
	job.Loaded = true
	return nil
}

func (j *Jobs) Start(_ context.Context, r contract.Ref) error {
	job := j.here().Jobs[r.Label]
	if job == nil || !job.Loaded {
		return j.failed("start", r)
	}
	if !job.Running {
		j.pid++
		job.Running, job.PID, job.LastExit = true, j.pid, 0
	}
	return nil
}

func (j *Jobs) Restart(ctx context.Context, r contract.Ref) error {
	job := j.here().Jobs[r.Label]
	if job == nil || !job.Loaded {
		return j.failed("restart", r)
	}
	job.Running = false
	return j.Start(ctx, r)
}

func (j *Jobs) Stop(_ context.Context, r contract.Ref) error {
	job := j.here().Jobs[r.Label]
	if job == nil || !job.Loaded {
		return j.failed("stop", r)
	}
	job.Loaded, job.Running, job.PID = false, false, 0
	return nil
}

func (j *Jobs) Remove(ctx context.Context, r contract.Ref) error {
	_ = j.Stop(ctx, r)
	delete(j.here().Jobs, r.Label)
	return nil
}
