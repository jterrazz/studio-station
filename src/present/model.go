package present

import "fmt"

type Tone int

const (
	Plain Tone = iota
	Strong
	Muted
	Accent
	Good
	Warn
	Bad
)

type Cell struct {
	Text string
	Tone Tone
}

type Row struct {
	Mark  Mark
	Cells []Cell
}

type Count struct {
	Mark Mark
	N    int
}

type Block interface{ isBlock() }

type Section struct {
	Title  string
	Counts []Count
	Blocks []Block
}

type Table struct{ Rows []Row }

type KeyValues struct{ Pairs []Pair }

type Pair struct {
	Key   string
	Value Cell
}

type Lines struct{ Lines []Cell }

type Notice struct {
	Tone Tone
	Text string
}

func (Section) isBlock()   {}
func (Table) isBlock()     {}
func (KeyValues) isBlock() {}
func (Lines) isBlock()     {}
func (Notice) isBlock()    {}

type Document struct {
	Command string
	Context string
	Blocks  []Block
	Hints   []string
}

type Answer struct {
	Doc  Document
	JSON any
}

func MachineContext(name, kind string, registered bool) string {
	if !registered || name == "" {
		return kind + " · " + Unregistered
	}
	return name + " · " + kind
}

func Bytes(n int64) string {
	// invariant: decimal units, as Finder, df -H and tart print sizes, so a size
	// station shows reads the same as the one the owner checks it against.
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value, suffix := float64(n)/unit, 0
	for value >= unit-0.05 && suffix < len(byteSuffixes)-1 {
		value /= unit
		suffix++
	}
	return fmt.Sprintf("%.1f %s", value, string(byteSuffixes[suffix])+"B")
}

const byteSuffixes = "KMGTPE"

const Unregistered = "unregistered"
