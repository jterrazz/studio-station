package machines

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/jterrazz/studio-station/src/domain/settings"
)

// value: the name sits in both markers, so each block names itself to a human who opens the file.
const (
	beginPrefix = "# >>> jterrazz machine "
	beginSuffix = " >>>"
	endPrefix   = "# <<< jterrazz machine "
	endSuffix   = " <<<"
)

func beginMarker(name string) string { return beginPrefix + name + beginSuffix }
func endMarker(name string) string   { return endPrefix + name + endSuffix }

type target struct {
	Name, User, Key string
	Host            bool
}

func (s *Service) block(t target) string {
	// invariant: the name IS the HostName, and each machine keeps its own
	// known_hosts, so a name reused by a new VM meets no changed host key.
	lines := []string{beginMarker(t.Name), "Host " + t.Name, "  HostName " + t.Name}
	if t.User != "" {
		lines = append(lines, "  User "+t.User)
	}
	if t.Host {
		key := t.Key
		if key == "" {
			key = "~/.ssh/id_ed25519"
		}
		lines = append(lines, "  IdentityFile "+key)
	}
	lines = append(lines,
		"  ServerAliveInterval 30",
		"  ServerAliveCountMax 3",
		"  UserKnownHostsFile "+s.tilde(path.Join(s.knownHostsDir(), t.Name)),
		"  StrictHostKeyChecking accept-new",
	)
	if s.tn.Mode() == string(settings.ModeUserspace) {
		lines = append(lines, "  ProxyCommand tailscale "+strings.Join(s.tn.Argv("nc", "%h", "%p"), " "))
	}
	return strings.Join(append(lines, endMarker(t.Name), ""), "\n")
}

func (s *Service) tilde(p string) string {
	home := strings.TrimSuffix(s.h.Env.Home(), "/")
	if home != "" && strings.HasPrefix(p, home+"/") {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

func (s *Service) SSHBlocks(_ context.Context, f Fleet) error {
	// hazard: the blocks of machines the fleet no longer holds are pruned only when
	// every host answered; an unreachable host's VMs are unknown, not gone.
	cfg, err := s.store.Load()
	if err != nil {
		return err
	}
	var targets []target
	complete := true
	for _, host := range f.Hosts {
		complete = complete && host.Reachable
		if !host.Here {
			h := cfg.Hosts[host.Name]
			targets = append(targets, target{Name: host.Name, User: h.User, Key: h.Key, Host: true})
		}
		for _, vm := range host.VMs {
			targets = append(targets, target{Name: vm.Name, User: s.vmUser(cfg)})
		}
	}
	if err := s.h.FS.MkdirAll(s.knownHostsDir(), 0o700); err != nil {
		return &SSHConfigError{Path: s.knownHostsDir(), Cause: err}
	}
	return s.editConfig(targets, complete, nil)
}

func (s *Service) editConfig(upsert []target, prune bool, remove []string) error {
	file := s.sshConfig()
	existing, err := s.h.FS.ReadFile(file)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return &SSHConfigError{Path: file, Cause: err}
	}
	updated := existing
	var conflict error
	for _, t := range upsert {
		next, clash := upsertBlock(updated, t.Name, s.block(t))
		if clash != "" && conflict == nil {
			conflict = fmt.Errorf("refusing to modify %s: %s", file, clash)
			continue
		}
		updated = next
	}
	for _, name := range remove {
		updated = removeBlock(updated, name)
	}
	if prune {
		keep := map[string]bool{}
		for _, t := range upsert {
			keep[t.Name] = true
		}
		for _, name := range managedNames(updated) {
			if !keep[name] {
				updated = removeBlock(updated, name)
			}
		}
	}
	if !bytes.Equal(updated, existing) {
		if err := s.h.FS.MkdirAll(path.Dir(file), 0o700); err != nil {
			return &SSHConfigError{Path: file, Cause: err}
		}
		if err := s.h.FS.WriteFile(file, updated, 0o600); err != nil {
			return &SSHConfigError{Path: file, Cause: err}
		}
	}
	if conflict != nil {
		return &SSHConfigError{Path: file, Cause: conflict}
	}
	return nil
}

func managedNames(text []byte) []string {
	var names []string
	for _, line := range strings.Split(string(text), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, beginPrefix) && strings.HasSuffix(trimmed, beginSuffix) {
			names = append(names, strings.TrimSuffix(strings.TrimPrefix(trimmed, beginPrefix), beginSuffix))
		}
	}
	sort.Strings(names)
	return names
}

func upsertBlock(existing []byte, name, block string) (updated []byte, conflict string) {
	text := string(existing)
	begin, end := beginMarker(name), endMarker(name)
	beginIdx := strings.Index(text, begin)
	endIdx := strings.Index(text, end)
	if beginIdx >= 0 && endIdx > beginIdx {
		endLine := endIdx + len(end)
		if endLine < len(text) && text[endLine] == '\n' {
			endLine++
		}
		return []byte(text[:beginIdx] + block + text[endLine:]), ""
	}
	if clash := foreignHost(text, name); clash != "" {
		return existing, clash
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if text != "" {
		text += "\n"
	}
	return []byte(text + block), ""
}

func removeBlock(existing []byte, name string) []byte {
	text := string(existing)
	begin, end := beginMarker(name), endMarker(name)
	beginIdx := strings.Index(text, begin)
	endIdx := strings.Index(text, end)
	if beginIdx < 0 || endIdx <= beginIdx {
		return existing
	}
	endLine := endIdx + len(end)
	if endLine < len(text) && text[endLine] == '\n' {
		endLine++
	}
	// invariant: an append always writes exactly one blank line before its block
	// (upsertBlock), even after a blank line the user wrote, so removal takes
	// exactly one and an add then remove gives the file back byte for byte.
	before, after := text[:beginIdx], text[endLine:]
	switch {
	case strings.HasSuffix(before, "\n\n"):
		before = before[:len(before)-1]
	case before == "" && strings.HasPrefix(after, "\n"):
		after = after[1:]
	}
	return []byte(before + after)
}

// invariant: any managed block counts, not only this machine's: a neighbour's
// block is not a foreign Host.
func foreignHost(text, name string) string {
	inManaged := false
	for i, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, beginPrefix) && strings.HasSuffix(trimmed, beginSuffix) {
			inManaged = true
			continue
		}
		if strings.HasPrefix(trimmed, endPrefix) && strings.HasSuffix(trimmed, endSuffix) {
			inManaged = false
			continue
		}
		if inManaged || !strings.HasPrefix(trimmed, "Host ") {
			continue
		}
		for _, host := range strings.Fields(trimmed)[1:] {
			if host == name {
				return fmt.Sprintf("Host %s already defined at line %d (outside any managed block)", name, i+1)
			}
		}
	}
	return ""
}
