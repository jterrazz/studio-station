package tailnet

type NotInstalledError struct{ Missing string }

func (e *NotInstalledError) Error() string { return e.Missing + " is not installed" }

type AuthError struct{ Detail string }

func (e *AuthError) Error() string { return "tailscale up failed: " + e.Detail }

type DaemonError struct{ Detail string }

func (e *DaemonError) Error() string { return e.Detail }

type NoVMJoinError struct{}

func (*NoVMJoinError) Error() string { return "the VM-join client is not set up" }

type NeedsAuthError struct{}

func (*NeedsAuthError) Error() string { return "the tailnet login is not set up" }

type MintRefusedError struct {
	Status int
	Detail string
}

func (e *MintRefusedError) Error() string {
	if e.Detail == "" {
		return "tailscale refused the VM-join request"
	}
	return "tailscale refused the VM-join request: " + e.Detail
}

type RemoveRefusedError struct {
	NodeID string
	Status int
}

func (e *RemoveRefusedError) Error() string {
	return "tailscale refused to remove the node \"" + e.NodeID + "\""
}
