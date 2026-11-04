package catalog

type Fact string // value: a fact of the machine an Inspector reads, never writes: what a Checked setting and the Overview look at.

const (
	FactFileVault  Fact = "filevault"
	FactFirewall   Fact = "firewall"
	FactSIP        Fact = "sip"
	FactGatekeeper Fact = "gatekeeper"
	// value: whether the operating system has updates waiting.
	FactSystemUpdates Fact = "system-updates"
	// value: the kernel's name and release ("Darwin 25.0.0").
	FactKernel  Fact = "kernel"
	FactLocalIP Fact = "local-ip"
	// value: the resolvers the machine asks, at most two, the local stub left out.
	FactDNS Fact = "dns"
	// value: the owner's global git identity and signing, the keys they sign
	// and push with, and who GitHub's CLI is signed in as.
	FactGitEmail   Fact = "git-email"
	FactGitName    Fact = "git-name"
	FactGitSigning Fact = "git-signing"
	FactGPGKey     Fact = "gpg-key"
	FactGitHub     Fact = "github"
	FactSSHKey     Fact = "ssh-key"
)
