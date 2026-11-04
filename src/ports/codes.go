package ports

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/core/fail"
)

const (
	CodeRangeFull fail.Code = "port.range-full"
	CodeNotFound  fail.Code = "port.not-found"
	CodeSignedOut fail.Code = "port.signed-out"
	CodeNeedsVM   fail.Code = "port.needs-vm"
	CodeElsewhere fail.Code = "port.on-another-mac"
)

func NotFound(vm string, port int) error {
	return fail.New(fail.Refused, CodeNotFound, vm+":"+strconv.Itoa(port), map[string]string{"vm": vm})
}

func SignedOut(mac string) error { return fail.New(fail.Failed, CodeSignedOut, mac, nil) }

func (e *VMFull) Refusal() error {
	return fail.New(fail.Failed, CodeRangeFull, e.VM, map[string]string{"max": strconv.Itoa(e.Max)})
}
