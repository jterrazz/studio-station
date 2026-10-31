package machines

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	natRetry      = time.Second
	natReady      = 60 * time.Second
	firstbootWait = 5 * time.Minute
	ipWait        = 60 * time.Second
)

// invariant: before a VM is on the tailnet, the image key IS the trust: the VM's
// host key is new at every clone, so it is neither checked nor kept.
func (s *Service) natArgs(ip, user string, command ...string) []string {
	args := make([]string, 0, 13+len(command))
	args = append(args,
		"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-o", "LogLevel=ERROR",
		"-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-i", s.img.KeyPath(), user+"@"+ip,
	)
	return append(args, command...)
}

func (s *Service) nat(ctx context.Context, ip, user, stdin string, timeout time.Duration, command ...string) (system.Output, error) {
	return s.h.Run.Output(ctx, system.Cmd{Name: "ssh", Args: s.natArgs(ip, user, command...), Stdin: stdin, Timeout: timeout})
}

func renderFirstboot(script, name string, l Lifetime, authKey string) string {
	return strings.NewReplacer("{{name}}", name, "{{lifetime}}", string(l), "{{authkey}}", authKey).Replace(script)
}

// hazard: the auth key travels only in the script on ssh's stdin, which the
// guest reads into `tailscale up --auth-key=file:/dev/stdin`: never argv, never disk.
func (s *Service) firstBoot(ctx context.Context, img catalog.Image, ip, user, name string, l Lifetime, authKey string) (string, error) {
	script, err := catalog.Recipes.ReadFile(img.Firstboot)
	if err != nil {
		return "", err
	}
	stdin := renderFirstboot(string(script), name, l, authKey)
	for waited := time.Duration(0); ; waited += natRetry {
		out, err := s.nat(ctx, ip, user, stdin, firstbootWait, "sudo", "bash", "-s")
		if err != nil {
			return "", err
		}
		switch {
		case out.ExitCode == 0:
			id := lastLine(out.Stdout)
			if id == "" {
				return "", errors.New("the first boot printed no node id")
			}
			return id, nil
		case out.ExitCode != 255:
			return "", errors.New("the first boot exited " + strconv.Itoa(out.ExitCode) + ": " + firstLine(out.Stderr))
		}
		if waited >= natReady {
			return "", errors.New("sshd did not answer in " + natReady.String())
		}
		if err := s.h.Clock.Sleep(ctx, natRetry); err != nil {
			return "", err
		}
	}
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
