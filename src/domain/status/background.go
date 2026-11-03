package status

import (
	"bytes"
	"context"
	"encoding/xml"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
)

type JobGroup string

const (
	GroupStation JobGroup = "station"
	GroupKnown   JobGroup = "known"
	GroupUnknown JobGroup = "unknown"
	GroupOrphan  JobGroup = "orphan"
)

type JobRun string

const (
	JobRunning   JobRun = "running"
	JobIdle      JobRun = "idle"
	JobFailing   JobRun = "failing"
	JobNotLoaded JobRun = "not loaded"
	JobUnread    JobRun = "unknown"
)

type Job struct {
	Label     string
	Group     JobGroup
	Run       JobRun
	PID       int
	Exit      int
	Scope     string
	Owner     string
	OwnerGone bool
	Plist     string
	Removal   string
	State     catalog.State
}

type launchDir struct {
	dir    string
	scope  string
	system bool
}

func (s *Service) launchDirs() []launchDir {
	return []launchDir{
		{dir: path.Join(s.h.Env.Home(), "Library", "LaunchAgents"), scope: "user"},
		{dir: path.Join("/", s.o.Root, "Library", "LaunchAgents"), scope: "root"},
		{dir: path.Join("/", s.o.Root, "Library", "LaunchDaemons"), scope: "root", system: true},
	}
}

func systemJob(label string) bool {
	for _, p := range catalog.LaunchSystemPrefixes {
		if strings.HasPrefix(label, p) {
			return true
		}
	}
	return false
}

type loaded struct {
	pid  string
	exit string
}

// outside: `launchctl list` prints `<pid or -><TAB><last exit><TAB><label>` per loaded job.
func parseList(text string) map[string]loaded {
	out := map[string]loaded{}
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[0] == "PID" {
			continue
		}
		out[f[2]] = loaded{pid: f[0], exit: f[1]}
	}
	return out
}

// invariant: a station read runs no launchctl verb but `list` and `print`;
// removing a job is the owner's, through the command its row shows.
func (s *Service) background(ctx context.Context) []Job {
	jobs := []Job{}
	if !s.mac() {
		return jobs
	}
	list := s.ask(ctx, cmd("launchctl", "list"), defaultBudget)
	var running map[string]loaded
	if list.ok() {
		running = parseList(list.text())
	}
	seen := map[string]bool{}
	for _, d := range s.launchDirs() {
		entries, err := s.h.FS.ReadDir(d.dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			label, ok := strings.CutSuffix(e.Name(), ".plist")
			if !ok || systemJob(label) || seen[label] {
				continue
			}
			seen[label] = true
			plist := path.Join(d.dir, e.Name())
			j := Job{Label: label, Scope: d.scope, Plist: plist}
			switch {
			case d.system:
				s.printed(ctx, &j)
			case running == nil:
				j.Run = JobUnread
			default:
				l, found := running[label]
				j.state(l, found)
			}
			program := s.programOf(plist)
			j.Plist = s.unrooted(plist)
			s.place(&j, program)
			jobs = append(jobs, j)
		}
	}
	for label, l := range running {
		if seen[label] || systemJob(label) || l.pid == "-" {
			continue
		}
		j := Job{Label: label, Scope: "user"}
		j.state(l, true)
		s.place(&j, "")
		jobs = append(jobs, j)
	}
	slices.SortFunc(jobs, func(a, b Job) int { return strings.Compare(a.Label, b.Label) })
	return jobs
}

func (j *Job) state(l loaded, found bool) {
	switch {
	case !found:
		j.Run = JobNotLoaded
	case l.pid != "-":
		j.Run, j.PID = JobRunning, atoi(l.pid)
	case l.exit != "0" && l.exit != "-":
		j.Run, j.Exit = JobFailing, atoi(l.exit)
	default:
		j.Run = JobIdle
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// outside: `launchctl print system/<label>` answers without root; its
// top-level keys are one tab deep (`\tstate = running`, `\tpid = 529`,
// `\tlast exit code = 1` or `(never exited)`); an unloaded label exits non-zero.
func (s *Service) printed(ctx context.Context, j *Job) {
	r := s.ask(ctx, cmd("launchctl", "print", "system/"+j.Label), defaultBudget)
	if r.late || !r.found {
		j.Run = JobUnread
		return
	}
	if r.out.ExitCode != 0 {
		j.Run = JobNotLoaded
		return
	}
	var state, pid, exit string
	for _, line := range strings.Split(r.text(), "\n") {
		if !strings.HasPrefix(line, "\t") || strings.HasPrefix(line, "\t\t") {
			continue
		}
		k, v, ok := strings.Cut(strings.TrimSpace(line), " = ")
		if !ok {
			continue
		}
		switch k {
		case "state":
			state = v
		case "pid":
			pid = v
		case "last exit code":
			exit = v
		}
	}
	switch {
	case state == "running" && pid != "":
		j.Run, j.PID = JobRunning, atoi(pid)
	case exit != "" && exit != "0" && !strings.HasPrefix(exit, "("):
		j.Run, j.Exit = JobFailing, atoi(exit)
	case state == "":
		j.Run = JobUnread
	default:
		j.Run = JobIdle
	}
}

// outside: an XML plist names its executable as `Program` or the first
// `ProgramArguments`; a binary plist is not read, so its program is unknown.
func programOf(body []byte) string {
	if !bytes.HasPrefix(bytes.TrimSpace(body), []byte("<?xml")) {
		return ""
	}
	d := xml.NewDecoder(bytes.NewReader(body))
	key, inArgs := "", false
	for {
		tok, err := d.Token()
		if err != nil {
			return ""
		}
		t, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch t.Name.Local {
		case "array":
			inArgs = key == "ProgramArguments"
		case "key", "string":
			var text string
			if err := d.DecodeElement(&text, &t); err != nil {
				return ""
			}
			if t.Name.Local == "key" {
				key = text
			} else if key == "Program" || inArgs {
				return text
			}
		}
	}
}

func (s *Service) programOf(plist string) string {
	body, err := s.h.FS.ReadFile(plist)
	if err != nil {
		return ""
	}
	return programOf(body)
}

func (s *Service) exists(p string) bool {
	_, err := s.h.FS.Stat(p)
	return err == nil
}

func (s *Service) place(j *Job, program string) {
	owner, known := catalog.LaunchOwnerOf(j.Label)
	if known {
		j.Owner = owner.App
		j.OwnerGone = owner.Bundle != "" && !s.exists(path.Join("/", s.o.Root, owner.Bundle))
	}
	missing := program != "" && strings.HasPrefix(program, "/") && !s.exists(s.rooted(program))
	switch {
	case strings.HasPrefix(j.Label, catalog.StationLaunchPrefix):
		j.Group = GroupStation
	case j.Plist != "" && (missing || j.OwnerGone):
		j.Group = GroupOrphan
		j.OwnerGone = true
	case known:
		j.Group = GroupKnown
	default:
		j.Group = GroupUnknown
	}
	switch {
	case j.Run == JobFailing:
		j.State = catalog.StateBroken
	case j.Group == GroupOrphan, j.Run == JobUnread:
		j.State = catalog.StateDegraded
	case j.Run == JobNotLoaded && j.Group == GroupStation:
		j.State = catalog.StateDegraded
	case j.Run == JobNotLoaded:
		j.State = catalog.StateAbsent
	default:
		j.State = catalog.StateOK
	}
	if j.Group != GroupStation && (j.Run == JobFailing || j.Group == GroupOrphan) {
		j.Removal = s.removal(*j)
	}
}

// value: a program path under the user's home stays where it is; the rest of
// the machine reads under the system root, as /Applications does.
func (s *Service) rooted(p string) string {
	if strings.HasPrefix(p, s.h.Env.Home()+"/") {
		return p
	}
	return path.Join("/", s.o.Root, p)
}

// value: the path the owner types: the system root dropped, the home as `~`.
func (s *Service) unrooted(p string) string {
	if root := path.Join("/", s.o.Root); root != "/" {
		if rest, ok := strings.CutPrefix(p, root+"/"); ok {
			return "/" + rest
		}
	}
	if rest, ok := strings.CutPrefix(p, s.h.Env.Home()+"/"); ok {
		return "~/" + rest
	}
	return p
}

func (s *Service) removal(j Job) string {
	gui := "gui/" + strconv.Itoa(s.h.Env.Euid()) + "/" + j.Label
	shown := j.Plist
	switch {
	case j.Plist == "":
		return ""
	case strings.HasPrefix(shown, "/Library/LaunchDaemons/"):
		return "sudo launchctl bootout system/" + j.Label + " && sudo rm " + shown
	case strings.HasPrefix(shown, "/Library/LaunchAgents/"):
		return "launchctl bootout " + gui + " && sudo rm " + shown
	}
	return "launchctl bootout " + gui + " && rm " + shown
}
