package catalog

import "time"

type ProviderRead struct {
	Name string
	Args []string
}

type Provider struct {
	// value: a provider of rights the Accounts page shows: where it is read,
	// the commands that read it (never one that prints a secret), and the
	// budget the whole read gets.
	ID, Label string
	Mac, VM   bool
	Command   string
	Reads     []ProviderRead
	Budget    time.Duration
	// value: where the owner ends a right station did not lend: shown on a
	// revoke of an own row, never run.
	Logout string
}

const (
	ProviderGitHub     = "github"
	ProviderNPM        = "npm"
	ProviderClaude     = "claude"
	ProviderTailscale  = "tailscale"
	ProviderCloudflare = "cloudflare"
	ProviderGit        = "git"
	ProviderSSH        = "ssh"
	ProviderDocker     = "docker"
	ProviderEntry      = "entry"
)

var Providers = []Provider{
	{ID: ProviderGitHub, Label: "GitHub", Logout: "gh auth logout", Mac: true, VM: true, Command: "gh", Reads: []ProviderRead{{Name: "gh", Args: []string{"auth", "status", "--hostname", "github.com"}}}, Budget: 5 * time.Second},
	{ID: ProviderNPM, Label: "npm", Logout: "npm logout", Mac: true, VM: true, Command: "npm", Reads: []ProviderRead{{Name: "npm", Args: []string{"whoami"}}}, Budget: 5 * time.Second},
	{ID: ProviderClaude, Label: "Claude", Logout: "claude /logout", Mac: true, VM: true, Command: "claude", Reads: []ProviderRead{{Name: "claude", Args: []string{"auth", "status"}}}, Budget: 5 * time.Second},
	{ID: ProviderTailscale, Label: "Tailscale", Logout: "tailscale logout", Mac: true},
	{ID: ProviderCloudflare, Label: "Cloudflare", Logout: "wrangler logout", Mac: true, VM: true, Command: "wrangler", Reads: []ProviderRead{{Name: "wrangler", Args: []string{"whoami"}}}, Budget: 10 * time.Second},
	{ID: ProviderGit, Label: "git", Mac: true, VM: true, Command: "git", Reads: []ProviderRead{
		{Name: "git", Args: []string{"config", "--global", "user.name"}},
		{Name: "git", Args: []string{"config", "--global", "user.email"}},
	}, Budget: 2 * time.Second},
	{ID: ProviderSSH, Label: "ssh", Logout: "ssh-add -D", Mac: true, VM: true, Command: "ssh-add", Reads: []ProviderRead{{Name: "ssh-add", Args: []string{"-l"}}}, Budget: 2 * time.Second},
	{ID: ProviderDocker, Label: "Docker", Logout: "docker logout", Mac: true, VM: true, Budget: 5 * time.Second},
	{ID: ProviderEntry, Label: "entry", VM: true},
}

const LeakShapes = `gh[opusr]_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-ant-[A-Za-z0-9_-]+|npm_[A-Za-z0-9]+|xox[abpr]-[A-Za-z0-9-]+|eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*|[A-Za-z0-9+/=_-]{32,}` // value: the token shapes no kept value may hold: GitHub, Anthropic, npm and Slack tokens, a JWT, or 32 and more characters of base64 or hex.

type ShareScope struct {
	// value: a lend-list name, the provider whose right its copy carries,
	// and what that right lets the VM do.
	Name, Provider, Scope string
}

const ClaudeTokenName = "operator:claude:oauth-token"

var ShareScopes = []ShareScope{
	{Name: ClaudeTokenName, Provider: ProviderClaude, Scope: "Claude Code inference"},
	{Name: "session:github", Provider: ProviderGitHub, Scope: "GitHub browser session"},
}

const DockerSettingsFile = ".docker/config.json"
