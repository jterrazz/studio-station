package catalog

type Fact string // value: a fact of the machine a Checked setting reads, never writes.

const (
	FactFileVault  Fact = "filevault"
	FactFirewall   Fact = "firewall"
	FactSIP        Fact = "sip"
	FactGatekeeper Fact = "gatekeeper"
	// value: whether the operating system has updates waiting.
	FactSystemUpdates Fact = "system-updates"
)
