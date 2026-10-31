package machines

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/system"
)

const agentPrefix = "com.jterrazz.station.vm."

func agentLabel(name string) string { return agentPrefix + name }

func (s *Service) plistPath(name string) string {
	return path.Join(s.h.Env.Home(), "Library", "LaunchAgents", agentLabel(name)+".plist")
}

func (s *Service) logPath(name string) string { return path.Join(s.o.Paths.State, "logs", name+".log") }

func (s *Service) consolePath(name string) string {
	return path.Join(s.o.Paths.State, "consoles", name)
}

func (s *Service) guiDomain() string { return "gui/" + strconv.Itoa(s.h.Env.Euid()) }

func (s *Service) agentTarget(name string) string { return s.guiDomain() + "/" + agentLabel(name) }

func (s *Service) launchctl(ctx context.Context, args ...string) (system.Output, error) {
	return s.h.Run.Output(ctx, system.Cmd{Name: "launchctl", Args: args})
}

func (s *Service) launchctlOK(ctx context.Context, args ...string) error {
	out, err := s.launchctl(ctx, args...)
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		return fmt.Errorf("launchctl %s exited %d: %s", args[0], out.ExitCode, strings.TrimSpace(out.Stderr+out.Stdout))
	}
	return nil
}

func (s *Service) loaded(ctx context.Context, name string) bool {
	out, err := s.launchctl(ctx, "print", s.agentTarget(name))
	return err == nil && out.ExitCode == 0
}

// invariant: every VM runs under its own agent, which runs the supervisor and not
// tart itself: only the supervisor reads the guest's poweroff intent. A temporary
// agent never loads at login (a host reboot leaves the VM stale); a lasting one
// does, and launchd restarts it after a crash but not after a declared poweroff.
func (s *Service) agentPlist(name string, l Lifetime) string {
	env := map[string]string{}
	for k, v := range s.o.AgentEnv {
		env[k] = v
	}
	env["STATION_VM_AGENT"] = name
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	b.WriteString("\t<key>Label</key>\n\t<string>" + xmlText(agentLabel(name)) + "</string>\n")
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, arg := range []string{s.h.Env.Executable(), "machines", "supervise", name} {
		b.WriteString("\t\t<string>" + xmlText(arg) + "</string>\n")
	}
	b.WriteString("\t</array>\n\t<key>EnvironmentVariables</key>\n\t<dict>\n")
	for _, k := range keys {
		b.WriteString("\t\t<key>" + xmlText(k) + "</key>\n\t\t<string>" + xmlText(env[k]) + "</string>\n")
	}
	b.WriteString("\t</dict>\n")
	if l == Lasting {
		b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n")
		b.WriteString("\t<key>KeepAlive</key>\n\t<dict>\n\t\t<key>SuccessfulExit</key>\n\t\t<false/>\n\t</dict>\n")
	} else {
		b.WriteString("\t<key>RunAtLoad</key>\n\t<false/>\n")
	}
	log := xmlText(s.logPath(name))
	b.WriteString("\t<key>StandardOutPath</key>\n\t<string>" + log + "</string>\n")
	b.WriteString("\t<key>StandardErrorPath</key>\n\t<string>" + log + "</string>\n")
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

func xmlText(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(v)
}

func (s *Service) installAgent(ctx context.Context, name string, l Lifetime) error {
	file := s.plistPath(name)
	for _, dir := range []string{path.Dir(file), path.Dir(s.logPath(name)), path.Dir(s.consolePath(name))} {
		if err := s.h.FS.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	if err := s.h.FS.WriteFile(file, []byte(s.agentPlist(name, l)), 0o600); err != nil {
		return err
	}
	if err := s.launchctlOK(ctx, "bootstrap", s.guiDomain(), file); err != nil {
		return err
	}
	return s.launchctlOK(ctx, "kickstart", s.agentTarget(name))
}
