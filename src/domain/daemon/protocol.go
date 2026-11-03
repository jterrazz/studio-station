package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"path"
	"sort"
	"strconv"
	"strings"
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
	V         int    `json:"v"`
	ID        string `json:"id"`
	Op        string `json:"op"`
	VM        string `json:"vm,omitempty"`
	GuestPort int    `json:"guest_port,omitempty"`
	Pinned    bool   `json:"pinned,omitempty"`
	Profile   string `json:"profile,omitempty"`
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
	Version   string    `json:"version"`
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
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

func RunCmd(argv []string, exitFile string) system.Cmd {
	// invariant: a VM's run is detached from the daemon, and its exit code is written
	// by its own wrapper, so a daemon restart reads the end of a run it never waited on.
	return system.Cmd{Name: "sh", Args: append([]string{"-c", `"$@"; echo "$?" >"` + exitFile + `"`, "sh"}, argv...)}
}

const Label = catalog.DaemonLabel

func Plist(executable string, env map[string]string, logPath string) string {
	// invariant: one agent per host; KeepAlive restarts the daemon after any exit,
	// and AbandonProcessGroup lets the VMs' runs outlive it.
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	b.WriteString("\t<key>Label</key>\n\t<string>" + Label + "</string>\n")
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n\t\t<string>" + xmlText(executable) + "</string>\n\t\t<string>daemon</string>\n\t</array>\n")
	b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
	for _, k := range keys {
		b.WriteString("\t\t<key>" + xmlText(k) + "</key>\n\t\t<string>" + xmlText(env[k]) + "</string>\n")
	}
	b.WriteString("\t</dict>\n")
	b.WriteString("\t<key>RunAtLoad</key>\n\t<true/>\n\t<key>KeepAlive</key>\n\t<true/>\n\t<key>AbandonProcessGroup</key>\n\t<true/>\n")
	b.WriteString("\t<key>StandardOutPath</key>\n\t<string>" + xmlText(logPath) + "</string>\n")
	b.WriteString("\t<key>StandardErrorPath</key>\n\t<string>" + xmlText(logPath) + "</string>\n")
	b.WriteString("</dict>\n</plist>\n")
	return b.String()
}

func xmlText(v string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;").Replace(v)
}
