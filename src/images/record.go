package images

import (
	"strconv"
	"strings"
	"time"
)

type Record struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Base      string    `json:"base"`
	Station   string    `json:"station"`
	RecipeSHA string    `json:"recipe_sha"`
	BuiltAt   time.Time `json:"built_at"`
	SizeGB    int       `json:"size_gb"`
}

type Report struct {
	Record Record
	Newest bool
	VMs    []string
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
