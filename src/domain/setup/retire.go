package setup

import (
	"context"
	"errors"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	backgroundNote = "the userspace tailscaled stops in the background"
	nodeConsole    = "https://login.tailscale.com/admin/machines"
	oauthConsole   = "https://login.tailscale.com/admin/settings/oauth"
)

// value: what the round-2 userspace tailscaled left in its folder, by name: the
// retirement removes exactly these, then the folder if it is empty.
func tailscaledFiles() []string {
	return []string{"tailscaled.state", "tailscaled.log", "tailscaled.sock", "tailscaled.pid", "caffeinate.pid", "caffeinate.log"}
}

func legacyNodeState() []string { return []string{legacyTailscale, tailscaledFiles()[0]} }

func retiredAccounts() []string { return []string{"vm-join-id", "vm-join-secret", "tailnet-authkey"} }

func (s *Service) tailscaledDirs() []string {
	return []string{path.Join(s.o.State, "tailscaled"), s.legacyPath(legacyTailscale)}
}

func (s *Service) livePIDs() []int {
	var pids []int
	for _, dir := range s.tailscaledDirs() {
		for _, name := range []string{"tailscaled.pid", "caffeinate.pid"} {
			data, err := s.h.FS.ReadFile(path.Join(dir, name))
			if err != nil {
				continue
			}
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && pid > 0 && s.h.Run.Alive(pid) {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

// invariant: the userspace era ends in three acts a rerun skips when done: its
// running daemon stops (detached, since a run arriving over that very node would
// cut its own session), its named files go, and its keychain accounts go; the old
// node stays in the admin console until the owner removes it, which the row says.
func (s *Service) retire(ctx context.Context) (rows []Step, background bool, err error) {
	if pids := s.livePIDs(); len(pids) > 0 {
		if !s.o.Detached {
			if err := s.detach(); err != nil {
				return nil, false, err
			}
			return []Step{{Name: stepLayout, State: catalog.StateDegraded, Detail: backgroundNote, Log: s.logPath(), Changed: true}}, true, nil
		}
		for _, pid := range pids {
			if err := s.stop(ctx, pid); err != nil {
				return nil, false, err
			}
		}
	}
	removed := false
	for _, dir := range s.tailscaledDirs() {
		for _, name := range tailscaledFiles() {
			removed = s.drop(path.Join(dir, name)) || removed
		}
		removed = s.drop(dir) || removed
	}
	if removed {
		here, _ := s.reg.Here()
		rows = append(rows, Step{Name: stepLayout, State: catalog.StateOK, Changed: true,
			Detail: "userspace tailscaled retired; its node \"" + here + "\" stays at " + nodeConsole + " until you remove it"})
	}
	left := false
	for _, account := range retiredAccounts() {
		switch err := s.vault.Forget(ctx, account); {
		case err == nil:
			left = true
		case errors.Is(err, accounts.ErrNotKept), errors.Is(err, system.ErrNotFound):
		default:
			return nil, false, err
		}
	}
	if left {
		rows = append(rows, Step{Name: stepLayout, State: catalog.StateOK, Changed: true,
			Detail: "the VM-join client and the tailnet auth key left the keychain; revoke the OAuth client at " + oauthConsole})
	}
	return rows, false, s.migrateStores(ctx)
}

func (s *Service) migrateStores(ctx context.Context) error {
	for _, m := range s.p.Stores {
		if err := m.Migrate(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) stop(ctx context.Context, pid int) error {
	if err := s.h.Run.Signal(pid, system.SigTerm); err != nil {
		return err
	}
	for range 20 {
		if !s.h.Run.Alive(pid) {
			return nil
		}
		if err := s.h.Clock.Sleep(ctx, 250*time.Millisecond); err != nil {
			return err
		}
	}
	return s.h.Run.Signal(pid, system.SigKill)
}
