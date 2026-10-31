package shell

import (
	"encoding/json"
	"strings"
)

const (
	blockStart = "# >>> station >>>"
	blockEnd   = "# <<< station <<<"
	tokenName  = "operator:claude:oauth-token"
)

// invariant: ~/.local/bin is on PATH before the eval runs, and the `command -v`
// guard keeps a deleted binary from breaking every new shell.
const block = blockStart + "\n" +
	`export PATH="$HOME/.local/bin:$PATH"` + "\n" +
	`command -v station >/dev/null 2>&1 && eval "$(station shell zsh)"` + "\n" +
	blockEnd + "\n"

func HasBlock(zshrc string) bool {
	return strings.Count(zshrc, block) == 1
}

func UpsertBlock(zshrc string) (updated string, changed bool) {
	lines := strings.Split(zshrc, "\n")
	kept := withoutLegacy(lines)
	if HasBlock(zshrc) && len(kept) == len(lines) && strings.Count(zshrc, blockStart) == 1 {
		return zshrc, false
	}
	var out []string
	inBlock := false
	for _, line := range kept {
		switch {
		case strings.TrimSpace(line) == blockStart:
			inBlock = true
		case strings.TrimSpace(line) == blockEnd:
			inBlock = false
		case !inBlock:
			out = append(out, line)
		}
	}
	text := strings.TrimRight(strings.Join(out, "\n"), "\n")
	if text != "" {
		text += "\n\n"
	}
	updated = text + block
	return updated, updated != zshrc
}

func ClaudeTokenLine(exportJSON []byte) (string, bool) {
	var values map[string]string
	if json.Unmarshal(exportJSON, &values) != nil {
		return "", false
	}
	token := values[tokenName]
	if token == "" {
		return "", false
	}
	return "export CLAUDE_CODE_OAUTH_TOKEN='" + strings.ReplaceAll(token, "'", `'\''`) + "'", true
}
