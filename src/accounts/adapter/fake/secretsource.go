package fake

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/world"
)

type SecretSource struct {
	w   *world.World
	mac string
}

var _ contract.SecretSource = (*SecretSource)(nil)

func NewSecretSource(w *world.World, mac string) *SecretSource {
	return &SecretSource{w: w, mac: mac}
}

func (s *SecretSource) Export(_ context.Context, names []string) ([]byte, error) {
	kept := mac(s.w, s.mac).Secrets
	out := make(map[string]string, len(names))
	for _, n := range names {
		v, ok := kept[n]
		if !ok {
			return nil, errors.New(n + " is not kept")
		}
		out[n] = v
	}
	return json.Marshal(out)
}
