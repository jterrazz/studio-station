package tailnet

import (
	"context"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	readyBudget   = 4 * time.Second
	readyInterval = 250 * time.Millisecond
)

type UpResult struct {
	Daemon    Daemon
	KeepAwake bool
}

type DownResult struct{ Stopped []Daemon }

func (s *Service) Configured(ctx context.Context) bool {
	if s.registered() {
		return true
	}
	cfg, err := s.store.Load()
	if err != nil || cfg.Tailnet.AuthMethod != settings.AuthAuthKey {
		return false
	}
	_, found, err := s.keys.Read(ctx, hostAuthKeyAccount)
	return err == nil && found
}

func (s *Service) registered() bool {
	_, err := s.h.FS.Stat(s.stateFile())
	return err == nil
}

func (s *Service) Up(ctx context.Context) (UpResult, error) {
	cfg, err := s.store.Load()
	if err != nil {
		return UpResult{}, err
	}
	for _, tool := range []string{"tailscaled", "tailscale"} {
		if _, ok := s.h.Run.LookPath(tool); !ok {
			return UpResult{}, &NotInstalledError{Missing: tool}
		}
	}
	key := ""
	if cfg.Tailnet.AuthMethod == settings.AuthAuthKey {
		value, found, keyErr := s.keys.Read(ctx, hostAuthKeyAccount)
		if keyErr != nil {
			return UpResult{}, keyErr
		}
		if found {
			key = value
		}
	}
	firstLogin := !s.registered() && key == ""
	if firstLogin && (cfg.Tailnet.AuthMethod != settings.AuthLogin || !s.o.Interactive) {
		return UpResult{}, &NeedsAuthError{}
	}
	if err := s.StartDaemon(ctx); err != nil {
		return UpResult{}, err
	}

	args := []string{"up"}
	if cfg.Here != "" {
		args = append(args, "--hostname="+cfg.Here)
	}
	stdin := ""
	if key != "" {
		args = append(args, "--auth-key=file:/dev/stdin")
		stdin = key
	}
	if err := s.up(ctx, args, stdin, firstLogin); err != nil {
		return UpResult{}, err
	}
	return UpResult{Daemon: Userspace, KeepAwake: s.keepAwakeStart()}, nil
}

// hazard: a first login blocks on the owner's browser and prints its URL on the
// way, so it runs attached; every other run is captured, because the retry
// below reads tailscale's refusal.
func (s *Service) up(ctx context.Context, args []string, stdin string, attached bool) error {
	cmd := system.Cmd{Name: "tailscale", Args: s.daemonArgv(Userspace, args...), Stdin: stdin}
	if attached {
		ran, err := s.h.Run.Attach(ctx, cmd, system.ToStderr)
		code := ran.ExitCode
		if err != nil {
			return err
		}
		if code != 0 {
			return &AuthError{Detail: "tailscale up exited " + strconv.Itoa(code)}
		}
		return nil
	}
	out, err := s.h.Run.Output(ctx, cmd)
	if err != nil {
		return err
	}
	if out.ExitCode == 0 {
		return nil
	}
	text := out.Stdout + "\n" + out.Stderr
	// outside: tailscale up refuses to drop a non-default preference; retrying with
	// the flags it suggests keeps them.
	if nonDefaultFlags.MatchString(strings.ToLower(text)) {
		if suggested := suggestedFlags(text); len(suggested) > 0 {
			cmd.Args = s.daemonArgv(Userspace, mergeFlags(args, suggested)...)
			retry, err := s.h.Run.Output(ctx, cmd)
			if err != nil {
				return err
			}
			if retry.ExitCode == 0 {
				return nil
			}
			text = retry.Stdout + "\n" + retry.Stderr
		}
	}
	return &AuthError{Detail: failureLine(text)}
}

func (s *Service) Down(ctx context.Context) (DownResult, error) {
	var result DownResult
	if s.userspaceRunning(ctx) {
		out, err := s.h.Run.Output(ctx, system.Cmd{Name: "tailscale", Args: s.daemonArgv(Userspace, "down")})
		s.keepAwakeStop()
		if stopErr := s.StopDaemon(ctx); stopErr != nil {
			return result, stopErr
		}
		if err != nil {
			return result, err
		}
		if out.ExitCode != 0 {
			return result, &DaemonError{Detail: "tailscale down failed: " + failureLine(out.Stdout+"\n"+out.Stderr)}
		}
		result.Stopped = append(result.Stopped, Userspace)
	} else {
		// hazard: the daemon can be gone and its keep-awake guard left behind.
		s.keepAwakeStop()
	}
	if s.mode() != settings.ModeAuto {
		return result, nil
	}
	if st, err := s.query(ctx, System); err == nil && st.BackendState == "Running" {
		out, err := s.h.Run.Output(ctx, system.Cmd{Name: "tailscale", Args: []string{"down"}})
		if err != nil {
			return result, err
		}
		if out.ExitCode != 0 {
			return result, &DaemonError{Detail: "tailscale down failed: " + failureLine(out.Stdout+"\n"+out.Stderr)}
		}
		result.Stopped = append(result.Stopped, System)
	}
	return result, nil
}

func (s *Service) DaemonRunning(ctx context.Context) bool { return s.userspaceRunning(ctx) }

func (s *Service) StartDaemon(ctx context.Context) error {
	if s.userspaceRunning(ctx) {
		return nil
	}
	if err := s.h.FS.MkdirAll(s.dir(), 0o700); err != nil {
		return &DaemonError{Detail: "cannot create " + s.dir() + ": " + err.Error()}
	}
	pid, err := s.h.Run.Start(system.Cmd{Name: "tailscaled", Args: []string{
		"--tun=userspace-networking",
		"--state=" + s.stateFile(),
		"--socket=" + s.socket(),
	}}, s.logFile())
	if err != nil {
		return &DaemonError{Detail: "cannot start the userspace tailscaled: " + err.Error()}
	}
	if err := s.h.FS.WriteFile(s.pidFile(), []byte(strconv.Itoa(pid)), 0o600); err != nil {
		return &DaemonError{Detail: "cannot record the tailscaled pid: " + err.Error()}
	}
	for waited := time.Duration(0); waited < readyBudget; waited += readyInterval {
		if s.userspaceRunning(ctx) {
			return nil
		}
		if err := s.h.Clock.Sleep(ctx, readyInterval); err != nil {
			return err
		}
	}
	return &DaemonError{Detail: "the userspace tailscaled did not become ready (see " + s.logFile() + ")"}
}

func (s *Service) StopDaemon(context.Context) error {
	pid, ok := s.pid(s.pidFile())
	if !ok {
		return nil
	}
	if err := s.h.Run.Signal(pid, system.SigTerm); err != nil && s.h.Run.Alive(pid) {
		return &DaemonError{Detail: "cannot stop the userspace tailscaled: " + err.Error()}
	}
	_ = s.h.FS.Remove(s.pidFile())
	return nil
}

func (s *Service) pid(file string) (int, bool) {
	data, err := s.h.FS.ReadFile(file)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid, err == nil && pid > 0
}

func (s *Service) keepAwake() bool {
	pid, ok := s.pid(s.awakeFile())
	return ok && s.h.Run.Alive(pid)
}

func (s *Service) keepAwakeStart() bool {
	if s.keepAwake() {
		return true
	}
	if _, ok := s.h.Run.LookPath("caffeinate"); !ok {
		return false
	}
	pid, err := s.h.Run.Start(system.Cmd{Name: "caffeinate", Args: []string{"-i"}}, path.Join(s.dir(), "caffeinate.log"))
	if err != nil {
		return false
	}
	return s.h.FS.WriteFile(s.awakeFile(), []byte(strconv.Itoa(pid)), 0o600) == nil
}

func (s *Service) keepAwakeStop() {
	if pid, ok := s.pid(s.awakeFile()); ok {
		_ = s.h.Run.Signal(pid, system.SigTerm)
	}
	_ = s.h.FS.Remove(s.awakeFile())
}

var nonDefaultFlags = regexp.MustCompile(`requires mentioning all\s+non-default flags`)

func suggestedFlags(output string) []string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "tailscale up ") {
			if fields := strings.Fields(trimmed); len(fields) > 2 {
				return fields[2:]
			}
		}
	}
	return nil
}

func failureLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Error:"); ok {
			return strings.TrimSpace(rest)
		}
	}
	for _, line := range strings.Split(output, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return "no detail"
}

type flag struct {
	key    string
	tokens []string
}

func parseFlags(tokens []string) []flag {
	var flags []flag
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if !strings.HasPrefix(t, "--") {
			continue
		}
		if eq := strings.Index(t, "="); eq > 0 {
			flags = append(flags, flag{key: t[:eq], tokens: []string{t}})
			continue
		}
		if i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "--") {
			flags = append(flags, flag{key: t, tokens: []string{t, tokens[i+1]}})
			i++
			continue
		}
		flags = append(flags, flag{key: t, tokens: []string{t}})
	}
	return flags
}

func mergeFlags(desiredArgs, suggested []string) []string {
	desired := parseFlags(desiredArgs[1:])
	byKey := make(map[string]flag, len(desired))
	for _, f := range desired {
		byKey[f.key] = f
	}
	named := map[string]bool{}
	merged := []string{"up"}
	for _, f := range parseFlags(suggested) {
		named[f.key] = true
		if d, ok := byKey[f.key]; ok {
			merged = append(merged, d.tokens...)
			continue
		}
		merged = append(merged, f.tokens...)
	}
	for _, f := range desired {
		if !named[f.key] {
			merged = append(merged, f.tokens...)
		}
	}
	return merged
}
