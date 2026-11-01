package catalog

import "slices"

var Tools = slices.Concat(coreTools, AgentSkills)

var coreTools = []Tool{
	{Name: "homebrew", Description: "macOS package manager", Category: CategoryPackageManagers, Installs: Mac(Script("https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh")), Provides: []string{"brew"}, Version: Probe{Args: []string{"brew", "--version"}, Pattern: `^Homebrew (\S+)`}, Storage: storageHomebrew, System: true},
	{Name: "mise", Description: "Polyglot runtime manager: node, go, python, bun, rust, java", Category: CategoryPackageManagers, Installs: Everywhere(Brew("mise"), Script("https://mise.run")), Provides: []string{"mise"}, Storage: storageMise, System: true},
	{Name: "mas", Description: "Mac App Store command line", Category: CategoryPackageManagers, Installs: Mac(Brew("mas")), Provides: []string{"mas"}},
	{Name: "uv", Description: "Fast Python package and project manager", Category: CategoryPackageManagers, Installs: Everywhere(Brew("uv"), Mise("uv", "latest")), Provides: []string{"uv"}, Storage: storageUv},
	{Name: "cocoapods", Description: "Dependency manager for iOS and macOS projects", Category: CategoryPackageManagers, Installs: Mac(Brew("cocoapods")), Provides: []string{"pod"}, Storage: storageCocoapods},
	{Name: "pnpm", Description: "Fast, disk space efficient Node package manager", Category: CategoryPackageManagers, Installs: Everywhere(Brew("pnpm"), Mise("pnpm", "latest")), Provides: []string{"pnpm"}, Storage: storagePnpm},
	{Name: "skills", Description: "Agent skills package manager", Category: CategoryPackageManagers, Installs: Anywhere(Bun("skills")), Provides: []string{"skills"}, Requires: []string{"bun"}, Storage: storageSkills},

	{Name: "bun", Description: "Fast JavaScript runtime and package manager", Category: CategoryRuntimes, Installs: Anywhere(Mise("bun", "latest")), Provides: []string{"bun"}, Storage: storageBun},
	{Name: "go", Description: "Compiled systems programming language by Google", Category: CategoryRuntimes, Installs: Anywhere(Mise("go", "latest")), Provides: []string{"go"}, Storage: storageGo},
	{Name: "node", Description: "JavaScript runtime built on Chrome's V8 engine", Category: CategoryRuntimes, Installs: Anywhere(Mise("node", "lts")), Provides: []string{"node", "npm", "npx"}, Storage: storageNode},
	{Name: "java", Description: "Java Development Kit (Temurin)", Category: CategoryRuntimes, Installs: Anywhere(Mise("java", "temurin-21")), Provides: []string{"java"}, Version: Probe{Args: []string{"java", "-version"}, Pattern: `version "([^"]+)"`}},
	{Name: "python", Description: "Python programming language runtime", Category: CategoryRuntimes, Installs: Anywhere(Mise("python", "latest")), Provides: []string{"python3"}, Version: Probe{Args: []string{"python3", "--version"}, Pattern: `^Python (\S+)`}, Storage: storagePython},
	{Name: "rust", Description: "Systems programming language for safety and speed", Category: CategoryRuntimes, Installs: Anywhere(Mise("rust", "stable")), Provides: []string{"rustc", "cargo"}, Storage: storageRust},

	{Name: "ghostty", Description: "GPU-accelerated terminal emulator", Category: CategoryShellTerminal, Installs: Mac(Cask("ghostty")), Bundle: "Ghostty", Then: []string{"ghostty-dotfiles"}, Storage: storageGhostty},
	{Name: "ohmyzsh", Description: "Oh My Zsh shell framework", Category: CategoryShellTerminal, Installs: Anywhere(ScriptEnv("https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh", []string{"RUNZSH=no", "CHSH=no", "KEEP_ZSHRC=yes"})), Version: Probe{Path: "~/.oh-my-zsh", Args: []string{"git", "-C", "~/.oh-my-zsh", "rev-parse", "--short", "HEAD"}, Pattern: `^(\S+)`}, Storage: storageOhmyzsh},
	{Name: "tmux", Description: "Terminal multiplexer for persistent sessions", Category: CategoryShellTerminal, Installs: Everywhere(Brew("tmux"), Mise("tmux", "latest")), Provides: []string{"tmux"}, Then: []string{"tmux-dotfiles"}, Storage: storageTmux},
	{Name: "starship", Description: "Cross-shell prompt (path, git branch/status, versions)", Category: CategoryShellTerminal, Installs: Everywhere(Brew("starship"), Mise("starship", "latest")), Provides: []string{"starship"}, Then: []string{"starship-dotfiles"}, Storage: storageStarship},
	{Name: "zoxide", Description: "Smarter cd command", Replaces: "cd (jumping around)", Category: CategoryShellTerminal, Installs: Everywhere(Brew("zoxide"), Mise("zoxide", "latest")), Provides: []string{"zoxide"}, Storage: storageZoxide},

	{Name: "yazi", Description: "Terminal file manager", Replaces: "Finder in the terminal", Category: CategoryCLITools, Installs: Everywhere(Brew("yazi"), Mise("yazi", "latest")), Provides: []string{"yazi"}, Storage: storageYazi},
	{Name: "bat", Description: "Cat clone with syntax highlighting", Replaces: "cat (for humans)", Category: CategoryCLITools, Installs: Everywhere(Brew("bat"), Mise("bat", "latest")), Provides: []string{"bat"}, Storage: storageBat},
	{Name: "dust", Description: "Intuitive disk usage tool", Replaces: "du -sh", Category: CategoryCLITools, Installs: Everywhere(Brew("dust"), Mise("dust", "latest")), Provides: []string{"dust"}},
	{Name: "eza", Description: "Modern ls replacement", Replaces: "ls", Category: CategoryCLITools, Installs: Everywhere(Brew("eza"), Mise("eza", "latest")), Provides: []string{"eza"}},
	{Name: "fd", Description: "Fast find alternative", Replaces: "find", Category: CategoryCLITools, Installs: Everywhere(Brew("fd"), Mise("fd", "latest")), Provides: []string{"fd"}},
	{Name: "ripgrep", Description: "Fast grep, the rg agents and editors shell out to", Replaces: "grep (what agent and editor tools call)", Category: CategoryCLITools, Installs: Everywhere(Brew("ripgrep"), Mise("ripgrep", "latest")), Provides: []string{"rg"}},
	{Name: "tgrep", Description: "Trigram-indexed grep for large codebases", Replaces: "grep -r", Category: CategoryCLITools, Installs: Mac(Brew("tgrep")), Provides: []string{"tgrep"}},
	{Name: "sd", Description: "Intuitive sed alternative", Replaces: "sed (simple replaces)", Category: CategoryCLITools, Installs: Everywhere(Brew("sd"), Mise("sd", "latest")), Provides: []string{"sd"}},
	{Name: "difftastic", Description: "Structural diff tool", Replaces: "diff", Category: CategoryCLITools, Installs: Everywhere(Brew("difftastic"), Mise("difftastic", "latest")), Provides: []string{"difft"}},
	{Name: "jq", Description: "Command-line JSON processor", Category: CategoryCLITools, Installs: Everywhere(Brew("jq"), Mise("jq", "latest")), Provides: []string{"jq"}},

	{Name: "git", Description: "Distributed version control system (the Command Line Tools)", Category: CategoryGit, Installs: Mac(Shell("xcode-select --install")), Provides: []string{"git"}, Version: Probe{Args: []string{"git", "--version"}, Pattern: `git version (\S+)`}, Storage: storageGit, System: true},
	{Name: "gpg", Description: "GNU Privacy Guard for encryption and signing", Category: CategoryGit, Installs: Mac(Brew("gnupg")), Provides: []string{"gpg"}, Then: []string{"git-signing"}, Storage: storageGpg},
	{Name: "lazygit", Description: "Terminal UI for git", Replaces: "raw git porcelain (interactive)", Category: CategoryGit, Installs: Everywhere(Brew("lazygit"), Mise("lazygit", "latest")), Provides: []string{"lazygit"}, Storage: storageLazygit},
	{Name: "gh", Description: "GitHub CLI for repository management", Replaces: "raw GitHub API / web UI", Category: CategoryGit, Installs: Everywhere(Brew("gh"), Mise("gh", "latest")), Provides: []string{"gh"}, Then: []string{"github-login"}, Storage: storageGh},

	{Name: "zed", Description: "Zed code editor", Category: CategoryEditors, Installs: Mac(Cask("zed")), Bundle: "Zed", Then: []string{"zed-dotfiles"}, Storage: storageZed},
	{Name: "cursor", Description: "AI-powered code editor", Category: CategoryEditors, Installs: Mac(Cask("cursor")), Bundle: "Cursor", Storage: storageCursor},
	{Name: "android-studio", Description: "Android development IDE", Category: CategoryEditors, Installs: Mac(Cask("android-studio")), Bundle: "Android Studio", Storage: storageAndroidStudio},
	{Name: "xcode", Description: "Apple development IDE", Category: CategoryEditors, Installs: Mac(Mas("Xcode", 497799835)), Provides: []string{"xcodebuild"}, Bundle: "Xcode", Version: Probe{Args: []string{"xcodebuild", "-version"}, Pattern: `^Xcode (\S+)`}, Storage: storageXcode},
	{Name: "xcodegen", Description: "Generates Xcode projects from a spec", Category: CategoryEditors, Installs: Mac(Brew("xcodegen")), Provides: []string{"xcodegen"}},

	{Name: "tart", Description: "Virtual machines on Apple silicon: the engine of station's VMs", Category: CategoryContainersVMs, Installs: HostOnly(Shell(`d="$HOME/.local/share"; case "$XDG_DATA_HOME" in /*) d="$XDG_DATA_HOME" ;; esac; mkdir -p "$d/station/tart" "$d/station/vms" "$HOME/.local/bin" && curl -fsSL https://github.com/cirruslabs/tart/releases/latest/download/tart.tar.gz | tar -xz -C "$d/station/tart"`)), Provides: []string{"tart"}, Then: []string{"tart-launcher"}, Version: Probe{Args: []string{"tart", "--version"}, Pattern: `^(\S+)`}, Storage: storageTart, System: true},
	{Name: "docker", Description: "Container engine, inside the Linux VM", Category: CategoryContainersVMs, Installs: LinuxOnly(Script("https://get.docker.com")), Provides: []string{"docker"}, Version: Probe{Args: []string{"docker", "--version"}, Pattern: `Docker version ([^,\s]+)`}, Storage: storageDocker},
	{Name: "orbstack", Description: "Docker and Linux machines on the Mac, for exploration on a host", Category: CategoryContainersVMs, Installs: HostOnly(Cask("orbstack")), Provides: []string{"orb"}, Bundle: "OrbStack", Storage: storageOrbstack},
	{Name: "lens", Description: "Kubernetes IDE for managing clusters", Category: CategoryContainersVMs, Installs: Mac(Cask("lens")), Bundle: "Lens"},
	{Name: "kubectl", Description: "Kubernetes command line", Category: CategoryContainersVMs, Installs: Everywhere(Brew("kubectl"), Mise("kubectl", "latest")), Provides: []string{"kubectl"}, Storage: storageKubectl},
	{Name: "helm", Description: "Kubernetes package manager", Category: CategoryContainersVMs, Installs: Everywhere(Brew("helm"), Mise("helm", "latest")), Provides: []string{"helm"}, Storage: storageHelm},
	{Name: "helmfile", Description: "Declarative deployments of Helm charts", Category: CategoryContainersVMs, Installs: Everywhere(Brew("helmfile"), Mise("helmfile", "latest")), Provides: []string{"helmfile"}},

	{Name: "ansible", Description: "Agentless IT automation and configuration management", Category: CategoryDeploy, Installs: Everywhere(Brew("ansible"), Mise("pipx:ansible", "latest")), Provides: []string{"ansible"}, Storage: storageAnsible},
	{Name: "eas", Description: "Expo Application Services CLI for app builds", Category: CategoryDeploy, Installs: Anywhere(Bun("eas-cli")), Provides: []string{"eas"}, Requires: []string{"bun"}, Storage: storageEas},
	{Name: "pulumi", Description: "Infrastructure as code using real programming languages", Category: CategoryDeploy, Installs: Everywhere(TrustedBrew("pulumi/tap/pulumi"), Mise("pulumi", "latest")), Provides: []string{"pulumi"}, Storage: storagePulumi},
	{Name: "terraform", Description: "Infrastructure as code provisioning tool", Category: CategoryDeploy, Installs: Everywhere(TrustedBrew("hashicorp/tap/terraform"), Mise("terraform", "latest")), Provides: []string{"terraform"}, Storage: storageTerraform},

	{Name: "claude", Description: "Claude Code CLI for agentic coding", Category: CategoryAIAgents, Installs: Anywhere(Script("https://claude.ai/install.sh")), Provides: []string{"claude"}, Version: Probe{Args: []string{"claude", "--version"}, Pattern: `^(\S+)`}, Storage: storageClaude},
	{Name: "codex", Description: "OpenAI's CLI coding agent", Category: CategoryAIAgents, Installs: Everywhere(Bun("@openai/codex"), Mise("codex", "latest")), Provides: []string{"codex"}, Requires: []string{"bun"}, Storage: storageCodex},
	{Name: "gemini", Description: "Google's CLI coding agent", Category: CategoryAIAgents, Installs: Everywhere(Brew("gemini-cli"), Mise("gemini", "latest")), Provides: []string{"gemini"}, Storage: storageGemini},
	{Name: "opencode", Description: "Open-source AI coding agent CLI", Category: CategoryAIAgents, Installs: Everywhere(Brew("opencode"), Mise("opencode", "latest")), Provides: []string{"opencode"}, Storage: storageOpencode},
	{Name: "hermes", Description: "Self-improving AI agent with persistent memory", Category: CategoryAIAgents, Installs: VMs(Script("https://hermes-agent.nousresearch.com/install.sh", "--skip-setup"), Script("https://hermes-agent.nousresearch.com/install.sh", "--skip-setup")), Provides: []string{"hermes"}, Version: Probe{Args: []string{"hermes", "--version"}, Pattern: `(\d+\.\d+\S*)`}, Storage: storageHermes},
	{Name: "conductor", Description: "Runs parallel AI coding agents in git worktrees", Category: CategoryAIAgents, Installs: HostOnly(Cask("conductor")), Bundle: "Conductor"},
	{Name: "herdr", Description: "Agent multiplexer that lives in your terminal", Category: CategoryAIAgents, Installs: Everywhere(Brew("herdr"), Mise("herdr", "latest")), Provides: []string{"herdr"}},
	{Name: "herdr-app", Description: "Desktop app that keeps coding agents running in the background", Category: CategoryAIAgents, Installs: HostOnly(Cask("herdr-app")), Bundle: "Herdr", Storage: storageHerdrApp},

	{Name: "ollama", Description: "Run large language models locally, on the host's GPU", Category: CategoryAITooling, Installs: HostOnly(Cask("ollama-app")), Provides: []string{"ollama"}, Bundle: "Ollama", Live: Liveness{Process: "ollama"}, Storage: storageOllama},
	{Name: "qmd", Description: "Local semantic search over markdown", Replaces: "grep over markdown notes", Category: CategoryAITooling, Installs: Anywhere(BunAs("github:tobi/qmd", "qmd")), Provides: []string{"qmd"}, Requires: []string{"bun"}, Storage: storageQmd},
	{Name: "rtk", Description: "Token-optimized CLI proxy for agents", Replaces: "raw dev commands (token-heavy output)", Category: CategoryAITooling, Installs: Everywhere(Brew("rtk"), Mise("rtk", "latest")), Provides: []string{"rtk"}, Storage: storageRtk},
	{Name: "browser-use", Description: "AI agent library for browser automation", Category: CategoryAITooling, Installs: VMs(Uv("browser-use"), Mise("pipx:browser-use", "latest")), Provides: []string{"browser-use"}, Requires: []string{"uv"}},
	{Name: "markitdown", Description: "Convert files to Markdown for LLMs", Category: CategoryAITooling, Installs: VMs(Uv("markitdown[all]"), Mise("pipx:markitdown[all]", "latest")), Provides: []string{"markitdown"}, Requires: []string{"uv"}},
	{Name: "playwright-mcp", Description: "Browser automation for AI agents via MCP", Category: CategoryAITooling, Installs: VMs(Npm("@playwright/mcp"), Mise("npm:@playwright/mcp", "latest")), Requires: []string{"node"}},
	{Name: "playwright-browsers", Description: "Chromium runtime for website specs (@jterrazz/test)", Category: CategoryAITooling, Installs: VMs(Shell("npx playwright install chromium"), Shell("npx playwright install chromium")), Requires: []string{"node"}, Version: Probe{Path: "~/Library/Caches/ms-playwright/chromium*"}, Storage: storagePlaywrightBrowsers},
	{Name: "inferrs", Description: "TurboQuant LLM inference server", Category: CategoryAITooling, Installs: Mac(TrustedBrew("ericcurtin/inferrs/inferrs")), Provides: []string{"inferrs"}},
	{Name: "agent-browser", Description: "Browser automation CLI for AI agents", Category: CategoryAITooling, Installs: VMs(Bun("agent-browser"), Bun("agent-browser")), Provides: []string{"agent-browser"}, Requires: []string{"bun"}, Storage: storageAgentBrowser},
	{Name: "plannotator", Description: "Review AI agent plans and code before committing", Category: CategoryAITooling, Installs: Anywhere(Script("https://plannotator.ai/install.sh")), Provides: []string{"plannotator"}, Version: Probe{Args: []string{"plannotator", "--version"}, Pattern: `(\d+\.\d+\S*)`}},

	{Name: "chatgpt", Description: "OpenAI ChatGPT desktop app", Category: CategoryAIApps, Installs: HostOnly(Cask("chatgpt")), Bundle: "ChatGPT"},
	{Name: "claude-desktop", Description: "Anthropic Claude desktop app", Category: CategoryAIApps, Installs: HostOnly(Cask("claude")), Bundle: "Claude", Storage: storageClaudeDesktop},
	{Name: "typewhisper", Description: "Whisper-based dictation app", Category: CategoryAIApps, Installs: HostOnly(Cask("typewhisper")), Bundle: "TypeWhisper"},

	{Name: "brave", Description: "Privacy-focused web browser", Category: CategoryBrowsers, Installs: HostOnly(Cask("brave-browser")), Bundle: "Brave Browser"},
	{Name: "dia", Description: "The Browser Company's AI-first browser", Category: CategoryBrowsers, Installs: HostOnly(App("download it from its website")), Bundle: "Dia"},

	{Name: "discord", Description: "Voice and text chat", Category: CategoryCommunication, Installs: HostOnly(Cask("discord")), Bundle: "Discord", Storage: storageDiscord},
	{Name: "slack", Description: "Team communication", Category: CategoryCommunication, Installs: HostOnly(Cask("slack")), Bundle: "Slack"},
	{Name: "whatsapp", Description: "Messaging app", Category: CategoryCommunication, Installs: HostOnly(Cask("whatsapp")), Bundle: "WhatsApp"},
	{Name: "messenger", Description: "Facebook Messenger", Category: CategoryCommunication, Installs: HostOnly(App("no current source")), Bundle: "Messenger"},

	{Name: "linear", Description: "Project management tool", Category: CategoryProductivity, Installs: HostOnly(Cask("linear-linear")), Bundle: "Linear"},
	{Name: "notion", Description: "Workspace for notes and docs", Category: CategoryProductivity, Installs: HostOnly(Cask("notion")), Bundle: "Notion", Storage: storageNotion},
	{Name: "obsidian", Description: "Knowledge base and note-taking", Category: CategoryProductivity, Installs: HostOnly(Cask("obsidian")), Bundle: "Obsidian", Storage: storageObsidian},
	{Name: "keynote", Description: "Apple presentations", Category: CategoryProductivity, Installs: HostOnly(Mas("Keynote", 361285480)), Bundle: "Keynote"},
	{Name: "numbers", Description: "Apple spreadsheets", Category: CategoryProductivity, Installs: HostOnly(Mas("Numbers", 361304891)), Bundle: "Numbers"},
	{Name: "pages", Description: "Apple word processor", Category: CategoryProductivity, Installs: HostOnly(Mas("Pages", 361309726)), Bundle: "Pages"},
	{Name: "raindrop", Description: "Bookmark manager", Category: CategoryProductivity, Installs: HostOnly(Mas("Save to Raindrop.io", 1549370672)), Bundle: "Save to Raindrop.io"},

	{Name: "broadcasts", Description: "Internet radio and stream player", Category: CategoryMedia, Installs: HostOnly(Mas("Broadcasts", 1469995354)), Bundle: "Broadcasts"},
	{Name: "compressor", Description: "Apple video compression tool", Category: CategoryMedia, Installs: HostOnly(Mas("Compressor", 424390742)), Bundle: "Compressor"},
	{Name: "final-cut-pro", Description: "Professional video editor", Category: CategoryMedia, Installs: HostOnly(Mas("Final Cut Pro", 424389933)), Bundle: "Final Cut Pro"},
	{Name: "lightroom", Description: "Adobe photo editor", Category: CategoryMedia, Installs: HostOnly(Mas("Adobe Lightroom", 1451544217)), Bundle: "Adobe Lightroom"},
	{Name: "logic-pro", Description: "Professional music production", Category: CategoryMedia, Installs: HostOnly(Mas("Logic Pro", 634148309)), Bundle: "Logic Pro"},

	{Name: "jump-desktop-connect", Description: "Remote GUI recovery host service for headless Macs", Category: CategoryRemoteAccess, Installs: HostOnly(Cask("jump-desktop-connect")), Bundle: "Jump Desktop Connect"},
	{Name: "jump-desktop", Description: "Jump Desktop viewer/client app", Category: CategoryRemoteAccess, Installs: HostOnly(Cask("jump-desktop")), Bundle: "Jump Desktop"},
	{Name: "tailscale", Description: "The Tailscale app: each host is a node, and station reads it", Category: CategoryRemoteAccess, Installs: HostOnly(Cask("tailscale-app")), Supersedes: "tailscale", Provides: []string{"tailscale"}, Bundle: "Tailscale", Storage: storageTailscale, System: true},

	{Name: "bitwarden", Description: "Password manager", Category: CategorySecurityApps, Installs: HostOnly(Cask("bitwarden")), Bundle: "Bitwarden"},
	{Name: "adguard", Description: "Ad blocker for Safari", Category: CategorySecurityApps, Installs: HostOnly(Mas("AdGuard for Safari", 1440147259)), Bundle: "AdGuard for Safari"},
	{Name: "passepartout", Description: "VPN client", Category: CategorySecurityApps, Installs: HostOnly(Mas("Passepartout", 1433648537)), Bundle: "Passepartout"},

	{Name: "mole", Description: "Clean, uninstall, analyze, and optimize macOS from the terminal", Category: CategorySystemUtilities, Installs: HostOnly(Brew("mole")), Provides: []string{"mo"}, Storage: storageMole},
	{Name: "betterdisplay", Description: "Flexible HiDPI scaling and display management", Category: CategorySystemUtilities, Installs: HostOnly(Cask("betterdisplay")), Bundle: "BetterDisplay"},
	{Name: "pipifier", Description: "Picture-in-Picture for Safari", Category: CategorySystemUtilities, Installs: HostOnly(Mas("PiPifier", 1160374471)), Bundle: "PiPifier"},
	{Name: "snippety", Description: "Code snippet manager", Category: CategorySystemUtilities, Installs: HostOnly(Mas("Snippety", 1530751461)), Bundle: "Snippety"},
	{Name: "speedtest", Description: "Internet speed test", Category: CategorySystemUtilities, Installs: HostOnly(Mas("Speedtest", 1153157709)), Bundle: "Speedtest"},
}
