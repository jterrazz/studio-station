package machines

// value: who a machine's login is and where, and the command line that
// reaches it; the owner's client blocks make the short name reach it, a VM
// through its Mac's pipe.
type SSHTarget struct {
	User, Host, Command string
}
