package catalog

import "embed"

// invariant: `all:` keeps the dotted files (zed/.oxfmtrc.json) the plain pattern drops.
//
//go:embed all:dotfiles
var DotfilesFS embed.FS
