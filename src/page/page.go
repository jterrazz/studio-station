package page

const Schema = "station.page/v1"

type Page struct {
	ID       ID        `json:"id,omitempty"`
	Command  string    `json:"command,omitempty"`
	Context  Context   `json:"context,omitzero"`
	State    State     `json:"state,omitempty"`
	Strip    *Strip    `json:"strip,omitempty"`
	Sections []Section `json:"sections,omitempty"`
	Hints    []Hint    `json:"hints,omitempty"`
	Error    *Error    `json:"error,omitempty"`
	Actions  []Action  `json:"actions,omitempty"`
}

type ID string

type State string

const (
	Loading State = "loading"
	Ready   State = "ready"
	Failed  State = "failed"
)

type Context struct {
	Machine    string `json:"machine,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Registered bool   `json:"registered,omitempty"`
}

type Section struct {
	ID      string  `json:"id,omitempty"`
	Title   string  `json:"title,omitempty"`
	Role    Role    `json:"role,omitempty"`
	Fold    Fold    `json:"fold,omitempty"`
	Caption []Value `json:"caption,omitempty"`
	// value: the caption reads on as the title's own words, one list with it
	// (All machines' first line), not a muted aside.
	Joined  bool     `json:"joined,omitempty"`
	Columns []Column `json:"columns,omitempty"`
	Items   []Item   `json:"items,omitempty"`
	Counts  []Tally  `json:"counts,omitempty"`
	// value: a section whose marks are no verdict to sum (storage's sizes) shows no counts.
	Uncounted bool `json:"uncounted,omitempty"`
	// value: the pipe prints a title's counts only where its records always
	// carried them (the tools and config categories, Home's Tools, Config and
	// Background); every other `# <title>` line stays bare (§20).
	PipeCounts bool `json:"pipe_counts,omitempty"`
	// value: the pipe prints each heading group as its own titled section
	// (Secrets' `# Lend list` and `# Kept`), never the section's title: its
	// records do not depend on how the eye nests them.
	PipeGroups bool `json:"pipe_groups,omitempty"`
	// value: a Summary drawn in two columns starts its right one at this item.
	Break int `json:"break,omitempty"`
}

type Role string

const (
	Plain   Role = ""
	Actions Role = "actions"
	Summary Role = "summary"
	Banner  Role = "banner"
)

type Fold string

const (
	Open   Fold = "open"
	Folded Fold = "folded"
	Auto   Fold = "auto"
)

type Column struct {
	Key    string   `json:"key,omitempty"`
	Label  string   `json:"label,omitempty"`
	Kind   Kind     `json:"kind,omitempty"`
	Keep   int      `json:"keep,omitempty"`
	Show   Show     `json:"show,omitempty"`
	Levels []string `json:"levels,omitempty"`
	// value: a column a short line drops whole rather than cuts (a process
	// name, a machine's resources: half of one says nothing, §13.5, §13.6).
	Whole bool `json:"whole,omitempty"`
}

type Show string

const (
	Everywhere Show = ""
	PipeOnly   Show = "pipe"
	// value: a column for the eye only (a peer's online state, a host's role):
	// the pipe's records keep the fields they always had (§20).
	EyeOnly Show = "eye"
)

type Item struct {
	Row   *Row   `json:"row,omitempty"`
	Fact  *Fact  `json:"fact,omitempty"`
	Group *Group `json:"group,omitempty"`
	Empty *Empty `json:"empty,omitempty"`
	Error *Error `json:"error,omitempty"`
	Note  *Note  `json:"note,omitempty"`
	Link  *Link  `json:"link,omitempty"`
}

type Row struct {
	ID    string `json:"id,omitempty"`
	Mark  Mark   `json:"mark,omitempty"`
	Label string `json:"label,omitempty"`
	// value: the label a terminal shows when the full one is for records
	// (a launchd label without its owner's prefix); the pipe and json keep Label.
	Short  string  `json:"short,omitempty"`
	Cells  []Value `json:"cells,omitempty"`
	Detail Value   `json:"detail,omitzero"`
	Reason Reason  `json:"reason,omitempty"`
	Hint   *Hint   `json:"hint,omitempty"`
}

type Fact struct {
	Key   string `json:"key,omitempty"`
	Value Value  `json:"value,omitzero"`
}

type Group struct {
	Head  Row    `json:"head,omitzero"`
	Items []Item `json:"items,omitempty"`
	Fold  Fold   `json:"fold,omitempty"`
}

func (g Group) Heading() bool {
	// value: a head with no mark and no cells only names its children's group
	// (Background's `known`): it carries no verdict and the pipe prints no record for it.
	return g.Head.Mark == Unjudged && len(g.Head.Cells) == 0
}

type Empty struct {
	Noun string `json:"noun,omitempty"`
	// value: the whole line when "no <noun>" is not it (No VMs yet).
	Text string `json:"text,omitempty"`
	Hint *Hint  `json:"hint,omitempty"`
}

type Error struct {
	Message string `json:"message,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

type Note struct {
	Text string `json:"text,omitempty"`
}

type Link struct {
	Label string `json:"label,omitempty"`
	// value: what the line leads with in the label's place, never cut (a
	// service's `5173 → 40001`); Label stays the link's name for the records.
	Lead  Value `json:"lead,omitzero"`
	Value Value `json:"value,omitzero"`
	// value: dropped whole, never cut, when the line is short (a process name).
	Detail Value  `json:"detail,omitzero"`
	Page   ID     `json:"page,omitempty"`
	Row    string `json:"row,omitempty"`
}

type Hint struct {
	Command  string `json:"command,omitempty"`
	External bool   `json:"external,omitempty"`
	Key      string `json:"key,omitempty"`
	Text     string `json:"text,omitempty"`
}

type Action struct {
	Key      string   `json:"key,omitempty"`
	Label    string   `json:"label,omitempty"`
	On       Scope    `json:"on,omitempty"`
	Rows     []string `json:"rows,omitempty"`
	Confirm  *Confirm `json:"confirm,omitempty"`
	Priority int      `json:"priority,omitempty"`
	Mutates  bool     `json:"mutates,omitempty"`
	ID       string   `json:"id,omitempty"`
	Run      *Run     `json:"run,omitempty"`
	Form     *Form    `json:"form,omitempty"`
}

type Run struct {
	// value: the call the action makes, by its name in calls.json, over the
	// engine's socket.
	Call string `json:"call,omitempty"`
	// value: the call's input the action fixes, by the path of each field
	// (`Request.Image`); a Form's answers fill the paths their keys name.
	In map[string]string `json:"in,omitempty"`
	// value: a new terminal runs station, with these words after it: the one
	// way an action starts a program.
	Terminal bool     `json:"terminal,omitempty"`
	Argv     []string `json:"argv,omitempty"`
}

type Scope string

const (
	OnRow  Scope = "row"
	OnPage Scope = "page"
)

type Confirm struct {
	Question string  `json:"question,omitempty"`
	Lines    []Value `json:"lines,omitempty"`
	Danger   bool    `json:"danger,omitempty"`
	Type     string  `json:"type,omitempty"`
	Yes      string  `json:"yes,omitempty"`
	No       string  `json:"no,omitempty"`
}

type Form struct {
	Title  string  `json:"title,omitempty"`
	Fields []Field `json:"fields,omitempty"`
}

type Field struct {
	Key      string    `json:"key,omitempty"`
	Label    string    `json:"label,omitempty"`
	Kind     FieldKind `json:"kind,omitempty"`
	Options  []Option  `json:"options,omitempty"`
	Default  string    `json:"default,omitempty"`
	Note     string    `json:"note,omitempty"`
	Required bool      `json:"required,omitempty"`
}

type FieldKind string

const (
	FieldText    FieldKind = "text"
	FieldSecret  FieldKind = "secret"
	FieldSelect  FieldKind = "select"
	FieldMulti   FieldKind = "multiselect"
	FieldConfirm FieldKind = "confirm"
)

type Option struct {
	Value string `json:"value,omitempty"`
	Label string `json:"label,omitempty"`
	Note  string `json:"note,omitempty"`
}

type Strip struct {
	CPU     Value   `json:"cpu,omitzero"`
	Spark   []int   `json:"spark,omitempty"`
	Memory  Value   `json:"memory,omitzero"`
	Battery Value   `json:"battery,omitzero"`
	Net     []Value `json:"net,omitempty"`
	// value: a held Power session's word and time left, after the battery.
	Power Value    `json:"power,omitzero"`
	Top   *Section `json:"top,omitempty"`
}

type Tally struct {
	Mark Mark `json:"mark,omitempty"`
	N    int  `json:"n,omitempty"`
}

type Mark string

const (
	Unjudged  Mark = ""
	OK        Mark = "ok"
	Attention Mark = "attention"
	Absent    Mark = "absent"
	Broken    Mark = "broken"
	Awaiting  Mark = "pending"
	// value: your choice not to manage it here (Off here).
	OffHere Mark = "off-here"
	// value: it does not apply to this machine (Not for this machine).
	NotForThisMachine Mark = "not-for-this-machine"
	// value: present but not station's, or not readable now (Not managed, Unknown).
	NotKnown Mark = "unknown"
)

func Marks() [8]Mark {
	return [8]Mark{OK, Attention, Absent, Broken, Awaiting, OffHere, NotForThisMachine, NotKnown}
}

type Reason string

const (
	Outdated    Reason = "outdated"
	Stray       Reason = "stray"
	Unreachable Reason = "unreachable"
	Mismatch    Reason = "mismatch"
	Unmanaged   Reason = "unmanaged"
	Missing     Reason = "missing"
	Partial     Reason = "partial"
	Differs     Reason = "differs"
	Unset       Reason = "unset"
	Stopped     Reason = "stopped"
	Late        Reason = "late"
	Degraded    Reason = "degraded"
	Orphan      Reason = "orphan"
	Waiting     Reason = "waiting"
	Failing     Reason = "failing"
	Refused     Reason = "refused"
	Down        Reason = "down"
	Outlived    Reason = "stale"
	Expiring    Reason = "expiring"
	Guard       Reason = "guard"
	Exposed     Reason = "exposed"
)

type Value struct {
	Kind  Kind    `json:"kind,omitempty"`
	Text  string  `json:"text,omitempty"`
	N     int64   `json:"n,omitempty"`
	Of    int64   `json:"of,omitempty"`
	At    string  `json:"at,omitempty"`
	Unit  string  `json:"unit,omitempty"`
	Label string  `json:"label,omitempty"`
	Items []Value `json:"items,omitempty"`
	Stale bool    `json:"stale,omitempty"`
}

type Kind string

const (
	KindText     Kind = "text"
	KindNone     Kind = "none"
	KindCount    Kind = "count"
	KindSize     Kind = "size"
	KindRatio    Kind = "ratio"
	KindPercent  Kind = "percent"
	KindVersion  Kind = "version"
	KindDuration Kind = "duration"
	KindAge      Kind = "age"
	KindUntil    Kind = "until"
	KindURL      Kind = "url"
	KindIdentity Kind = "identity"
	KindPath     Kind = "path"
	KindPort     Kind = "port"
	KindPortMap  Kind = "portmap"
	KindRange    Kind = "range"
	KindCommand  Kind = "command"
	KindList     Kind = "list"
	KindLevels   Kind = "levels"
	KindOn       Kind = "on"
	KindOff      Kind = "off"
	KindUnknown  Kind = "unknown"
	KindPending  Kind = "pending"
	KindListing  Kind = "listing"
)
