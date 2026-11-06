package words

const PowerOnItsMac = "ssh <host> station power"

func SettingsAsRoot(verb, item string) string { return "sudo station settings " + verb + " " + item }

func PasswordlessRestart(mac string) string {
	return "check that sudo needs no password for fdesetup on " + mac
}

func ReachedThrough(vm, mac string) string {
	return "ssh " + vm + " goes through " + mac + ": station machines"
}

func RunOnItself(machine string) string { return "run it on " + machine + " itself: ssh " + machine }
