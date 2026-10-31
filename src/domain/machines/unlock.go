package machines

import (
	"context"
	"path"
	"strings"

	"github.com/jterrazz/studio-station/src/system"
)

func (s *Service) Unlock(ctx context.Context, name string) error {
	if err := s.CheckHost(); err != nil {
		return err
	}
	m, err := s.RemoteHost(name, "unlock")
	if err != nil {
		return err
	}
	if mkErr := s.h.FS.MkdirAll(s.knownHostsDir(), 0o700); mkErr != nil {
		return mkErr
	}
	// invariant: pre-boot sshd advertises another host key than the running OS, so
	// the whole conversation keeps its own known_hosts file.
	preboot := []string{
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "UserKnownHostsFile=" + path.Join(s.knownHostsDir(), m.Name+".preboot"),
	}
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "ssh", Args: concat(batchSSH(), preboot, []string{m.Name, "true"})})
	if err != nil {
		return &UnreachableError{Name: name}
	}
	// outside: a host in FileVault's pre-boot state answers but refuses the key with
	// "Permission denied": that is the state unlock exists for, not unreachable.
	if out.ExitCode != 0 && !strings.Contains(out.Stderr, "Permission denied") {
		return &UnreachableError{Name: name}
	}
	// outside: ssh disconnects as soon as the password unlocks the disk, so the
	// session's own exit is the success path and carries no verdict.
	_, err = s.h.Run.Attach(ctx, system.Cmd{Name: "ssh", Args: concat(
		[]string{"-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no"}, preboot, []string{m.Name},
	)}, system.ToStdout)
	return err
}

func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
