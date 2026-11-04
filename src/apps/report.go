package apps

import "github.com/jterrazz/studio-station/src/catalog"

type Report struct {
	Tool    catalog.Tool
	Method  catalog.Method
	State   catalog.State
	Version string
	Managed bool
	Detail  string
	Live    string
	Pending bool
	// value: why the state is not ok, in a word a page can judge; Needs names
	// the catalogue tool whose absence explains it.
	Why   Why
	Needs string
}

type Why string

const (
	WhyMissing   Why = "missing"
	WhyUnmanaged Why = "unmanaged"
	WhyOutdated  Why = "outdated"
	WhyPartial   Why = "partial"
	WhyLate      Why = "late"
	WhyFailing   Why = "failing"
)

type Outcome struct {
	Name   string
	Verb   string
	Result catalog.State
	Detail string
	Steps  []Step
}

type Step struct {
	Item   string
	Result catalog.State
	Detail string
}

type InstallRequest struct {
	Names       []string
	Interactive bool
}

type UpgradeRequest struct {
	Names []string
	All   bool
	Via   []catalog.Manager
}
