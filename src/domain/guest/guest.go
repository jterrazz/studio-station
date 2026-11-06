package guest

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	KeysBegin    = "# >>> station hosts >>>"
	KeysEnd      = "# <<< station hosts <<<"
	scanEvery    = time.Second
	snapshotEach = 60
	dockerEach   = 5
	relayBudget  = 30 * time.Second
	readBudget   = 10 * time.Second
)

type Options struct {
	// value: the guest's own state folder and the services' env file.
	State, ServicesEnv string
	Kind               catalog.MachineKind
	Version            string
	// value: what the host's `accounts` request is answered with: this VM's
	// providers, parsed into identities and scopes, never a token.
	Accounts func(ctx context.Context, keys []string) any
	// value: what listens in this VM, read by the adapter of its kind.
	Listeners Listeners
}

type Listeners interface {
	Read(ctx context.Context) ([]ports.Listener, error)
}

type Service struct {
	h system.Host
	o Options
}

func New(h system.Host, o Options) *Service { return &Service{h: h, o: o} }

type HostOnlyRefusedError struct{}

func (*HostOnlyRefusedError) Error() string { return "station guest runs only inside a VM" }

type Hello struct {
	Host       string            `json:"host"`
	VM         string            `json:"vm"`
	PublicHost string            `json:"public_host"`
	Keys       []string          `json:"keys"`
	Env        map[string]string `json:"env"`
	Ignored    []int             `json:"ignored"`
	MinPort    int               `json:"min_port"`
}

type message struct {
	V      int             `json:"v"`
	Type   string          `json:"type"`
	ID     string          `json:"id,omitempty"`
	Op     string          `json:"op,omitempty"`
	OK     bool            `json:"ok,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
	Hello
}

func (s *Service) Check() error {
	if s.o.Kind == catalog.Mac {
		return &HostOnlyRefusedError{}
	}
	return nil
}

func (s *Service) SocketPath() string { return path.Join(s.o.State, "guest.sock") }

type session struct {
	s       *Service
	out     io.Writer
	mu      sync.Mutex
	hello   Hello
	ready   bool
	last    []Listener
	docker  []Listener
	pending map[string]chan message
	seq     int
}

func (g *session) send(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	_, _ = g.out.Write(append(data, '\n'))
}

func (s *Service) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	// invariant: the guest never opens a connection to its host: its stdin and
	// stdout ARE the host's channel, and a stdin that ends ends the guest.
	if err := s.Check(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	g := &session{s: s, out: out, pending: map[string]chan message{}}
	lines := make(chan message)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(in)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			var m message
			if json.Unmarshal(scanner.Bytes(), &m) == nil {
				select {
				case lines <- m:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	sock := s.listen(ctx, g, cancel)
	defer func() {
		if sock != nil {
			_ = sock.Close()
		}
		_ = s.h.FS.Remove(s.SocketPath())
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case m, ok := <-lines:
			if !ok {
				return nil
			}
			g.handle(ctx, m)
		}
	}
}

func (g *session) handle(ctx context.Context, m message) {
	switch m.Type {
	case "hello":
		g.mu.Lock()
		g.hello = m.Hello
		g.mu.Unlock()
		_ = g.s.writeFiles(m.Hello)
		if !g.ready {
			g.ready = true
			g.send(map[string]any{"v": 1, "type": "ready", "guest_version": g.s.o.Version, "kind": string(g.s.o.Kind)})
			g.last = g.s.scan(ctx, g.hello, &g.docker, true)
			g.send(map[string]any{"v": 1, "type": "snapshot", "ports": g.last})
			go g.watch(ctx)
		}
	case "ping":
		g.send(map[string]any{"v": 1, "type": "pong"})
	case "request":
		go g.answer(ctx, m)
	case "reply":
		g.mu.Lock()
		ch := g.pending[m.ID]
		delete(g.pending, m.ID)
		g.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	}
}

// invariant: a host's request is answered on the same channel, with only the
// parsed result: the providers' own output never leaves this process.
func (g *session) answer(ctx context.Context, m message) {
	if m.Op != "accounts" || g.s.o.Accounts == nil {
		g.send(map[string]any{"v": 1, "type": "reply", "id": m.ID, "ok": false, "error": map[string]string{"code": "unknown_op", "message": "unknown op \"" + m.Op + "\"", "hint": "station accounts"}})
		return
	}
	ctx, cancel := context.WithTimeout(ctx, readBudget)
	defer cancel()
	g.mu.Lock()
	keys := g.hello.Keys
	g.mu.Unlock()
	g.send(map[string]any{"v": 1, "type": "reply", "id": m.ID, "ok": true, "result": g.s.o.Accounts(ctx, keys)})
}

func (g *session) watch(ctx context.Context) {
	for tick := 1; ; tick++ {
		if g.s.h.Clock.Sleep(ctx, scanEvery) != nil {
			return
		}
		now := g.s.scan(ctx, g.hello, &g.docker, tick%dockerEach == 0)
		if tick%snapshotEach == 0 {
			g.send(map[string]any{"v": 1, "type": "snapshot", "ports": now})
		} else {
			for _, e := range Diff(g.last, now) {
				if e.Listen != nil {
					g.send(struct {
						V    int    `json:"v"`
						Type string `json:"type"`
						Listener
					}{1, "listen", *e.Listen})
				} else {
					g.send(map[string]any{"v": 1, "type": "unlisten", "port": e.Unlisten})
				}
			}
		}
		g.last = now
	}
}

// invariant: docker is asked every dockerEach scans, and its last answer is
// merged into every scan between, so a container's port keeps its name and
// never flaps (its socket is root's docker-proxy, or none without the proxy).
func (s *Service) scan(ctx context.Context, h Hello, docker *[]Listener, refresh bool) []Listener {
	var found []Listener
	if s.o.Listeners != nil {
		if ls, err := s.o.Listeners.Read(ctx); err == nil {
			found = Sockets(ls)
		}
	}
	if refresh {
		if _, ok := s.h.Run.LookPath("docker"); !ok {
			*docker = nil
		} else if out, err := s.h.Run.Output(ctx, system.Cmd{Name: "docker", Args: []string{"ps", "--format", "{{json .}}"}, Timeout: 5 * time.Second}); err == nil && out.ExitCode == 0 {
			*docker = ParseDockerPS([]byte(out.Stdout))
		}
	}
	found = Merge(found, *docker)
	return Filter(found, h.MinPort, h.Ignored)
}

func (s *Service) writeFiles(h Hello) error {
	home := s.h.Env.Home()
	return errors.Join(s.writeKeys(path.Join(home, ".ssh", "authorized_keys"), h.Keys), s.writeEnv(h.Env), s.writeHost(h))
}

func (s *Service) writeKeys(file string, keys []string) error {
	current, _ := s.h.FS.ReadFile(file)
	var kept []string
	inside := false
	for _, line := range strings.Split(strings.TrimRight(string(current), "\n"), "\n") {
		switch {
		case line == KeysBegin:
			inside = true
		case line == KeysEnd:
			inside = false
		case !inside && line != "":
			kept = append(kept, line)
		}
	}
	lines := append(append(append(kept, KeysBegin), keys...), KeysEnd)
	if err := s.h.FS.MkdirAll(path.Dir(file), 0o700); err != nil {
		return err
	}
	if err := s.h.FS.WriteFile(file, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return err
	}
	return s.h.FS.Chmod(file, 0o600)
}

func Quote(v string) string { return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'" }

func (s *Service) writeEnv(env map[string]string) error {
	names := make([]string, 0, len(env))
	for k := range env {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, k := range names {
		b.WriteString("export " + k + "=" + Quote(env[k]) + "\n")
	}
	file := s.o.ServicesEnv
	if err := s.h.FS.MkdirAll(path.Dir(file), 0o700); err != nil {
		return err
	}
	return s.h.FS.WriteFile(file, []byte(b.String()), 0o644)
}

func (s *Service) writeHost(h Hello) error {
	data, err := json.MarshalIndent(map[string]string{"host": h.Host, "vm": h.VM, "public_host": h.PublicHost}, "", "  ")
	if err != nil {
		return err
	}
	if err := s.h.FS.MkdirAll(s.o.State, 0o700); err != nil {
		return err
	}
	return s.h.FS.WriteFile(path.Join(s.o.State, "guest.json"), append(data, '\n'), 0o600)
}

type Placement struct {
	Host       string `json:"host"`
	VM         string `json:"vm"`
	PublicHost string `json:"public_host"`
}

func (s *Service) Placement() Placement {
	var p Placement
	data, err := s.h.FS.ReadFile(path.Join(s.o.State, "guest.json"))
	if err == nil {
		_ = json.Unmarshal(data, &p)
	}
	return p
}

func (s *Service) listen(ctx context.Context, g *session, quit context.CancelFunc) system.Listener {
	file := s.SocketPath()
	if conn, err := s.h.Sockets.DialUnix(ctx, file, time.Second); err == nil {
		_, _ = conn.Write([]byte(`{"v":2,"id":"q","call":"` + callQuit + `"}` + "\n"))
		_, _ = bufio.NewReader(conn).ReadString('\n')
		_ = conn.Close()
	}
	_ = s.h.FS.Remove(file)
	if err := s.h.FS.MkdirAll(path.Dir(file), 0o700); err != nil {
		return nil
	}
	l, err := s.h.Sockets.ListenUnix(file)
	if err != nil {
		return nil
	}
	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			go g.serve(ctx, conn, quit)
		}
	}()
	return l
}

// value: the calls the socket inside a VM serves, in station.api/v2 frames:
// the three a command here asks its Mac's engine, relayed over the guest
// channel, and the quit a newer guest sends the one it replaces.
const (
	callServices = "engine.services"
	callReserve  = "engine.reserve"
	callRelease  = "engine.release"
	callQuit     = "guest.quit"
)

type request struct {
	V    int    `json:"v"`
	ID   string `json:"id"`
	Call string `json:"call"`
	In   struct {
		Port int `json:"guest_port"`
	} `json:"in"`
}

type fault struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

func (g *session) serve(ctx context.Context, conn system.Conn, quit context.CancelFunc) {
	defer func() { _ = conn.Close() }()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var r request
		if json.Unmarshal(scanner.Bytes(), &r) != nil {
			continue
		}
		if r.Call == callQuit {
			_, _ = conn.Write([]byte(`{"v":2,"id":"` + r.ID + `","out":null}` + "\n"))
			quit()
			return
		}
		frame := map[string]any{"v": 2, "id": r.ID}
		if op, ok := map[string]string{callServices: "ports", callReserve: "reserve", callRelease: "release"}[r.Call]; !ok || r.V != 2 {
			frame["error"] = fault{Type: "refused", Code: "call.unknown", Message: "no call is named " + r.Call, Hint: "station services"}
		} else if reply := g.relay(ctx, op, r.In.Port); reply.OK {
			frame["out"] = reply.Result
		} else {
			var f fault
			_ = json.Unmarshal(reply.Error, &f)
			f.Type = "failed"
			if strings.HasPrefix(f.Code, "unknown_") {
				f.Type = "refused"
			}
			frame["error"] = f
		}
		data, _ := json.Marshal(frame)
		if _, err := conn.Write(append(data, '\n')); err != nil {
			return
		}
	}
}

func (g *session) relay(ctx context.Context, op string, port int) message {
	g.mu.Lock()
	g.seq++
	id := "g" + strconv.Itoa(g.seq)
	ch := make(chan message, 1)
	g.pending[id] = ch
	g.mu.Unlock()
	g.send(map[string]any{"v": 1, "type": "request", "id": id, "op": op, "port": port})
	ctx, cancel := context.WithTimeout(ctx, relayBudget)
	defer cancel()
	select {
	case m := <-ch:
		return m
	case <-ctx.Done():
		return message{Error: json.RawMessage(`{"code":"internal","message":"the host did not answer","hint":"station services"}`)}
	}
}
