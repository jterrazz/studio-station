package catalog

import "time"

type CheckedSetting struct {
	// value: a setting station reads from macOS and never applies: what the
	// machine should have, the facts it is judged on, and how the owner
	// fixes it.
	Name, Label, Description string
	Places                   []MachineKind
	// value: the facts it is judged on, the first its own.
	Facts []Fact
	// value: where the owner fixes it, a System Settings path or a step
	// outside station; never a station command.
	Fix string
	// value: how long its facts may take; past it the row is Unknown.
	Budget time.Duration
}

const (
	checkedBudget = 2 * time.Second
	updatesBudget = 10 * time.Second
)

var CheckedSettings = []CheckedSetting{
	{Name: "filevault", Label: "Disk encryption", Description: "FileVault encrypts the disk", Places: hostKind, Facts: []Fact{FactFileVault}, Fix: "System Settings > Privacy & Security > FileVault", Budget: checkedBudget},
	{Name: "firewall", Label: "Firewall", Description: "The macOS firewall refuses unasked connections", Places: hostKind, Facts: []Fact{FactFirewall}, Fix: "System Settings > Network > Firewall", Budget: checkedBudget},
	{Name: "sip", Label: "System Integrity Protection", Description: "SIP keeps the system files read-only", Places: hostKind, Facts: []Fact{FactSIP}, Fix: "Recovery > Terminal > csrutil enable", Budget: checkedBudget},
	{Name: "automatic-updates", Label: "Automatic security updates", Description: "macOS installs its security responses by itself", Places: hostKind, Facts: []Fact{FactAutoUpdates}, Fix: "System Settings > General > Software Update > Automatic updates", Budget: checkedBudget},
	{Name: "screen-lock", Label: "Screen lock", Description: "The screen locks after 5 min idle (a desktop: 5 min after the display turns off)", Places: hostKind, Facts: []Fact{FactScreenLock, FactDisplaySleep}, Fix: "System Settings > Lock Screen", Budget: checkedBudget},
	{Name: "gatekeeper", Label: "Gatekeeper", Description: "Gatekeeper opens only signed apps", Places: hostKind, Facts: []Fact{FactGatekeeper}, Fix: "System Settings > Privacy & Security > Security", Budget: checkedBudget},
	{Name: "remote-login", Label: "Remote login", Description: "Remote login is off on a laptop, on on an always-on Mac", Places: hostKind, Facts: []Fact{FactRemoteLogin}, Fix: "System Settings > General > Sharing > Remote Login", Budget: checkedBudget},
	{Name: "time-machine", Label: "Time Machine", Description: "Time Machine's last backup is within 7 days", Places: hostKind, Facts: []Fact{FactTimeMachine}, Fix: "System Settings > General > Time Machine", Budget: checkedBudget},
	{Name: "macos-updates", Label: "macOS updates", Description: "No macOS update is waiting", Places: hostKind, Facts: []Fact{FactSystemUpdates}, Fix: "System Settings > General > Software Update", Budget: updatesBudget},
}
