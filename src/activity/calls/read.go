package calls

import (
	"context"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/finding"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/core/section"
)

type ReadIn struct {
	// value: a VM of this Mac whose history its engine keeps, by name;
	// empty reads this machine.
	Of string `json:"of,omitempty"`
	// value: the machine as the reader names it, and its kind: what the
	// findings' fixes say; empty is this machine.
	Machine machine.Ref  `json:"machine,omitzero"`
	Kind    machine.Kind `json:"kind,omitempty"`
	// value: the 24 h series of the engine that keeps it, for scripts.
	History bool `json:"history,omitempty"`
	// value: what the engine keeps, else one sample now: a read that never
	// waits (an Overview's line).
	Quick bool `json:"quick,omitempty"`
}

var Read = call.Watch[Deps, ReadIn, activity.Report]{
	Name: "activity.read", Section: section.Activity, Where: call.AtTarget, Applies: activity.Applies,
	Run: func(ctx context.Context, d Deps, in ReadIn, _ call.Progress) (<-chan call.Update[activity.Report], error) {
		out := make(chan activity.Report, 1)
		out <- d.read(ctx, in)
		close(out)
		return call.Updates(ctx, out), nil
	},
}

func (d Deps) read(ctx context.Context, in ReadIn) activity.Report {
	// invariant: SPEC §6.2: the history of the engine that keeps it when it
	// spans a rate; else, on the machine itself, two samples 1 s apart. A VM
	// this Mac's engine keeps nothing of is NotKept: its reader asks the VM.
	ref, kind, name := in.Machine, in.Kind, in.Of
	if ref.Name == "" {
		ref.Name = d.Name
	}
	if kind == "" {
		kind = d.Kind
	}
	if name == "" {
		name = d.Name
	}
	if d.Kept != nil {
		if k, since, ok, err := d.Kept(ctx, name); err == nil && ok {
			if r, usable := fromHistory(k, since, in); usable {
				r.Findings = findings(ref, kind, &k.Series)
				return r
			}
		}
	}
	if in.Of != "" {
		return activity.Report{Source: activity.NotKept, Findings: []finding.Finding{}}
	}
	return d.direct(ctx, ref, kind, in.Quick)
}

func fromHistory(k activity.Kept, since time.Time, in ReadIn) (activity.Report, bool) {
	r := activity.Report{Source: activity.FromEngine, Since: since, Last: k.Last}
	if k.Last.Run != nil {
		r.Source = activity.FromMacProcess
		return r, true
	}
	r.Rates, r.Window = activity.Recent(&k.Series)
	if in.History {
		r.History = k.Series.Coarse()
	}
	return r, r.Window > 0
}

func (d Deps) direct(ctx context.Context, ref machine.Ref, kind machine.Kind, quick bool) activity.Report {
	r := activity.Report{Source: activity.FromDirect, Findings: []finding.Finding{}}
	first, err := d.budgeted(ctx)
	if err != nil {
		r.Silent = true
		return r
	}
	r.Last = first
	points := []activity.Sample{first.Sample}
	if !quick && d.Sleep != nil && d.Sleep(ctx, activity.DirectGap) == nil {
		second, err := d.budgeted(ctx)
		if err != nil {
			r.Silent = true
			return r
		}
		r.Last = second
		points = append(points, second.Sample)
		r.Rates, r.Window = activity.Between(first.Sample, second.Sample), second.Sample.At.Sub(first.Sample.At)
	}
	s := activity.SeriesOf(nil, points)
	r.Findings = findings(ref, kind, &s)
	return r
}

func (d Deps) budgeted(ctx context.Context) (activity.Reading, error) {
	ctx, cancel := context.WithTimeout(ctx, activity.Budget)
	defer cancel()
	return compose(ctx, d)
}

func findings(ref machine.Ref, kind machine.Kind, s *activity.Series) []finding.Finding {
	out := activity.Findings(ref, kind, s)
	if out == nil {
		return []finding.Finding{}
	}
	return out
}
