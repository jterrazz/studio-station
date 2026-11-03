package accounts_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/accounts"
	"github.com/jterrazz/studio-station/src/system"
)

const sentinel = "SENTINELgho_0123456789abcdefghijklmnopqrstuvwxyzAB"

func kept(a accounts.Account) string {
	data, _ := json.Marshal(a)
	return string(data)
}

func TestNoParserKeepsATokenItsToolPrinted(t *testing.T) {
	cases := map[string][]system.Output{
		catalog.ProviderGitHub:     {{Stdout: "github.com\n  Logged in to github.com account jterrazz (keyring)\n  - Git operations protocol: ssh\n  - Token: " + sentinel + "\n  - Token scopes: 'gist', '" + sentinel + "'\n", Stderr: sentinel}},
		catalog.ProviderNPM:        {{Stdout: sentinel + "\n", Stderr: "notice " + sentinel}},
		catalog.ProviderClaude:     {{Stdout: `{"loggedIn":true,"email":"` + sentinel + `","subscriptionType":"max","oauthToken":"` + sentinel + `","authMethod":"claude.ai"}`}},
		catalog.ProviderCloudflare: {{Stdout: "You are logged in with an OAuth Token, associated with the email " + sentinel + ".\nToken: " + sentinel + "\n| " + sentinel + " | 0123456789abcdef0123456789abcdef |\n"}},
		catalog.ProviderGit:        {{Stdout: sentinel + "\n"}, {Stdout: "admin@jterrazz.com\n"}},
		catalog.ProviderSSH:        {{Stdout: "256 SHA256:76MCcfWNZv0Ixcc4fM0b6Ue1o8l7W1Xo0n3zKhK1Vy4 " + sentinel + " (ED25519)\n"}},
		catalog.ProviderDocker:     {{Stdout: `{"https://registry.jterrazz.com":"` + sentinel + `"}`}},
	}
	for provider, outs := range cases {
		got := accounts.Redact(accounts.Parse(provider, accounts.Answer{Outputs: outs}))
		if strings.Contains(kept(got), "SENTINEL") || strings.Contains(kept(got), "gho_") {
			t.Errorf("%s kept a token: %s", provider, kept(got))
		}
		if got.State != catalog.StateOK {
			t.Errorf("%s: the read itself is lost with the token: %s", provider, kept(got))
		}
	}
}

func TestRedactDropsEveryTokenShape(t *testing.T) {
	shapes := []string{
		"ghp_abcdefghij0123456789", "gho_x", "ghs_abc", "ghu_abc", "ghr_abc", "github_pat_11ABCDEF_xyz",
		"sk-ant-api03-abcdef", "npm_abcdefghij0123456789", "xoxb-1234-5678", "xoxp-1-2",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2lnbmF0dXJl",
		"0123456789abcdef0123456789abcdef", "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo0MTIz",
	}
	for _, s := range shapes {
		a := accounts.Redact(accounts.Account{Identity: []string{s}, Scopes: []string{"repo", s}, Detail: s, Notes: []string{s}})
		if a.Identity[0] != accounts.Unknown || a.Scopes[1] != accounts.Unknown || a.Detail != accounts.Unknown || a.Notes[0] != accounts.Unknown || !a.Redacted {
			t.Errorf("%q survived: %+v", s, a)
		}
		if a.Scopes[0] != "repo" {
			t.Errorf("a plain scope beside %q was dropped", s)
		}
	}
	for _, s := range []string{"admin@jterrazz.com", "Jean-Baptiste Terrazzoni", "SHA256:76MCcfWNZv0Ixcc4fM0b6Ue1o8l7W1Xo0n3zKhK1Vy4", "registry.jterrazz.com", "read:org"} {
		if a := accounts.Redact(accounts.Account{Identity: []string{s}}); a.Identity[0] != s || a.Redacted {
			t.Errorf("an identity %q was taken for a token", s)
		}
	}
}
