package wire

import (
	"context"
	"time"

	machinescalls "github.com/jterrazz/studio-station/src/machines/calls"
	"github.com/jterrazz/studio-station/src/platform/local"
)

type thisMac struct{ h local.Host }

func (t thisMac) Now() time.Time { return t.h.Clock.Now() }
func (t thisMac) Sleep(ctx context.Context, d time.Duration) error {
	return t.h.Clock.Sleep(ctx, d)
}
func (t thisMac) Home() string     { return t.h.Env.Home() }
func (t thisMac) UID() int         { return t.h.Env.Euid() }
func (t thisMac) User() string     { return t.h.Env.User() }
func (t thisMac) Hostname() string { return t.h.Env.Hostname() }
func (t thisMac) Exists(path string) bool {
	_, err := t.h.FS.Stat(path)
	return err == nil
}
func (t thisMac) Remove(path string) error { return t.h.FS.Remove(path) }

func (t thisMac) Names(dir string) ([]string, error) {
	entries, err := t.h.FS.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

type machinesDaemon struct{ hostDaemon }

func (d machinesDaemon) GuestExec(ctx context.Context, vm, line, stdin string) (machinescalls.GuestRan, error) {
	out, err := d.hostDaemon.GuestExec(ctx, vm, line, stdin)
	return machinescalls.GuestRan{Stdout: out.Stdout, Stderr: out.Stderr, ExitCode: out.ExitCode}, err
}
