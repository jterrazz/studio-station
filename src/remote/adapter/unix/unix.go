package unix

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/fail"
	"github.com/jterrazz/studio-station/src/core/machine"
	"github.com/jterrazz/studio-station/src/remote/api"
	"github.com/jterrazz/studio-station/src/remote/contract"
)

// Guest is the guest's own API, over the one connection its Mac's engine holds.
const Guest = "/usr/local/bin/station guest serve --stdio"

type Engine interface {
	// value: runs line inside vm over the engine's own connection to it,
	// stdin its input; the engine is reached over its unix socket.
	GuestExec(ctx context.Context, vm, line, stdin string) (stdout string, code int, err error)
}

type Dialer struct {
	engine Engine
	// value: this Mac's own Hello: its engine is this binary.
	here func() contract.Hello
}

func New(engine Engine, here func() contract.Hello) Dialer { return Dialer{engine: engine, here: here} }

func (d Dialer) Dial(context.Context, string) (contract.Endpoint, error) {
	return endpoint(d), nil
}

type endpoint Dialer

func (e endpoint) Hello(context.Context) (contract.Hello, error) { return e.here(), nil }

func (e endpoint) Close() error { return nil }

func (e endpoint) Invoke(ctx context.Context, target machine.Ref, name string, in json.RawMessage, p call.Progress) (json.RawMessage, error) {
	frames, err := e.exchange(ctx, target, api.Frame{Call: name, Target: api.TargetOf(target), In: in})
	if err != nil {
		return nil, err
	}
	for _, f := range frames {
		switch {
		case f.Progress != nil:
			api.Report(p, f.Progress)
		case f.Error != nil:
			return nil, f.Error.Err()
		default:
			return f.Out, nil
		}
	}
	return nil, lost(target, errors.New("the guest answered nothing"))
}

func (e endpoint) Watch(ctx context.Context, target machine.Ref, name string, in json.RawMessage, p call.Progress) (<-chan call.Update[json.RawMessage], error) {
	frames, err := e.exchange(ctx, target, api.Frame{Watch: name, Target: api.TargetOf(target), In: in})
	if err != nil {
		return nil, err
	}
	out := make(chan call.Update[json.RawMessage], len(frames))
	for _, f := range frames {
		switch {
		case f.Progress != nil:
			api.Report(p, f.Progress)
		case f.Error != nil:
			out <- call.Update[json.RawMessage]{Err: f.Error.Err()}
		case !f.Done:
			out <- call.Update[json.RawMessage]{Value: f.Out}
		}
	}
	close(out)
	return out, nil
}

// invariant: one exchange per call: the guest's server reads the hello and
// the request to the end of its stdin, answers both and exits; a guest whose
// binary serves no such call is too old, read again at every exchange.
func (e endpoint) exchange(ctx context.Context, target machine.Ref, request api.Frame) ([]api.Frame, error) {
	request.V, request.ID = api.Version, "2"
	hello, _ := json.Marshal(api.Frame{V: api.Version, ID: "1", Hello: &api.Hello{}})
	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	stdout, code, err := e.engine.GuestExec(ctx, target.Name, Guest, string(hello)+"\n"+string(body)+"\n")
	if err != nil {
		return nil, err
	}
	var answers []api.Frame
	var far *api.Hello
	s := bufio.NewScanner(strings.NewReader(stdout))
	s.Buffer(make([]byte, 0, 64<<10), 64<<20)
	for s.Scan() {
		var f api.Frame
		if json.Unmarshal(s.Bytes(), &f) != nil {
			continue
		}
		switch {
		case f.ID == "1" && f.Hello != nil:
			far = f.Hello
		case f.ID == "2":
			answers = append(answers, f)
		}
	}
	name := request.Call + request.Watch
	switch {
	case far == nil:
		return nil, lost(target, errors.New("the guest's station did not answer, exit "+strconv.Itoa(code)))
	case !slices.Contains(far.Calls, name):
		return nil, fail.New(fail.FarTooOld, call.FarTooOld, target.Name, map[string]string{"version": far.Version, "call": name})
	}
	return answers, nil
}

func lost(target machine.Ref, cause error) error {
	e := fail.New(fail.Unreachable, call.Unreachable, target.Name, nil)
	e.Cause = cause
	return e
}
