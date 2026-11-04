package wire

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/background/adapter/launchd"
	"github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/engine"
)

func (m machine) jobs(budget time.Duration) *launchd.Jobs {
	return launchd.New(m.host.Run, m.host.FS, launchd.Options{
		Home: m.host.Env.Home(), Root: m.systemRoot(), UID: m.host.Env.Euid(), Budget: budget,
	})
}

func (m machine) daemonJob() contract.Ref {
	return contract.Ref{Domain: contract.Domain{UID: m.host.Env.Euid()}, Label: engine.Label, File: catalog.DaemonPlist(m.host.Env.Home())}
}

type engineJob struct {
	jobs contract.Jobs
	ref  contract.Ref
}

func (m machine) engineJob() engineJob {
	return engineJob{jobs: m.jobs(engine.JobBudget), ref: m.daemonJob()}
}

func (j engineJob) Loaded(ctx context.Context) bool { return j.jobs.Read(ctx, j.ref).Loaded }

func (j engineJob) Load(ctx context.Context) error { return booted(j.jobs.Load(ctx, j.ref)) }

func (j engineJob) Restart(ctx context.Context) error { return booted(j.jobs.Restart(ctx, j.ref)) }

func (j engineJob) Stop(ctx context.Context) error { return booted(j.jobs.Stop(ctx, j.ref)) }

func booted(err error) error {
	var failed *contract.Failed
	if !errors.As(err, &failed) {
		return err
	}
	cause := failed.Err
	if cause == nil {
		cause = errors.New("exited " + strconv.Itoa(failed.Exit) + " " + failed.Stderr)
	}
	return &engine.BootError{Message: failed.Verb() + ": " + cause.Error(), See: failed.See, Cause: cause}
}
