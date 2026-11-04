//go:build !darwin || !cgo

package darwin

type Kernel struct{}

func (Kernel) Counters() (Counters, error) { return Counters{}, errUnsupported }

func (Kernel) Processes() (Procs, error) { return Procs{}, errUnsupported }

func (Kernel) Process(int) (Procs, error) { return Procs{}, errUnsupported }
