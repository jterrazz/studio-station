package accounts

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	ReasonUnset       = "unset"
	ReasonMissing     = "missing"
	ReasonRefused     = "refused"
	ReasonLate        = "late"
	ReasonFailing     = "failing"
	ReasonExpiring    = "expiring"
	ReasonUnreachable = "unreachable"

	OriginOwn     = "own"
	OriginLent    = "lent"
	OriginBoth    = "own+lent"
	OriginUnknown = "?"

	Unknown = "?"
)

type Account struct {
	Provider string        `json:"provider"`
	State    catalog.State `json:"state"`
	Reason   string        `json:"reason,omitempty"`
	Identity []string      `json:"identity,omitempty"`
	Origin   string        `json:"origin,omitempty"`
	Scopes   []string      `json:"scopes,omitempty"`
	Source   string        `json:"source,omitempty"`
	Detail   string        `json:"detail,omitempty"`
	Notes    []string      `json:"notes,omitempty"`
	Hint     string        `json:"hint,omitempty"`
	Lent     *Lend         `json:"lent,omitempty"`
	Redacted bool          `json:"-"`
}

type Lend struct {
	Name  string    `json:"name"`
	At    time.Time `json:"at"`
	Until time.Time `json:"until"`
}

type Answer struct {
	Outputs []system.Output
	Err     error
	Late    bool
}

// value: the token shapes no kept value may hold: GitHub, Anthropic, npm and
// Slack tokens, a JWT, or 32 and more characters of base64 or hex.
var secretShape = regexp.MustCompile(`gh[opusr]_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-ant-[A-Za-z0-9_-]+|npm_[A-Za-z0-9]+|xox[abpr]-[A-Za-z0-9-]+|eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*|[A-Za-z0-9+/=_-]{32,}`)

// value: a key fingerprint is an identity, kept whole though it is long.
var fingerprintShape = regexp.MustCompile(`^SHA256:[A-Za-z0-9+/]{43}$`)

func Redact(a Account) Account {
	clean := func(v string) string {
		if fingerprintShape.MatchString(v) {
			return v
		}
		if secretShape.MatchString(v) {
			a.Redacted = true
			return Unknown
		}
		return v
	}
	cleanAll := func(vs []string) []string {
		out := make([]string, 0, len(vs))
		for _, v := range vs {
			out = append(out, clean(v))
		}
		return out
	}
	a.Identity, a.Scopes = cleanAll(a.Identity), cleanAll(a.Scopes)
	a.Source, a.Detail, a.Notes = clean(a.Source), clean(a.Detail), cleanAll(a.Notes)
	if a.Lent != nil {
		l := *a.Lent
		l.Name = clean(l.Name)
		a.Lent = &l
	}
	return a
}

func ok(provider string) Account {
	return Account{Provider: provider, State: catalog.StateOK, Origin: OriginOwn}
}

func absent(provider, reason, detail string) Account {
	return Account{Provider: provider, State: catalog.StateAbsent, Reason: reason, Detail: detail}
}

func broken(provider, reason, detail string) Account {
	return Account{Provider: provider, State: catalog.StateBroken, Reason: reason, Detail: detail, Origin: OriginOwn, Identity: []string{Unknown}}
}

func lines(outs ...system.Output) []string {
	all := make([]string, 0, 4*len(outs))
	for _, o := range outs {
		all = append(all, strings.Split(o.Stdout, "\n")...)
		all = append(all, strings.Split(o.Stderr, "\n")...)
	}
	return all
}

var (
	ghAccount  = regexp.MustCompile(`Logged in to github\.com account (\S+) \(([^)]+)\)`)
	ghProtocol = regexp.MustCompile(`Git operations protocol: (\S+)`)
	ghScopes   = regexp.MustCompile(`Token scopes: (.*)$`)
	ghScope    = regexp.MustCompile(`'([^']*)'`)
)

func Parse(provider string, a Answer) Account {
	if a.Late {
		p := find(provider)
		return Account{Provider: provider, State: catalog.StateDegraded, Reason: ReasonLate, Identity: []string{Unknown}, Detail: "no answer in " + strconv.Itoa(int(p.Budget.Seconds())) + "s"}
	}
	if len(a.Outputs) == 0 {
		return absent(provider, ReasonMissing, missingDetail(provider))
	}
	switch provider {
	case catalog.ProviderGitHub:
		return parseGitHub(a.Outputs[0])
	case catalog.ProviderNPM:
		return parseNPM(a.Outputs[0])
	case catalog.ProviderClaude:
		return parseClaude(a.Outputs[0])
	case catalog.ProviderCloudflare:
		return parseWrangler(a.Outputs[0])
	case catalog.ProviderGit:
		return parseGit(a.Outputs)
	case catalog.ProviderSSH:
		return parseSSH(a.Outputs[0])
	case catalog.ProviderDocker:
		return parseDocker(a.Outputs[0])
	}
	return absent(provider, ReasonMissing, "")
}

func missingDetail(provider string) string {
	if p := find(provider); p.Tool != "" {
		return p.Tool + " not on PATH"
	}
	return "not installed"
}

func find(provider string) catalog.Provider {
	for _, p := range catalog.Providers {
		if p.ID == provider {
			return p
		}
	}
	return catalog.Provider{ID: provider}
}

func parseGitHub(out system.Output) Account {
	acc := ok(catalog.ProviderGitHub)
	acc.Source = "gh"
	var login, source, protocol string
	for _, line := range lines(out) {
		if m := ghAccount.FindStringSubmatch(line); m != nil && login == "" {
			login, source = m[1], m[2]
		}
		if m := ghProtocol.FindStringSubmatch(line); m != nil && protocol == "" {
			protocol = m[1]
		}
		if m := ghScopes.FindStringSubmatch(line); m != nil && acc.Scopes == nil {
			for _, s := range ghScope.FindAllStringSubmatch(m[1], -1) {
				acc.Scopes = append(acc.Scopes, s[1])
			}
		}
	}
	if login == "" {
		return absent(catalog.ProviderGitHub, ReasonUnset, "not logged in")
	}
	acc.Identity = []string{login}
	acc.Notes = slices.DeleteFunc([]string{source, protocol}, func(s string) bool { return s == "" })
	return acc
}

func parseNPM(out system.Output) Account {
	switch {
	case strings.Contains(out.Stderr, "E401") || strings.Contains(out.Stderr, "E403"):
		return broken(catalog.ProviderNPM, ReasonRefused, "token refused by registry.npmjs.org")
	case strings.Contains(out.Stderr, "ENEEDAUTH") || out.ExitCode != 0:
		return absent(catalog.ProviderNPM, ReasonUnset, "not logged in")
	}
	login := strings.TrimSpace(strings.SplitN(strings.TrimSpace(out.Stdout), "\n", 2)[0])
	if login == "" {
		return absent(catalog.ProviderNPM, ReasonUnset, "not logged in")
	}
	acc := ok(catalog.ProviderNPM)
	acc.Identity = []string{login}
	return acc
}

func parseClaude(out system.Output) Account {
	var status struct {
		LoggedIn         bool   `json:"loggedIn"`
		AuthMethod       string `json:"authMethod"`
		Email            string `json:"email"`
		SubscriptionType string `json:"subscriptionType"`
	}
	if json.Unmarshal([]byte(out.Stdout), &status) != nil || !status.LoggedIn {
		return absent(catalog.ProviderClaude, ReasonUnset, "not logged in")
	}
	acc := ok(catalog.ProviderClaude)
	acc.Identity = []string{status.Email}
	if status.Email == "" {
		acc.Identity = []string{Unknown}
	}
	if status.SubscriptionType != "" {
		acc.Scopes = []string{status.SubscriptionType}
	}
	acc.Detail = status.AuthMethod
	return acc
}

var (
	wranglerEmail = regexp.MustCompile(`associated with the email (\S+?)\.?\s*$`)
	wranglerRow   = regexp.MustCompile(`^[\x{2502}|]\s*(.+?)\s*[\x{2502}|]\s*[0-9a-f]{32}\s*[\x{2502}|]`)
)

func parseWrangler(out system.Output) Account {
	all := lines(out)
	acc := ok(catalog.ProviderCloudflare)
	acc.Source = "wrangler"
	for _, line := range all {
		if strings.Contains(line, "You are not authenticated") {
			return absent(catalog.ProviderCloudflare, ReasonUnset, "not logged in")
		}
		if m := wranglerEmail.FindStringSubmatch(line); m != nil && acc.Identity == nil {
			acc.Identity = []string{m[1]}
		}
		if m := wranglerRow.FindStringSubmatch(line); m != nil {
			acc.Scopes = append(acc.Scopes, m[1])
		}
	}
	if acc.Identity == nil {
		return absent(catalog.ProviderCloudflare, ReasonUnset, "not logged in")
	}
	return acc
}

func parseGit(outs []system.Output) Account {
	var values []string
	for _, o := range outs {
		if v := strings.TrimSpace(strings.SplitN(o.Stdout, "\n", 2)[0]); o.ExitCode == 0 && v != "" {
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return absent(catalog.ProviderGit, ReasonUnset, "no user.name nor user.email")
	}
	acc := ok(catalog.ProviderGit)
	acc.Identity = values
	return acc
}

func parseSSH(out system.Output) Account {
	switch out.ExitCode {
	case 0:
	case 1:
		return absent(catalog.ProviderSSH, ReasonUnset, "no key in the agent")
	default:
		return absent(catalog.ProviderSSH, ReasonUnset, "no agent")
	}
	var comments []string
	n := 0
	for _, line := range strings.Split(out.Stdout, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.HasPrefix(fields[1], "SHA256:") {
			continue
		}
		n++
		comment := strings.Join(fields[2:], " ")
		if i := strings.LastIndex(comment, " ("); i >= 0 {
			comment = comment[:i]
		}
		if comment != "" && !slices.Contains(comments, comment) {
			comments = append(comments, comment)
		}
	}
	if n == 0 {
		return absent(catalog.ProviderSSH, ReasonUnset, "no key in the agent")
	}
	acc := ok(catalog.ProviderSSH)
	acc.Identity = []string{keys(n)}
	acc.Scopes = comments
	return acc
}

func keys(n int) string {
	if n == 1 {
		return "1 key"
	}
	return strconv.Itoa(n) + " keys"
}

func DockerStore(config []byte) (store string, listed bool) {
	var c struct {
		Auths       map[string]json.RawMessage `json:"auths"`
		CredsStore  string                     `json:"credsStore"`
		CredHelpers map[string]string          `json:"credHelpers"`
	}
	if json.Unmarshal(config, &c) != nil {
		return "", false
	}
	if c.CredsStore == "" {
		for _, h := range c.CredHelpers {
			c.CredsStore = h
			break
		}
	}
	return c.CredsStore, len(c.Auths) > 0 || c.CredsStore != ""
}

func parseDocker(out system.Output) Account {
	var registries map[string]string
	if out.ExitCode != 0 || json.Unmarshal([]byte(out.Stdout), &registries) != nil {
		return Account{Provider: catalog.ProviderDocker, State: catalog.StateBroken, Reason: ReasonFailing, Origin: OriginOwn, Identity: []string{Unknown}, Detail: "the credential helper did not list its registries"}
	}
	if len(registries) == 0 {
		return absent(catalog.ProviderDocker, ReasonUnset, "no registry login")
	}
	names := make([]string, 0, len(registries))
	for r := range registries {
		names = append(names, r)
	}
	slices.Sort(names)
	acc := ok(catalog.ProviderDocker)
	for _, r := range names {
		host := strings.TrimPrefix(strings.TrimPrefix(r, "https://"), "http://")
		host, _, _ = strings.Cut(host, "/")
		if !slices.Contains(acc.Identity, registries[r]) {
			acc.Identity = append(acc.Identity, registries[r])
		}
		acc.Scopes = append(acc.Scopes, host)
	}
	return acc
}

func Fingerprint(line string) (fingerprint, comment string, ok bool) {
	fields := strings.Fields(line)
	for i, f := range fields {
		blob, err := base64.StdEncoding.DecodeString(f)
		if err != nil || len(blob) < 16 || i == 0 && !strings.HasPrefix(f, "AAAA") {
			continue
		}
		return "SHA256:" + strings.TrimRight(base64.StdEncoding.EncodeToString(digest(blob)), "="), strings.Join(fields[i+1:], " "), true
	}
	return "", "", false
}

func digest(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
}
