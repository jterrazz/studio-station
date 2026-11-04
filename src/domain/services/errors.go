package services

import "strconv"

type BadPortError struct{ Value string }

func (e *BadPortError) Error() string { return "\"" + e.Value + "\" is not a port" }

type IgnoredPortError struct{ Port, MinPort int }

func (e *IgnoredPortError) Error() string {
	return "port " + strconv.Itoa(e.Port) + " is never shared"
}

type HostAwayError struct{ Host string }

func (e *HostAwayError) Error() string {
	if e.Host == "" {
		return "no host has connected to this VM yet"
	}
	return e.Host + " is not connected to this VM"
}

func (e *HostAwayError) Hint() string {
	if e.Host == "" {
		return "on the VM's host: station setup"
	}
	return "on " + e.Host + ": station setup"
}

type UnknownVMError struct{ Name string }

func (e *UnknownVMError) Error() string { return "unknown VM \"" + e.Name + "\"" }
