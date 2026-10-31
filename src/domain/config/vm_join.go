package config

import (
	"context"

	"github.com/jterrazz/studio-station/src/catalog"
)

const (
	vmJoinID     = "vm-join-id"
	vmJoinSecret = "vm-join-secret"
)

func (s *Service) checkVMJoin(ctx context.Context) (state catalog.State, detail string) {
	found := 0
	for _, account := range []string{vmJoinID, vmJoinSecret} {
		ok, err := s.kept(ctx, account)
		if err != nil {
			return catalog.StateUnknown, firstLine(err.Error())
		}
		if ok {
			found++
		}
	}
	switch found {
	case 2:
		return catalog.StateOK, "OAuth client kept in the keychain"
	case 0:
		return catalog.StateAbsent, ""
	}
	return catalog.StateDegraded, "one of the two keychain accounts is missing"
}

func (s *Service) installVMJoin(ctx context.Context, values map[string]string) (Change, error) {
	var change Change
	for _, kv := range [][2]string{{vmJoinID, values["client-id"]}, {vmJoinSecret, values["client-secret"]}} {
		if err := s.kc.Write(ctx, kv[0], kv[1]); err != nil {
			return change, err
		}
		change.Steps = append(change.Steps, okStep(kv[0], "kept in the keychain"))
	}
	return change, nil
}

func (s *Service) uninstallVMJoin(ctx context.Context) (Change, error) {
	var change Change
	for _, account := range []string{vmJoinID, vmJoinSecret} {
		if err := s.kc.Remove(ctx, account); err != nil {
			return change, err
		}
		change.Steps = append(change.Steps, okStep(account, "removed from the keychain"))
	}
	return change, nil
}
