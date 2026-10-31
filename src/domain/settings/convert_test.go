package settings_test

import (
	"reflect"
	"testing"

	"github.com/jterrazz/studio-station/src/domain/settings"
)

func TestConvertV1KeepsTheRegistryAndDropsRolesAndTheSecret(t *testing.T) {
	v1 := `{
  "remote": {"mode": "userspace", "auth_method": "oauth", "secret": "client-secret", "hostname": "mbp"},
  "user_email": "a@b.c",
  "self": "macbook",
  "machines": {
    "macbook": {"role": "client"},
    "mac-mini": {"role": "server", "ssh": "jterrazz.agent@mac-mini", "identity": "~/.ssh/id_ed25519"}
  }
}`
	got, key, err := settings.ConvertV1([]byte(v1), "jterrazz")
	if err != nil {
		t.Fatal(err)
	}
	want := settings.Settings{
		Here: "macbook",
		Hosts: map[string]settings.Host{
			"macbook":  {},
			"mac-mini": {User: "jterrazz.agent", Key: "~/.ssh/id_ed25519"},
		},
		Tailnet:   settings.Tailnet{Mode: settings.ModeUserspace, AuthMethod: settings.AuthLogin},
		VMUser:    "jterrazz",
		UserEmail: "a@b.c",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("converted\n%+v\nwant\n%+v", got, want)
	}
	if key != "" {
		t.Fatalf("an oauth secret is no auth key, got %q", key)
	}
}

func TestConvertV1HandsAnAuthKeyToTheKeychainAndNamesHereFromTheHostname(t *testing.T) {
	v1 := `{"remote": {"mode": "auto", "auth_method": "authkey", "secret": "tskey-auth-x", "hostname": "mac-mini"}}`
	got, key, err := settings.ConvertV1([]byte(v1), "agent")
	if err != nil {
		t.Fatal(err)
	}
	if key != "tskey-auth-x" {
		t.Fatalf("auth key %q", key)
	}
	if got.Here != "mac-mini" || got.Tailnet.Mode != settings.ModeAuto || got.Tailnet.AuthMethod != settings.AuthAuthKey {
		t.Fatalf("converted %+v", got)
	}
	if _, ok := got.Hosts["mac-mini"]; !ok {
		t.Fatalf("here is not a host: %+v", got.Hosts)
	}
}

func TestConvertV1RefusesWhatIsNotJSON(t *testing.T) {
	if _, _, err := settings.ConvertV1([]byte("{"), "u"); err == nil {
		t.Fatal("a truncated v1 converted")
	}
}
