package overview

import (
	"path"
	"strings"

	"github.com/jterrazz/studio-station/src/background"
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
	Program   string
	Removal   string
	State     catalog.State
}

// invariant: a station read only reads the job manager; removing a job is
// the owner's, through the command its row shows. root and home turn a path
// into the one the owner types.
func Jobs(listed []background.Job, root, home string) []Job {
	jobs := []Job{}
	for _, l := range listed {
		j := Job{Label: l.Label, Scope: l.Scope, PID: l.PID, Plist: unrooted(l.File, root, home), Program: unrooted(l.Program, root, home), Removal: l.Removal}
		switch {
		case l.Unread:
			j.Run = JobUnread
		case !l.Loaded:
			j.Run = JobNotLoaded
		case l.Running:
			j.Run = JobRunning
		case l.LastExit != 0:
			j.Run, j.Exit = JobFailing, l.LastExit
		default:
			j.Run = JobIdle
		}
		place(&j, l)
		jobs = append(jobs, j)
	}
	return jobs
}

func place(j *Job, l background.Job) {
	owner, known := catalog.LaunchOwnerOf(j.Label)
	if known {
		j.Owner = owner.App
		j.OwnerGone = !l.OwnerExists
	}
	switch {
	case strings.HasPrefix(j.Label, catalog.StationLaunchPrefix):
		j.Group = GroupStation
	case j.Plist != "" && (!l.ProgramExists || j.OwnerGone):
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
	if j.Group == GroupStation || (j.Run != JobFailing && j.Group != GroupOrphan) {
		j.Removal = ""
	}
}

// value: the path the owner types: the system root dropped, the home as `~`.
func unrooted(p, root, home string) string {
	if root := path.Join("/", root); root != "/" {
		if rest, ok := strings.CutPrefix(p, root+"/"); ok {
			return "/" + rest
		}
	}
	if rest, ok := strings.CutPrefix(p, home+"/"); ok {
		return "~/" + rest
	}
	return p
}
