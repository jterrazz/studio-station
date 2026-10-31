package tart

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/system"
)

const commandGrace = 15 * time.Second

type engine struct {
	run  system.Runner
	path string
}

func New(run system.Runner) system.Engine {
	// invariant: the tart path is resolved here, absolute, because a launchd agent has no PATH of the owner's.
	path, ok := run.LookPath("tart")
	if !ok {
		path = "tart"
	}
	return &engine{run: run, path: path}
}

type entry struct {
	Source string `json:"Source"`
	Name   string `json:"Name"`
	Disk   *int   `json:"Disk"`
	Size   int    `json:"Size"`
	State  string `json:"State"`
}

func parseList(data string) ([]system.VMInfo, error) {
	var entries []entry
	if err := json.Unmarshal([]byte(data), &entries); err != nil {
		return nil, fmt.Errorf("tart list: unreadable output: %w", err)
	}
	vms := []system.VMInfo{}
	for _, e := range entries {
		if e.Source != "local" {
			continue
		}
		vms = append(vms, system.VMInfo{Name: e.Name, State: stateOf(e.State), DiskGB: e.Disk, SizeGB: e.Size})
	}
	return vms, nil
}

func stateOf(s string) system.VMState {
	switch strings.ToLower(s) {
	case "running":
		return system.VMRunning
	case "suspended":
		return system.VMSuspended
	default:
		return system.VMStopped
	}
}

func (e *engine) exec(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	out, err := e.run.Output(ctx, system.Cmd{Name: e.path, Args: args, Timeout: timeout})
	if err != nil {
		return "", fmt.Errorf("tart %s: %w", args[0], err)
	}
	if out.ExitCode != 0 {
		return out.Stdout, fmt.Errorf("tart %s: exit %d: %s", args[0], out.ExitCode, strings.TrimSpace(out.Stderr))
	}
	return out.Stdout, nil
}

func seconds(d time.Duration) string {
	return strconv.Itoa(int((d + time.Second - 1) / time.Second))
}

func (e *engine) Version(ctx context.Context) (string, error) {
	out, err := e.exec(ctx, 0, "--version")
	return strings.TrimSpace(out), err
}

func (e *engine) List(ctx context.Context) ([]system.VMInfo, error) {
	out, err := e.exec(ctx, 0, "list", "--source", "local", "--format", "json")
	if err != nil {
		return nil, err
	}
	return parseList(out)
}

func (e *engine) Pull(ctx context.Context, ref string) error {
	_, err := e.exec(ctx, 0, "pull", ref)
	return err
}

func (e *engine) Clone(ctx context.Context, from, to string) error {
	_, err := e.exec(ctx, 0, "clone", from, to)
	return err
}

func (e *engine) Rename(ctx context.Context, from, to string) error {
	_, err := e.exec(ctx, 0, "rename", from, to)
	return err
}

func (e *engine) Resize(ctx context.Context, name string, r system.Resources) error {
	args := []string{"set", name}
	if r.CPUs > 0 {
		args = append(args, "--cpu", strconv.Itoa(r.CPUs))
	}
	if r.MemoryBytes > 0 {
		args = append(args, "--memory", strconv.FormatInt((r.MemoryBytes+1<<20-1)>>20, 10))
	}
	if r.DiskBytes > 0 {
		args = append(args, "--disk-size", strconv.FormatInt((r.DiskBytes+1e9-1)/1e9, 10))
	}
	_, err := e.exec(ctx, 0, args...)
	return err
}

func (e *engine) RunArgv(name, console string) []string {
	return []string{e.path, "run", "--no-graphics", "--no-audio", "--serial-path", console, name}
}

func (e *engine) Stop(ctx context.Context, name string, wait time.Duration) error {
	_, err := e.exec(ctx, wait+commandGrace, "stop", "--timeout", seconds(wait), name)
	return err
}

func (e *engine) Delete(ctx context.Context, name string) error {
	_, err := e.exec(ctx, 0, "delete", name)
	return err
}

func (e *engine) IP(ctx context.Context, name string, wait time.Duration) (string, error) {
	out, err := e.exec(ctx, wait+commandGrace, "ip", name, "--wait", seconds(wait))
	if err != nil {
		return "", errors.Join(system.ErrTimeout, err)
	}
	return strings.TrimSpace(out), nil
}
