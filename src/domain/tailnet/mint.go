package tailnet

import (
	"context"
	"encoding/json"
	"net/url"
	"time"
)

const (
	vmTag       = "tag:station-vm"
	defaultTTL  = 10 * time.Minute
	tokenPath   = "/api/v2/oauth/token"
	keysPath    = "/api/v2/tailnet/-/keys"
	devicesPath = "/api/v2/device/"
)

type MintRequest struct {
	Tags   []string
	Expiry time.Duration
}

// AuthKey hides its value from every formatter and encoder: only Reveal
// returns it, and only the first-boot rendering calls that.
type AuthKey struct{ value string }

func (k AuthKey) Reveal() string { return k.value }

func (AuthKey) String() string { return "tskey-…" }

func (AuthKey) GoString() string { return "tskey-…" }

func (AuthKey) MarshalJSON() ([]byte, error) { return []byte(`"tskey-…"`), nil }

func (s *Service) token(ctx context.Context) (string, error) {
	id, foundID, err := s.keys.Read(ctx, joinIDAccount)
	if err != nil {
		return "", err
	}
	secret, foundSecret, err := s.keys.Read(ctx, joinSecretAccount)
	if err != nil {
		return "", err
	}
	if !foundID || !foundSecret {
		return "", &NoVMJoinError{}
	}
	form := url.Values{"client_id": {id}, "client_secret": {secret}, "grant_type": {"client_credentials"}}
	status, body, err := s.h.Net.Post(ctx, s.o.APIURL+tokenPath,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, []byte(form.Encode()))
	if err != nil {
		return "", err
	}
	var reply struct {
		AccessToken string `json:"access_token"`
	}
	if status/100 != 2 || json.Unmarshal(body, &reply) != nil || reply.AccessToken == "" {
		return "", &MintRefusedError{Status: status, Detail: "the OAuth client was refused"}
	}
	return reply.AccessToken, nil
}

// Mint asks for a single-use key for one new VM. Nodes are never ephemeral:
// an ephemeral node can vanish while a temporary VM is merely paused, so
// station removes every node itself.
func (s *Service) Mint(ctx context.Context, r MintRequest) (AuthKey, error) {
	token, err := s.token(ctx)
	if err != nil {
		return AuthKey{}, err
	}
	tags := r.Tags
	if len(tags) == 0 {
		tags = []string{vmTag}
	}
	expiry := r.Expiry
	if expiry == 0 {
		expiry = defaultTTL
	}
	request := map[string]any{
		"capabilities": map[string]any{"devices": map[string]any{"create": map[string]any{
			"reusable": false, "ephemeral": false, "preauthorized": true, "tags": tags,
		}}},
		"expirySeconds": int(expiry / time.Second),
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return AuthKey{}, err
	}
	status, body, err := s.h.Net.Post(ctx, s.o.APIURL+keysPath, bearer(token), payload)
	if err != nil {
		return AuthKey{}, err
	}
	var reply struct {
		Key     string `json:"key"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(body, &reply)
	if status/100 != 2 || reply.Key == "" {
		return AuthKey{}, &MintRefusedError{Status: status, Detail: reply.Message}
	}
	return AuthKey{value: reply.Key}, nil
}

// Remove deletes a node through the API; a node that is already gone counts
// as removed.
func (s *Service) Remove(ctx context.Context, nodeID string) error {
	token, err := s.token(ctx)
	if err != nil {
		return err
	}
	status, _, err := s.h.Net.Delete(ctx, s.o.APIURL+devicesPath+url.PathEscape(nodeID), bearer(token))
	if err != nil {
		return err
	}
	if status/100 != 2 && status != 404 {
		return &RemoveRefusedError{NodeID: nodeID, Status: status}
	}
	return nil
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/json"}
}
