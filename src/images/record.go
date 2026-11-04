package images

import (
	"strconv"
	"strings"
	"time"
)

type Ref struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (r Ref) String() string { return r.Name + "@" + r.Version }

type Record struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Base      string    `json:"base"`
	Station   string    `json:"station"`
	RecipeSHA string    `json:"recipe_sha"`
	BuiltAt   time.Time `json:"built_at"`
	SizeGB    int       `json:"size_gb"`
	// value: fetched from where the image is published, not built here.
	Published bool `json:"published,omitempty"`
}

func (r Record) Ref() Ref { return Ref{Name: r.Name, Version: r.Version} }

type Report struct {
	Record Record
	Newest bool
	VMs    []string
	// value: the Mac that holds this version.
	Mac string
}

type Update struct {
	Ref Ref
	// value: the version was already the newest here: nothing was fetched.
	Newest bool
}

func CompareVersions(a, b string) int {
	// invariant: versions are dotted numbers (2026.11.01, 2026.11.01.2); a
	// missing part counts as 1, so a day's first build sorts before its second.
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(pa), len(pb)) {
		if c := part(pa, i) - part(pb, i); c != 0 {
			return c
		}
	}
	return 0
}

func part(parts []string, i int) int {
	if i >= len(parts) {
		return 1
	}
	n, err := strconv.Atoi(parts[i])
	if err != nil {
		return 0
	}
	return n
}
