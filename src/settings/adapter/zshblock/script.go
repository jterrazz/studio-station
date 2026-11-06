package zshblock

import (
	"context"
	"io/fs"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings"
)

const (
	// value: how long a new shell waits for the Claude token; past it, the shell starts without.
	TokenBudget = 2 * time.Second
)

type BlockOptions struct {
	Here     catalog.MachineKind
	Services string
	// value: the scripts of the shell functions settings applied here; one
	// not written is skipped.
	Functions []string
}

// value: the script station's ~/.zshrc block evaluates: the dotfiles'
// loader, the tools' activation, completion, the Claude token, the
// services' variables and the settings' shell functions.
type Block struct {
	files    Files
	embedded fs.FS
	source   contract.SecretSource
	o        BlockOptions
}

func NewBlock(files Files, embedded fs.FS, source contract.SecretSource, o BlockOptions) *Block {
	return &Block{files: files, embedded: embedded, source: source, o: o}
}

func (s *Block) Zsh(ctx context.Context, completion string) (string, error) {
	loader, err := fs.ReadFile(s.embedded, catalog.ZshLoader)
	if err != nil {
		return "", err
	}
	// invariant: an interactive shell that ran no compinit (a fresh Mac, no oh-my-zsh)
	// gets it from the block, so the completion is there; a script's shell runs none,
	// and the completion is guarded by `$+functions[compdef]`.
	script := string(loader) + "\n" + catalog.ToolActivation + "\n" +
		"if (( ! $+functions[compdef] )) && [[ -o interactive ]]; then\n  autoload -Uz compinit && compinit -i\nfi\n" +
		"if (( $+functions[compdef] )); then\n" + completion + "\nfi\n"
	if line, ok := s.token(ctx); ok {
		script += "\n" + line + "\n"
	}
	return script + s.services() + s.functions(), nil
}

// invariant: the functions of an applied setting are a file read, like the
// profiles' exports: a shell start never waits on anything.
func (s *Block) functions() string {
	var out strings.Builder
	for _, f := range s.o.Functions {
		if data, err := s.files.ReadFile(f); err == nil && len(data) > 0 {
			out.WriteString("\n" + string(data))
		}
	}
	return out.String()
}

// invariant: in a VM the profiles' exports the guest wrote are a file read, never
// a subprocess: a shell start never waits on the host.
func (s *Block) services() string {
	if (s.o.Here != catalog.Linux && s.o.Here != catalog.MacOS) || s.o.Services == "" {
		return ""
	}
	data, err := s.files.ReadFile(s.o.Services)
	if err != nil || len(data) == 0 {
		return ""
	}
	return "\n" + string(data)
}

// hazard: the file backend is forced because a VM has no keychain, and any failure
// (no studio, a timeout, an empty store) answers nothing so a new shell never breaks.
func (s *Block) token(ctx context.Context) (string, bool) {
	if s.o.Here != catalog.Linux && s.o.Here != catalog.MacOS {
		return "", false
	}
	exported, err := s.source.Export(ctx, []string{catalog.ClaudeTokenName})
	if err != nil {
		return "", false
	}
	return settings.ClaudeTokenLine(exported)
}
