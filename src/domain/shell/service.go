package shell

import (
	"context"
	"io/fs"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	loaderPath  = "dotfiles/zsh/zshrc.sh"
	tokenBudget = 2 * time.Second
)

// invariant: mise is activated only when it is on PATH, and the activation comes after the
// loader's PATH lines, so a machine without mise gets a quiet shell.
const miseBlock = `if command -v mise >/dev/null 2>&1; then
  eval "$(mise activate zsh)"
fi
`

type Options struct {
	Here     catalog.MachineKind
	Services string
}

type Service struct {
	h     system.Host
	files fs.FS
	o     Options
}

func New(h system.Host, files fs.FS, o Options) *Service {
	return &Service{h: h, files: files, o: o}
}

func (s *Service) Zsh(ctx context.Context, completion string) (string, error) {
	loader, err := fs.ReadFile(s.files, loaderPath)
	if err != nil {
		return "", err
	}
	// invariant: the completion is guarded by `$+functions[compdef]`, so a shell without
	// compinit gets none and no `command not found: compdef`.
	script := string(loader) + "\n" + miseBlock + "\n" +
		"if (( $+functions[compdef] )); then\n" + completion + "\nfi\n"
	if line, ok := s.token(ctx); ok {
		script += "\n" + line + "\n"
	}
	return script + s.services(), nil
}

// invariant: in a VM the profiles' exports the guest wrote are a file read, never
// a subprocess: a shell start never waits on the host.
func (s *Service) services() string {
	if (s.o.Here != catalog.Linux && s.o.Here != catalog.MacOS) || s.o.Services == "" {
		return ""
	}
	data, err := s.h.FS.ReadFile(s.o.Services)
	if err != nil || len(data) == 0 {
		return ""
	}
	return "\n" + string(data)
}

// hazard: the file backend is forced because a VM has no keychain, and any failure
// (no studio, a timeout, an empty store) answers nothing so a new shell never breaks.
func (s *Service) token(ctx context.Context) (string, bool) {
	if s.o.Here != catalog.Linux && s.o.Here != catalog.MacOS {
		return "", false
	}
	out, err := s.h.Run.Output(ctx, system.Cmd{
		Name:    "studio",
		Args:    []string{"credentials", "export", tokenName},
		Env:     []string{"STUDIO_CREDENTIALS_BACKEND=file"},
		Timeout: tokenBudget,
	})
	if err != nil || out.ExitCode != 0 {
		return "", false
	}
	return ClaudeTokenLine([]byte(out.Stdout))
}
