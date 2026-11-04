package config

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/background/contract"
	"github.com/jterrazz/studio-station/src/catalog"
)

const (
	lockLabel  = "ai.jterrazz.lock-after-login"
	lockScript = agentHome + "/.openclaw/scripts/lock-after-login.sh"
	lockPlist  = agentHome + "/Library/LaunchAgents/" + lockLabel + ".plist"
	lockLogDir = agentHome + "/.openclaw/logs"
)

func (s *Service) installLockAfterLogin(ctx context.Context) (Change, error) {
	if err := s.requireDarwin(); err != nil {
		return Change{}, err
	}
	if !s.exists(lockScript) {
		return Change{}, errors.New("missing " + lockScript + ", expected the openclaw lock-after-login script")
	}
	change := Change{Before: s.lockFacts(ctx)}
	// invariant: launchd executes this script: the x bit is the point.
	if err := s.h.FS.Chmod(lockScript, 0o755); err != nil {
		return change, err
	}
	if err := s.h.FS.MkdirAll(lockLogDir, 0o750); err != nil {
		return change, err
	}
	if err := s.write(lockPlist, []byte(lockPlistText()), 0o600); err != nil {
		return change, err
	}
	uid, gid, known := s.agentIDs(ctx)
	if known {
		_ = s.h.FS.Chown(lockPlist, uid, gid)
		_ = s.h.FS.Chown(lockLogDir, uid, gid)
	}
	change.Steps = append(change.Steps, okStep("plist", lockPlist))

	command := s.o.Jobs.Names().Command
	owner, _ := s.probe(ctx, "stat", "-f", "%Su", "/dev/console")
	if owner == agentUser && known {
		// invariant: stopped first, so installing over a loaded agent succeeds.
		_ = s.o.Jobs.Stop(ctx, lockJob(uid))
		if err := s.o.Jobs.Load(ctx, lockJob(uid)); err != nil {
			change.Steps = append(change.Steps, Step{Label: command + " bootstrap", State: catalog.StateDegraded, Detail: "it loads at the next auto-login anyway: " + firstLine(jobFailure(err))})
		} else {
			change.Steps = append(change.Steps, okStep(command+" bootstrap", "gui/"+strconv.Itoa(uid)))
		}
	} else {
		change.Steps = append(change.Steps, skipStep(command, "no GUI session for "+agentUser+": the agent loads at the next auto-login"))
	}
	change.After = s.lockFacts(ctx)
	return change, nil
}

func (s *Service) uninstallLockAfterLogin(ctx context.Context) (Change, error) {
	if err := s.requireDarwin(); err != nil {
		return Change{}, err
	}
	if uid, _, known := s.agentIDs(ctx); known {
		_ = s.o.Jobs.Stop(ctx, lockJob(uid))
	}
	if err := s.h.FS.Remove(lockPlist); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Change{}, err
	}
	return Change{Steps: []Step{okStep("plist", "removed")}, After: s.lockFacts(ctx)}, nil
}

func (s *Service) checkLockAfterLogin(ctx context.Context) (state catalog.State, detail string) {
	if !s.exists(lockPlist) {
		return catalog.StateAbsent, ""
	}
	if uid, _, known := s.agentIDs(ctx); known {
		if s.o.Jobs.Read(ctx, lockJob(uid)).Loaded {
			return catalog.StateOK, "agent loaded"
		}
	}
	return catalog.StateDegraded, "agent installed, not loaded"
}

func (s *Service) lockFacts(ctx context.Context) []Fact {
	facts := []Fact{{Key: "plist", Value: presence(s.exists(lockPlist))}, {Key: "script", Value: presence(s.exists(lockScript))}}
	if uid, _, known := s.agentIDs(ctx); known {
		status := s.o.Jobs.Read(ctx, lockJob(uid))
		value := "not loaded in gui/" + strconv.Itoa(uid)
		switch {
		case status.State != "":
			value = status.State
		case status.Loaded:
			value = "loaded"
		}
		facts = append(facts, Fact{Key: s.o.Jobs.Names().Command, Value: value})
	}
	return facts
}

func lockJob(uid int) contract.Ref {
	return contract.Ref{Domain: contract.Domain{UID: uid}, Label: lockLabel, File: lockPlist}
}

func jobFailure(err error) string {
	var failed *contract.Failed
	if !errors.As(err, &failed) || failed.Err != nil {
		return err.Error()
	}
	reason := failed.Stderr
	if reason == "" {
		reason = failed.Stdout
	}
	if reason == "" {
		reason = "exited " + strconv.Itoa(failed.Exit)
	}
	return strings.Join(failed.Argv, " ") + ": " + reason
}

func (s *Service) agentIDs(ctx context.Context) (uid, gid int, known bool) {
	u, ok := s.probe(ctx, "id", "-u", agentUser)
	if !ok {
		return 0, 0, false
	}
	g, ok := s.probe(ctx, "id", "-g", agentUser)
	if !ok {
		return 0, 0, false
	}
	uid, uerr := strconv.Atoi(u)
	gid, gerr := strconv.Atoi(g)
	return uid, gid, uerr == nil && gerr == nil
}

func presence(ok bool) string {
	if ok {
		return "present"
	}
	return "absent"
}

func lockPlistText() string {
	return strings.Join([]string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">`,
		`<plist version="1.0">`,
		`<dict>`,
		`    <key>Label</key>`,
		`    <string>` + lockLabel + `</string>`,
		`    <key>ProgramArguments</key>`,
		`    <array>`,
		`        <string>/bin/zsh</string>`,
		`        <string>` + lockScript + `</string>`,
		`    </array>`,
		`    <key>RunAtLoad</key>`,
		`    <true/>`,
		`    <key>StandardOutPath</key>`,
		`    <string>` + path.Join(lockLogDir, "lock-after-login.log") + `</string>`,
		`    <key>StandardErrorPath</key>`,
		`    <string>` + path.Join(lockLogDir, "lock-after-login.err.log") + `</string>`,
		`    <key>ProcessType</key>`,
		`    <string>Interactive</string>`,
		`</dict>`,
		`</plist>`,
		"",
	}, "\n")
}
