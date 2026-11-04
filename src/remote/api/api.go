package api

import (
	"encoding/json"
	"maps"

	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
)

const (
	Name    = "station.api/v2"
	Version = 2
)

type Frame struct {
	// invariant: one JSON object per line; a request carries call or watch,
	// an answer carries progress, out or error, under the request's id. A
	// call ends at its out or its error; a watch sends an out per update.
	V        int             `json:"v"`
	ID       string          `json:"id"`
	Call     string          `json:"call,omitempty"`
	Watch    string          `json:"watch,omitempty"`
	Target   *Target         `json:"target,omitempty"`
	In       json.RawMessage `json:"in,omitempty"`
	Progress *Step           `json:"progress,omitempty"`
	Out      json.RawMessage `json:"out,omitempty"`
	Error    *Fault          `json:"error,omitempty"`
}

type Target struct {
	Name string `json:"name"`
	On   string `json:"on,omitempty"`
}

func TargetOf(r machine.Ref) *Target {
	if r == (machine.Ref{}) {
		return nil
	}
	return &Target{Name: r.Name, On: r.On}
}

type Step struct {
	Task   string `json:"task"`
	Detail string `json:"detail,omitempty"`
	Done   bool   `json:"done,omitempty"`
	Failed string `json:"failed,omitempty"`
}

type Fault struct {
	Type    string            `json:"type"`
	Code    string            `json:"code"`
	Subject string            `json:"subject,omitempty"`
	Detail  map[string]string `json:"detail,omitempty"`
	Message string            `json:"message"`
	Hint    string            `json:"hint,omitempty"`
}

func (f *Fault) Error() string { return f.Message }

func FaultOf(e *fail.Error) *Fault {
	t := e.Type
	if t == "" {
		t = fail.Failed
		if e.Exit == fail.ExitUsage {
			t = fail.Refused
		}
	}
	return &Fault{Type: string(t), Code: string(e.Code), Subject: e.Subject, Detail: maps.Clone(e.Detail), Message: e.Message, Hint: e.Hint}
}

func (f *Fault) Err() *fail.Error {
	// invariant: a fault comes back as the error it was raised as: its type
	// decides the exit code, and its words are the far side's when it sent
	// them, else this binary's table words its code.
	return &fail.Error{Type: fail.Type(f.Type), Code: fail.Code(f.Code), Subject: f.Subject, Detail: maps.Clone(f.Detail), Message: f.Message, Hint: f.Hint}
}

type Down struct{}

func (*Down) Error() string { return "the station engine is not running" }
