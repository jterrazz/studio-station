package machines

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const dialBudget = 2 * time.Second

func batchSSH() []string { return []string{"-o", "ConnectTimeout=2", "-o", "BatchMode=yes"} }

func (s *Service) Probe(ctx context.Context, name string) (ProbeReport, error) {
	m, err := s.Remote(name)
	if err != nil {
		return ProbeReport{}, err
	}
	ip := s.resolve(ctx, name)
	report := ProbeReport{Name: name, User: m.User, IP: ip}

	report.Checks = append(report.Checks, s.ping(ctx, ip))
	ssh := s.sshCheck(ctx, name)
	report.Checks = append(report.Checks, ssh)
	if m.Kind == catalog.Host {
		report.Checks = append(report.Checks,
			s.port(ctx, "openclaw gateway", ip, s.o.Ports.OpenClaw),
			s.port(ctx, "hermes gateway", ip, s.o.Ports.Hermes),
		)
		if ssh.State == catalog.StateOK {
			report.Console, report.Checks = s.console(ctx, name, report.Checks)
		}
	}
	return report, nil
}

func (s *Service) resolve(ctx context.Context, name string) string {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "ssh", Args: []string{"-G", name}})
	if err == nil && out.ExitCode == 0 {
		for _, line := range strings.Split(out.Stdout, "\n") {
			if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == "hostname" {
				return fields[1]
			}
		}
	}
	return name
}

func (s *Service) ping(ctx context.Context, ip string) Check {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "ping", Args: []string{"-c", "1", "-W", "1000", ip}})
	if err == nil && out.ExitCode == 0 {
		return Check{Name: "ping", State: catalog.StateOK, Value: ip}
	}
	return Check{Name: "ping", State: catalog.StateBroken, Value: ip}
}

func (s *Service) sshCheck(ctx context.Context, name string) Check {
	if s.sshTrue(ctx, name) {
		return Check{Name: "ssh", State: catalog.StateOK, Value: "BatchMode auth ok"}
	}
	return Check{Name: "ssh", State: catalog.StateBroken, Value: "auth failed", Detail: "or pre-boot: try station machines unlock " + name}
}

func (s *Service) port(ctx context.Context, label, ip string, port int) Check {
	p := strconv.Itoa(port)
	if s.h.Net.Dial(ctx, ip, port, dialBudget) {
		return Check{Name: label, State: catalog.StateOK, Value: ip, Detail: "port " + p + " open"}
	}
	return Check{Name: label, State: catalog.StateBroken, Value: ip, Detail: "port " + p + " closed"}
}

func (s *Service) console(ctx context.Context, name string, checks []Check) (string, []Check) {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "ssh", Args: concat(batchSSH(), []string{name, "stat -f %Su /dev/console"})})
	if err != nil || out.ExitCode != 0 {
		return "", checks
	}
	owner := strings.TrimSpace(out.Stdout)
	check := Check{Name: "console owner", State: catalog.StateOK, Value: owner}
	if owner == "root" {
		check.State, check.Detail = catalog.StateDegraded, "loginwindow, no GUI session"
	}
	return owner, append(checks, check)
}

func (s *Service) sshTrue(ctx context.Context, name string) bool {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "ssh", Args: concat(batchSSH(), []string{name, "true"})})
	return err == nil && out.ExitCode == 0
}
