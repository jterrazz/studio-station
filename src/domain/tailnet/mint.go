package tailnet

import (
	"context"
	"encoding/json"
	"strings"
	"time"
)

const (
	vmTag         = "tag:station-vm"
	defaultTTL    = 10 * time.Minute
	oauthEndpoint = "/api/v2/oauth/token"
	keysPath      = "/api/v2/tailnet/-/keys"
	devicesPath   = "/api/v2/device/"
)

type MintRequest struct {
	Tags   []string
	Expiry time.Duration
}

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
	form := "grant_type=client_credentials&client_id=" + escape(id) + "&client_secret=" + escape(secret)
	status, body, err := s.h.Net.Post(ctx, s.o.APIURL+oauthEndpoint,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, []byte(form))
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

func (s *Service) Mint(ctx context.Context, r MintRequest) (AuthKey, error) {
	// invariant: nodes are never ephemeral; an ephemeral node can vanish while a
	// temporary VM is merely paused, so station removes every node itself.
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

func (s *Service) Remove(ctx context.Context, nodeID string) error {
	token, err := s.token(ctx)
	if err != nil {
		return err
	}
	status, _, err := s.h.Net.Delete(ctx, s.o.APIURL+devicesPath+escape(nodeID), bearer(token))
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

// value: percent-encoding everything but RFC 3986's unreserved characters is
// right for a form value and for a path segment alike.
func escape(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&15])
	}
	return b.String()
}
