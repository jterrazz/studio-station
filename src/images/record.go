package images

import (
	"encoding/json"
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
	// value: fetched from the registry that releases the image, not built
	// here.
	Fetched bool `json:"fetched,omitempty"`
}

func (r *Record) UnmarshalJSON(data []byte) error {
	// value: alias of v2's `published` key of an image record, read as
	// fetched; the next write says fetched.
	type plain Record
	var v struct {
		plain
		Published bool `json:"published"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*r = Record(v.plain)
	r.Fetched = r.Fetched || v.Published
	return nil
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

type UnknownError struct{ Image string }

func (e *UnknownError) Error() string { return "no image \"" + e.Image + "\"" }
