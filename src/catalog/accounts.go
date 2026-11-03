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
	Host, VM  bool
	Tool      string
	Reads     []ProviderRead
	Budget    time.Duration
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
	{ID: ProviderGitHub, Label: "GitHub", Host: true, VM: true, Tool: "gh", Reads: []ProviderRead{{Name: "gh", Args: []string{"auth", "status", "--hostname", "github.com"}}}, Budget: 5 * time.Second},
	{ID: ProviderNPM, Label: "npm", Host: true, VM: true, Tool: "npm", Reads: []ProviderRead{{Name: "npm", Args: []string{"whoami"}}}, Budget: 5 * time.Second},
	{ID: ProviderClaude, Label: "Claude", Host: true, VM: true, Tool: "claude", Reads: []ProviderRead{{Name: "claude", Args: []string{"auth", "status"}}}, Budget: 5 * time.Second},
	{ID: ProviderTailscale, Label: "Tailscale", Host: true},
	{ID: ProviderCloudflare, Label: "Cloudflare", Host: true, VM: true, Tool: "wrangler", Reads: []ProviderRead{{Name: "wrangler", Args: []string{"whoami"}}}, Budget: 10 * time.Second},
	{ID: ProviderGit, Label: "git", Host: true, VM: true, Tool: "git", Reads: []ProviderRead{
		{Name: "git", Args: []string{"config", "--global", "user.name"}},
		{Name: "git", Args: []string{"config", "--global", "user.email"}},
	}, Budget: 2 * time.Second},
	{ID: ProviderSSH, Label: "ssh", Host: true, VM: true, Tool: "ssh-add", Reads: []ProviderRead{{Name: "ssh-add", Args: []string{"-l"}}}, Budget: 2 * time.Second},
	{ID: ProviderDocker, Label: "Docker", Host: true, VM: true, Budget: 5 * time.Second},
	{ID: ProviderEntry, Label: "entry", VM: true},
}

type LendScope struct {
	// value: a lend-list name, the provider whose right its copy carries,
	// and what that right lets the VM do.
	Name, Provider, Scope string
}

const ClaudeTokenName = "operator:claude:oauth-token"

var LendScopes = []LendScope{
	{Name: ClaudeTokenName, Provider: ProviderClaude, Scope: "Claude Code inference"},
	{Name: "session:github", Provider: ProviderGitHub, Scope: "GitHub browser session"},
}

const DockerConfig = ".docker/config.json"
