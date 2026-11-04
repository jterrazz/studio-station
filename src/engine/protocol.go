package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"path"
	"strconv"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	Schema        = "station.daemon/v1"
	connectBudget = 2 * time.Second
)

type Request struct {
	V         int      `json:"v"`
	ID        string   `json:"id"`
	Op        string   `json:"op"`
	VM        string   `json:"vm,omitempty"`
	GuestPort int      `json:"guest_port,omitempty"`
	Pinned    bool     `json:"pinned,omitempty"`
	Profile   string   `json:"profile,omitempty"`
	Pages     []string `json:"pages,omitempty"`
	Lid       bool     `json:"lid,omitempty"`
	Battery   bool     `json:"battery,omitempty"`
	For       int      `json:"for,omitempty"`
	By        string   `json:"by,omitempty"`
	Line      string   `json:"line,omitempty"`
	// invariant: what a command inside a VM reads, a secret included: it travels
	// in the request alone, never in Line.
	Stdin string `json:"stdin,omitempty"`
}

type OpError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (e *OpError) Error() string { return e.Message }

type Response struct {
	V      int             `json:"v"`
	ID     string          `json:"id"`
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *OpError        `json:"error,omitempty"`
}

type Ping struct {
	Version    string    `json:"version"`
	PID        int       `json:"pid"`
	StartedAt  time.Time `json:"started_at"`
	Executable string    `json:"executable,omitempty"`
}

type VMView struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	RunPID int    `json:"run_pid"`
	Guest  string `json:"guest"`
	IP     string `json:"ip"`
	Detail string `json:"detail"`
}

type DownError struct{}

func (*DownError) Error() string { return "the station daemon is not running" }

type RunningError struct{ PID int }

func (e *RunningError) Error() string {
	return "a station daemon already runs (pid " + strconv.Itoa(e.PID) + ")"
}

type HostOnlyError struct{ Here catalog.MachineKind }

func (e *HostOnlyError) Error() string {
	return "the daemon runs only on a host; here is " + e.Here.Article()
}

func SocketPath(p settings.Paths) string { return path.Join(p.State, "daemon.sock") }

func ChildrenDir(p settings.Paths) string { return path.Join(p.State, "children") }

type Client struct {
	sockets system.Sockets
	path    string
}

func Dial(h system.Host, p settings.Paths) Client {
	return Client{sockets: h.Sockets, path: SocketPath(p)}
}

func DialPath(h system.Host, file string) Client { return Client{sockets: h.Sockets, path: file} }

func (c Client) Ping(ctx context.Context) (Ping, error) {
	var p Ping
	err := c.Do(ctx, "ping", Request{}, &p)
	return p, err
}

func (c Client) Do(ctx context.Context, op string, args Request, out any) error {
	// invariant: a socket that is absent, refuses, or does not answer within the
	// connect budget is a daemon that is down: DownError, never another error.
	cctx, cancel := context.WithTimeout(ctx, connectBudget)
	conn, err := c.sockets.DialUnix(cctx, c.path, connectBudget)
	cancel()
	if err != nil {
		return &DownError{}
	}
	defer func() { _ = conn.Close() }()
	args.V, args.ID, args.Op = 1, "1", op
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	if _, err := conn.Write(append(data, '\n')); err != nil {
		return &DownError{}
	}
	type answer struct {
		line []byte
		err  error
	}
	got := make(chan answer, 1)
	go func() {
		line, err := bufio.NewReader(conn).ReadBytes('\n')
		got <- answer{line, err}
	}()
	var a answer
	select {
	case <-ctx.Done():
		return ctx.Err()
	case a = <-got:
	}
	if a.err != nil {
		if op == "ping" {
			return &DownError{}
		}
		return a.err
	}
	var resp Response
	if err := json.Unmarshal(a.line, &resp); err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error == nil {
			return &OpError{Code: "internal", Message: "the daemon refused " + op}
		}
		return resp.Error
	}
	if out != nil && len(resp.Result) > 0 {
		return json.Unmarshal(resp.Result, out)
	}
	return nil
}

const Label = catalog.DaemonLabel

func (c Client) Subscribe(ctx context.Context, pages []string) (<-chan json.RawMessage, error) {
	// invariant: a subscription is its own connection, held until ctx ends or
	// the daemon goes; each page event's page is sent as the daemon wrote it,
	// and the channel closes when the stream does.
	cctx, cancel := context.WithTimeout(ctx, connectBudget)
	conn, err := c.sockets.DialUnix(cctx, c.path, connectBudget)
	cancel()
	if err != nil {
		return nil, &DownError{}
	}
	data, err := json.Marshal(Request{V: 1, ID: "1", Op: "subscribe", Pages: pages})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, werr := conn.Write(append(data, '\n')); werr != nil {
		_ = conn.Close()
		return nil, &DownError{}
	}
	r := bufio.NewReader(conn)
	line, err := r.ReadBytes('\n')
	if err != nil {
		_ = conn.Close()
		return nil, &DownError{}
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil || !resp.OK {
		_ = conn.Close()
		if resp.Error != nil {
			return nil, resp.Error
		}
		return nil, &OpError{Code: "internal", Message: "the daemon refused subscribe"}
	}
	out := make(chan json.RawMessage)
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	go func() {
		defer close(out)
		for {
			line, err := r.ReadBytes('\n')
			if err != nil {
				return
			}
			var ev Event
			if json.Unmarshal(line, &ev) != nil || ev.Event != "page" {
				continue
			}
			select {
			case out <- ev.Page:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}
