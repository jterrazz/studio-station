package catalog

import "embed"

// invariant: `all:` keeps the dotted files (zed/.oxfmtrc.json) the plain pattern drops.

//go:embed all:dotfiles
var DotfilesFS embed.FS

const ZshLoader = "dotfiles/zsh/zshrc.sh" // value: the dotfiles' zsh loader, in DotfilesFS: the first lines of station's block.

// invariant: the version manager station installs tools with is activated
// only when it is on PATH, after the loader's PATH lines, so a machine
// without it gets a quiet shell.
const (
	ToolActivation = `if command -v mise >/dev/null 2>&1; then
  eval "$(mise activate zsh)"
fi
`
)
