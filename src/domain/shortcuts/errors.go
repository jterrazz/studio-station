package shortcuts

type DockerStepError struct {
	Step   string
	Detail string
}

func (e *DockerStepError) Error() string {
	if e.Detail == "" {
		return e.Step + " failed"
	}
	return e.Step + " failed: " + e.Detail
}

type NotInstalledError struct{ Tool string }

func (e *NotInstalledError) Error() string { return e.Tool + " is not installed" }

type UnknownShortcutError struct{ Name string }

func (e *UnknownShortcutError) Error() string { return "unknown shortcut \"" + e.Name + "\"" }
