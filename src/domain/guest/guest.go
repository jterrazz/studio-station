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
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	keysBegin    = "# >>> station hosts >>>"
	keysEnd      = "# <<< station hosts <<<"
	scanEvery    = time.Second
	snapshotEach = 60
	dockerEach   = 5
	relayBudget  = 30 * time.Second
)

type Options struct {
	Root    string
	Paths   settings.Paths
	Kind    catalog.MachineKind
	Version string
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
	OK     bool            `json:"ok,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
	Hello
}

func (s *Service) Check() error {
	if s.o.Kind == catalog.Host {
		return &HostOnlyRefusedError{}
	}
	return nil
}

func (s *Service) SocketPath() string { return path.Join(s.o.Paths.State, "guest.sock") }

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
		g.hello = m.Hello
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

func (s *Service) root(p string) string { return path.Join(s.o.Root, p) }

// invariant: docker is asked every dockerEach scans, and its last answer is
// merged into every scan between, so a container's port keeps its name and
// never flaps (its socket is root's docker-proxy, or none without the proxy).
func (s *Service) scan(ctx context.Context, h Hello, docker *[]Listener, refresh bool) []Listener {
	var found []Listener
	if s.o.Kind == catalog.MacOS {
		out, err := s.h.Run.Output(ctx, system.Cmd{Name: "lsof", Args: []string{"-nP", "-iTCP", "-sTCP:LISTEN", "-Fpcn"}, Timeout: 5 * time.Second})
		if err == nil {
			found = ParseLsof([]byte(out.Stdout))
		}
	} else {
		tcp, _ := s.h.FS.ReadFile(s.root("/proc/net/tcp"))
		tcp6, _ := s.h.FS.ReadFile(s.root("/proc/net/tcp6"))
		found = s.owners(ParseProcNet(tcp, tcp6))
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

func (s *Service) owners(ls []Listener) []Listener {
	byInode := map[string]int{}
	for i, l := range ls {
		if l.inode != "" && l.inode != "0" {
			byInode["socket:["+l.inode+"]"] = i
		}
	}
	if len(byInode) == 0 {
		return ls
	}
	entries, err := s.h.FS.ReadDir(s.root("/proc"))
	if err != nil {
		return ls
	}
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		fds, err := s.h.FS.ReadDir(s.root(path.Join("/proc", e.Name(), "fd")))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := s.h.FS.Readlink(s.root(path.Join("/proc", e.Name(), "fd", fd.Name())))
			if err != nil {
				data, readErr := s.h.FS.ReadFile(s.root(path.Join("/proc", e.Name(), "fd", fd.Name())))
				if readErr != nil {
					continue
				}
				link = strings.TrimSpace(string(data))
			}
			if i, ok := byInode[link]; ok {
				ls[i].PID, ls[i].Process = pid, s.program(e.Name())
			}
		}
	}
	return ls
}

// invariant: a process is named by its program, argv[0]'s base name, since
// comm holds the main thread's name a runtime may rename (node-MainThread);
// an argv[0] rewritten into a title (postgres: checkpointer) leaves comm.
func (s *Service) program(pid string) string {
	cmdline, _ := s.h.FS.ReadFile(s.root(path.Join("/proc", pid, "cmdline")))
	argv0, _, _ := strings.Cut(string(cmdline), "\x00")
	if argv0 != "" && !strings.ContainsAny(argv0, " :") {
		return path.Base(argv0)
	}
	comm, _ := s.h.FS.ReadFile(s.root(path.Join("/proc", pid, "comm")))
	return strings.TrimSpace(string(comm))
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
		case line == keysBegin:
			inside = true
		case line == keysEnd:
			inside = false
		case !inside && line != "":
			kept = append(kept, line)
		}
	}
	lines := append(append(append(kept, keysBegin), keys...), keysEnd)
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
	file := path.Join(s.o.Paths.Config, "services.env")
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
	if err := s.h.FS.MkdirAll(s.o.Paths.State, 0o700); err != nil {
		return err
	}
	return s.h.FS.WriteFile(path.Join(s.o.Paths.State, "guest.json"), append(data, '\n'), 0o600)
}

type Placement struct {
	Host       string `json:"host"`
	VM         string `json:"vm"`
	PublicHost string `json:"public_host"`
}

func (s *Service) Placement() Placement {
	var p Placement
	data, err := s.h.FS.ReadFile(path.Join(s.o.Paths.State, "guest.json"))
	if err == nil {
		_ = json.Unmarshal(data, &p)
	}
	return p
}

func (s *Service) listen(ctx context.Context, g *session, quit context.CancelFunc) system.Listener {
	file := s.SocketPath()
	if conn, err := s.h.Sockets.DialUnix(ctx, file, time.Second); err == nil {
		_, _ = conn.Write([]byte(`{"v":1,"id":"q","op":"quit"}` + "\n"))
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

type request struct {
	V    int    `json:"v"`
	ID   string `json:"id"`
	Op   string `json:"op"`
	Port int    `json:"guest_port"`
}

func (g *session) serve(ctx context.Context, conn system.Conn, quit context.CancelFunc) {
	defer func() { _ = conn.Close() }()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var r request
		if json.Unmarshal(scanner.Bytes(), &r) != nil {
			continue
		}
		if r.Op == "quit" {
			_, _ = conn.Write([]byte(`{"v":1,"id":"` + r.ID + `","ok":true}` + "\n"))
			quit()
			return
		}
		reply := g.relay(ctx, r)
		data, _ := json.Marshal(map[string]any{"v": 1, "id": r.ID, "ok": reply.OK, "result": reply.Result, "error": reply.Error})
		if _, err := conn.Write(append(data, '\n')); err != nil {
			return
		}
	}
}

func (g *session) relay(ctx context.Context, r request) message {
	g.mu.Lock()
	g.seq++
	id := "g" + strconv.Itoa(g.seq)
	ch := make(chan message, 1)
	g.pending[id] = ch
	g.mu.Unlock()
	g.send(map[string]any{"v": 1, "type": "request", "id": id, "op": r.Op, "port": r.Port})
	ctx, cancel := context.WithTimeout(ctx, relayBudget)
	defer cancel()
	select {
	case m := <-ch:
		return m
	case <-ctx.Done():
		return message{Error: json.RawMessage(`{"code":"internal","message":"the host did not answer","hint":"station services"}`)}
	}
}
