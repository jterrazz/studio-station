package catalog

type ConfigCategory string

const (
	CategoryShell    ConfigCategory = "Shell"
	CategoryTerminal ConfigCategory = "Terminal"
	CategorySecurity ConfigCategory = "Security"
	CategoryEditor   ConfigCategory = "Editor"
	CategorySystem   ConfigCategory = "System"
	CategoryHeadless ConfigCategory = "Headless"
	CategoryNetwork  ConfigCategory = "Network"
)

type InputKind int

const (
	InputText InputKind = iota
	InputPassword
	InputSelect
	InputConfirm
)

type Input struct {
	Name, Label, Help string
	Kind              InputKind
	Options           []string
	DefaultEnv        string
	Required          bool
}

type Effect interface{ isEffect() }

type Touch struct{ Path string }

type Dotfiles struct {
	Files  []Copy
	Reload []string
}

type Copy struct{ From, To string }

type Procedure string

const (
	ProcGitSigning     Procedure = "git-signing"
	ProcSSHKey         Procedure = "ssh-key"
	ProcGitHubLogin    Procedure = "github-login"
	ProcSpotlight      Procedure = "spotlight-exclude"
	ProcDNS            Procedure = "dns"
	ProcDockReset      Procedure = "dock-reset"
	ProcDockSpacer     Procedure = "dock-spacer"
	ProcAutologin      Procedure = "autologin"
	ProcPower          Procedure = "power"
	ProcLockAfterLogin Procedure = "lock-after-login"
	ProcSshd           Procedure = "sshd"
	ProcTailnetAuth    Procedure = "tailnet-auth"
	ProcVMJoin         Procedure = "vm-join"
	ProcClaudeToken    Procedure = "claude-token"
)

func (Touch) isEffect()     {}
func (Dotfiles) isEffect()  {}
func (Procedure) isEffect() {}

type ConfigItem struct {
	Name, Description, Help string
	Category                ConfigCategory
	Places                  []MachineKind
	Requires                string
	Inputs                  []Input
	Effect                  Effect
	Interactive             bool
	NeedsRoot               bool
	Reversible              bool
	RunOnce                 bool
}

var (
	everyKind = []MachineKind{Host, Linux, MacOS}
	macKinds  = []MachineKind{Host, MacOS}
	hostKind  = []MachineKind{Host}
)

var ConfigItems = []ConfigItem{
	{Name: "hushlogin", Description: "Silence the terminal login message", Category: CategoryTerminal, Places: everyKind, Effect: Touch{Path: "~/.hushlogin"}},
	{Name: "ghostty-dotfiles", Description: "The Ghostty config and its Catppuccin themes", Category: CategoryTerminal, Places: macKinds, Requires: "ghostty", Effect: Dotfiles{Files: []Copy{
		{From: "ghostty/config", To: "~/.config/ghostty/config"},
		{From: "ghostty/themes/catppuccin-espresso-blur", To: "~/.config/ghostty/themes/catppuccin-espresso-blur"},
		{From: "ghostty/themes/catppuccin-latte-blur", To: "~/.config/ghostty/themes/catppuccin-latte-blur"},
	}}},
	{Name: "tmux-dotfiles", Description: "The tmux config", Category: CategoryTerminal, Places: everyKind, Requires: "tmux", Effect: Dotfiles{
		Files:  []Copy{{From: "tmux/tmux.conf", To: "~/.tmux.conf"}},
		Reload: []string{"tmux", "source-file", "~/.tmux.conf"},
	}},
	{Name: "starship-dotfiles", Description: "The Starship prompt config", Category: CategoryTerminal, Places: everyKind, Requires: "starship", Effect: Dotfiles{Files: []Copy{{From: "starship/starship.toml", To: "~/.config/starship.toml"}}}},
	{Name: "git-signing", Description: "Sign every commit with a GPG key", Category: CategorySecurity, Places: macKinds, Requires: "gpg", Effect: ProcGitSigning, Interactive: true},
	{Name: "ssh-key", Description: "An ed25519 SSH key, kept in the Keychain", Category: CategorySecurity, Places: hostKind, Effect: ProcSSHKey, Interactive: true},
	{Name: "github-login", Description: "The GitHub CLI logged in over SSH", Category: CategorySecurity, Places: everyKind, Requires: "gh", Effect: ProcGitHubLogin, Interactive: true},
	{Name: "spotlight-exclude", Description: "Exclude ~/Developer from Spotlight (guided, opens Settings)", Category: CategorySecurity, Places: macKinds, Effect: ProcSpotlight},
	{Name: "dns", Description: "Encrypted DNS via Quad9 (DoH)", Category: CategorySecurity, Places: hostKind, Effect: ProcDNS},
	{Name: "claude-token", Description: "The Claude Code subscription token a VM may borrow", Help: "run `claude setup-token` and paste the token", Category: CategorySecurity, Places: hostKind, Effect: ProcClaudeToken, Reversible: true, Inputs: []Input{
		{Name: "token", Label: "Claude Code token", Help: "run `claude setup-token` and paste the token", Kind: InputPassword, Required: true},
	}},
	{Name: "zed-dotfiles", Description: "The Zed settings and its formatter config", Category: CategoryEditor, Places: macKinds, Requires: "zed", Effect: Dotfiles{Files: []Copy{
		{From: "zed/settings.json", To: "~/.config/zed/settings.json"},
		{From: "zed/.oxfmtrc.json", To: "~/.config/zed/.oxfmtrc.json"},
	}}},
	{Name: "dock-reset", Description: "Reset the dock to the system defaults", Category: CategorySystem, Places: macKinds, Effect: ProcDockReset, RunOnce: true},
	{Name: "dock-spacer", Description: "Add a small spacer tile to the dock", Category: CategorySystem, Places: macKinds, Effect: ProcDockSpacer, RunOnce: true},
	{Name: "autologin", Description: "GUI auto-login for jterrazz.agent (FileVault-aware)", Category: CategoryHeadless, Places: hostKind, Effect: ProcAutologin, Interactive: true, NeedsRoot: true, Reversible: true, Inputs: []Input{
		{Name: "password", Label: "Password of jterrazz.agent", Kind: InputPassword, DefaultEnv: "AGENT_PASSWORD", Required: true},
	}},
	{Name: "power", Description: "Always-on power policy (no sleep, autorestart, wake)", Category: CategoryHeadless, Places: hostKind, Effect: ProcPower, NeedsRoot: true, Reversible: true},
	{Name: "lock-after-login", Description: "A LaunchAgent that locks the screen ~20s after auto-login", Category: CategoryHeadless, Places: hostKind, Effect: ProcLockAfterLogin, NeedsRoot: true, Reversible: true},
	{Name: "sshd", Description: "macOS Remote Login (sshd)", Category: CategoryHeadless, Places: hostKind, Effect: ProcSshd, NeedsRoot: true, Reversible: true},
	{Name: "tailnet-auth", Description: "How this host joins the tailnet: the daemon mode and the login", Category: CategoryNetwork, Places: hostKind, Effect: ProcTailnetAuth, Reversible: true, Inputs: []Input{
		{Name: "mode", Label: "Daemon", Kind: InputSelect, Options: []string{"auto", "userspace"}, Required: true},
		{Name: "method", Label: "Login", Kind: InputSelect, Options: []string{"login", "authkey"}, Required: true},
		{Name: "authkey", Label: "Auth key", Help: "required when the login is authkey", Kind: InputPassword},
	}},
	{Name: "vm-join", Description: "The OAuth client that mints a tailnet key for each new VM", Category: CategoryNetwork, Places: hostKind, Effect: ProcVMJoin, Reversible: true, Inputs: []Input{
		{Name: "client-id", Label: "OAuth client id", Kind: InputText, Required: true},
		{Name: "client-secret", Label: "OAuth client secret", Kind: InputPassword, Required: true},
	}},
}
