package services

import "strconv"

type RangeFullError struct{ Low, High int }

func (e *RangeFullError) Error() string {
	return "range " + strconv.Itoa(e.Low) + "-" + strconv.Itoa(e.High) + " full"
}

type MaxPerVMError struct {
	VM  string
	Max int
}

func (e *MaxPerVMError) Error() string { return strconv.Itoa(e.Max) + " per VM" }

type NoLeaseError struct {
	VM   string
	Port int
}

func (e *NoLeaseError) Error() string {
	return e.VM + " holds no lease on " + strconv.Itoa(e.Port)
}

type BadPortError struct{ Value string }

func (e *BadPortError) Error() string { return "\"" + e.Value + "\" is not a port" }

type BadTargetError struct {
	Value string
	InVM  bool
}

func (e *BadTargetError) Error() string { return "\"" + e.Value + "\" is not a service to release" }

type IgnoredPortError struct{ Port, MinPort int }

func (e *IgnoredPortError) Error() string {
	return "port " + strconv.Itoa(e.Port) + " is not published"
}

type HostAwayError struct{ Host string }

func (e *HostAwayError) Error() string { return e.Host + " is not connected to this VM" }

type PinnedError struct {
	VM   string
	Port int
}

func (e *PinnedError) Error() string {
	return e.VM + ":" + strconv.Itoa(e.Port) + " is a profile's lease: its URL is in the VM's environment"
}

type VMOnlyError struct{}

func (*VMOnlyError) Error() string { return "station services url runs inside a VM" }

type UnknownVMError struct{ Name string }

func (e *UnknownVMError) Error() string { return "unknown VM \"" + e.Name + "\"" }

type RemoteError struct {
	Host string
	Code int
}

func (e *RemoteError) Error() string {
	return "cannot reach \"" + e.Host + "\""
}

type HereInVMError struct{}

func (*HereInVMError) Error() string { return "--here is for a host: a VM lists only its own services" }
