package catalog

import "io/fs"

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

type Effect interface{ Kind() EffectKind }

type EffectKind string

const (
	EffectTouch   EffectKind = "touch"
	EffectDotfile EffectKind = "dotfile"
)

type Touch struct{ Path string }

type Dotfiles struct {
	Files  []Copy
	Reload []string
}

type Copy struct {
	From, To string
	Mode     fs.FileMode
}

type Procedure string

const (
	ProcGitSigning       Procedure = "git-signing"
	ProcSSHKey           Procedure = "ssh-key"
	ProcGitHubLogin      Procedure = "github-login"
	ProcSpotlight        Procedure = "spotlight-exclude"
	ProcDNS              Procedure = "dns"
	ProcDockReset        Procedure = "dock-reset"
	ProcDockSpacer       Procedure = "dock-spacer"
	ProcAutologin        Procedure = "autologin"
	ProcPower            Procedure = "power"
	ProcLockAfterLogin   Procedure = "lock-after-login"
	ProcSshd             Procedure = "sshd"
	ProcClaudeToken      Procedure = "claude-token"
	ProcBackupExclusions Procedure = "backup-exclusions"
)

func (Touch) Kind() EffectKind    { return EffectTouch }
func (Dotfiles) Kind() EffectKind { return EffectDotfile }

func (p Procedure) Kind() EffectKind { return EffectKind("procedure:" + string(p)) }

type ConfigItem struct {
	// value: a procedure is its own kind of effect: one Applier each.
	Name, Description, Help string
	Category                ConfigCategory
	Places                  []MachineKind
	Requires                string
	Inputs                  []Input
	Effect                  Effect
	Interactive             bool
	// value: it opens System Settings for the owner to finish it.
	Guided     bool
	NeedsRoot  bool
	Reversible bool
	RunOnce    bool
	Writes     []Stored
	// value: a setting that writes no file: the values station sets, as
	// its Applier reads them back.
	Sets []Value
}

func (it ConfigItem) AlwaysOn() bool {
	// invariant: Always-on Mac changes how a Mac sleeps, logs in and is
	// reached, so it is applied by name only, never by setup.
	return it.Category == CategoryHeadless
}

func (it ConfigItem) NeedsOwner() bool {
	// invariant: a value, admin rights, the owner at System Settings, or no
	// trace to tell it was applied: setup lists it as a next step.
	return len(it.Inputs) > 0 || it.Interactive || it.Guided || it.NeedsRoot || it.RunOnce
}

type Value struct{ Key, Value string }

var (
	everyKind = []MachineKind{Host, Linux, MacOS}
	macKinds  = []MachineKind{Host, MacOS}
	hostKind  = []MachineKind{Host}
)

var ConfigItems = []ConfigItem{
	{Name: "hushlogin", Description: "Silence the terminal login message", Category: CategoryTerminal, Places: everyKind, Effect: Touch{Path: "~/.hushlogin"}, Writes: []Stored{conf("~/.hushlogin")}},
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
	{Name: "git-signing", Description: "Sign every commit with a GPG key", Category: CategorySecurity, Places: macKinds, Requires: "gpg", Effect: ProcGitSigning, Sets: []Value{{"commit.gpgsign", "true"}, {"gpg.program", "gpg"}}, Interactive: true, Writes: []Stored{kept("~/.gnupg"), noted(conf("~/.gitconfig"), "`git config --global` user.signingkey, commit.gpgsign")}},
	{Name: "ssh-key", Description: "An ed25519 SSH key, kept in the Keychain", Category: CategorySecurity, Places: hostKind, Effect: ProcSSHKey, Sets: []Value{{"~/.ssh/id_ed25519", "present"}, {"~/.ssh/config", "the Keychain stanza"}}, Interactive: true, Writes: []Stored{kept("~/.ssh/id_ed25519"), kept("~/.ssh/id_ed25519.pub"), noted(conf("~/.ssh/config"), "the `Host *` stanza: `UseKeychain yes`, `IdentityFile ~/.ssh/id_ed25519`")}},
	{Name: "github-login", Description: "The GitHub CLI logged in over SSH", Category: CategorySecurity, Places: everyKind, Requires: "gh", Effect: ProcGitHubLogin, Sets: []Value{{"gh", "logged in"}}, Interactive: true, Writes: []Stored{noted(kept("{config}/gh/hosts.yml"), "gh's login")}},
	{Name: "spotlight-exclude", Description: "Exclude ~/Developer from Spotlight (guided, opens Settings)", Category: CategorySecurity, Places: macKinds, Effect: ProcSpotlight, Sets: []Value{{"~/Developer", "excluded from Spotlight"}}, Guided: true, Writes: []Stored{noted(conf("/System/Volumes/Data/.Spotlight-V100"), "the Spotlight privacy list, through System Settings")}},
	{Name: "dns", Description: "Encrypted DNS via Quad9 (DoH)", Category: CategorySecurity, Places: hostKind, Effect: ProcDNS, Sets: []Value{{"encrypted DNS", "Quad9 DoH"}}, Guided: true, Writes: []Stored{noted(conf("{state}/station/dns/quad9-dns.mobileconfig"), "installed as the profile `com.jterrazz.dns.quad9`")}},
	{Name: "claude-token", Description: "The Claude Code subscription token a VM may borrow", Help: "run `claude setup-token` and paste the token", Category: CategorySecurity, Places: hostKind, Effect: ProcClaudeToken, Sets: []Value{{"token", "kept by the studio credential store"}, {"share list", "holds it"}}, Reversible: true, Inputs: []Input{
		{Name: "token", Label: "Claude Code token", Help: "run `claude setup-token` and paste the token", Kind: InputPassword, Required: true},
	}, Writes: []Stored{noted(kept("{config}/studio-os"), "`operator:claude:oauth-token` in the OS credential store")}},
	{Name: "zed-dotfiles", Description: "The Zed settings and its formatter config", Category: CategoryEditor, Places: macKinds, Requires: "zed", Effect: Dotfiles{Files: []Copy{
		{From: "zed/settings.json", To: "~/.config/zed/settings.json"},
		{From: "zed/.oxfmtrc.json", To: "~/.config/zed/.oxfmtrc.json"},
	}}},
	{Name: "dock-reset", Description: "Reset the dock to the system defaults", Category: CategorySystem, Places: macKinds, Effect: ProcDockReset, Sets: []Value{{"Dock", "reset to the system defaults"}}, RunOnce: true, Writes: []Stored{noted(conf("~/Library/Preferences/com.apple.dock.plist"), "`com.apple.dock` defaults")}},
	{Name: "dock-spacer", Description: "Add a small spacer tile to the dock", Category: CategorySystem, Places: macKinds, Effect: ProcDockSpacer, Sets: []Value{{"Dock", "spacer tile added"}}, RunOnce: true, Writes: []Stored{noted(conf("~/Library/Preferences/com.apple.dock.plist"), "`com.apple.dock` defaults: a spacer tile")}},
	{Name: "autologin", Description: "GUI auto-login for jterrazz.agent (FileVault-aware)", Category: CategoryHeadless, Places: hostKind, Effect: ProcAutologin, Sets: []Value{{"autoLoginUser", "jterrazz.agent"}, {"/etc/kcpassword", "present"}}, Interactive: true, NeedsRoot: true, Reversible: true, Inputs: []Input{
		{Name: "password", Label: "Password of jterrazz.agent", Kind: InputPassword, DefaultEnv: "AGENT_PASSWORD", Required: true},
	}, Writes: []Stored{noted(kept("/etc/kcpassword"), "the agent user's login password, obfuscated"), noted(conf("/Library/Preferences/com.apple.loginwindow"), "`autoLoginUser` in `com.apple.loginwindow`")}},
	{Name: "power", Description: "Always-on power policy (no sleep, autorestart, wake)", Category: CategoryHeadless, Places: hostKind, Effect: ProcPower, Sets: []Value{{"autorestart", "1"}, {"sleep", "0"}, {"displaysleep", "5"}, {"disksleep", "0"}, {"powernap", "0"}, {"womp", "1"}}, NeedsRoot: true, Reversible: true, Writes: []Stored{noted(conf("/Library/Preferences/SystemConfiguration/com.apple.PowerManagement.plist"), "pmset: no sleep, autorestart, wake")}},
	{Name: "lock-after-login", Description: "A LaunchAgent that locks the screen ~20s after auto-login", Category: CategoryHeadless, Places: hostKind, Effect: ProcLockAfterLogin, Sets: []Value{{"agent", "present"}, {"loaded", "yes"}}, NeedsRoot: true, Reversible: true, Writes: []Stored{noted(conf("/Users/jterrazz.agent/Library/LaunchAgents/ai.jterrazz.lock-after-login.plist"), "the agent user's `lock-after-login.plist`"), noted(cache("/Users/jterrazz.agent/.openclaw/logs"), "its logs")}},
	{Name: "sshd", Description: "macOS Remote Login (sshd)", Category: CategoryHeadless, Places: hostKind, Effect: ProcSshd, Sets: []Value{{"Remote Login", "on"}}, NeedsRoot: true, Reversible: true, Writes: []Stored{noted(conf("/System/Library/LaunchDaemons/ssh.plist"), "Remote Login, through systemsetup")}},
	{Name: "backup-exclusions", Description: "Keep ~/.cache and station's VMs out of Time Machine", Category: CategorySystem, Places: hostKind, Effect: ProcBackupExclusions, Reversible: true, Writes: []Stored{
		noted(conf("{cache}"), "a sticky Time Machine exclusion (`tmutil addexclusion`)"),
		noted(conf("{data}/station/vms"), "a sticky Time Machine exclusion (`tmutil addexclusion`)"),
	}},
	{Name: "tart-launcher", Description: "The tart launcher: Tart's home is station's VMs folder", Category: CategorySystem, Places: hostKind, Effect: Dotfiles{Files: []Copy{{From: "tart/tart", To: "~/.local/bin/tart", Mode: 0o755}}}},
}
