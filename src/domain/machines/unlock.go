package machines

import (
	"context"
	"errors"

	"github.com/jterrazz/studio-station/src/machines/contract"
)

func (s *Service) Unlock(ctx context.Context, name string) error {
	if err := s.CheckHost(); err != nil {
		return err
	}
	m, err := s.RemoteHost(name, "unlock")
	if err != nil {
		return err
	}
	if err := s.o.Unlocker.Unlock(ctx, m.Name); errors.Is(err, contract.ErrUnreachable) {
		return &UnreachableError{Name: name}
	} else if err != nil {
		return err
	}
	return nil
}
