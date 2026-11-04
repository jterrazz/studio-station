package settings

import "github.com/jterrazz/studio-station/src/catalog"

type Report struct {
	Item    catalog.ConfigItem
	State   catalog.State
	Detail  string
	Applies bool
}

type Change struct {
	Item   string
	Verb   string
	Result catalog.State
	Before []KeyValue
	After  []KeyValue
	Steps  []ChangeStep
}

type KeyValue struct{ Key, Value string }

type ChangeStep struct {
	Label  string
	State  catalog.State
	Detail string
}
