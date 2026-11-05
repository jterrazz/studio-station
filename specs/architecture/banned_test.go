package architecture_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

type banned struct {
	word string
	// value: the word as written counts only in that case (`Tasks` for the
	// strip, `NAT`), not as the plain English word it also is.
	exact bool
	// value: the phrases SPEC §2.2 keeps, taken out of a text before it is
	// read, and another program's own command line (`git config`, a fix the
	// owner types).
	allowed []string
}

// invariant: rule words, SPEC §2.2: the words v3 retired, each for the
// concept it used to name, with the phrases it keeps.
func bannedWords() []banned {
	return []banned{
		{word: "host", allowed: []string{"host:port"}},
		{word: "fleet"},
		{word: "daemon"},
		{word: "tool"},
		{word: "tools"},
		{word: "config", allowed: []string{".config/", "git config"}},
		{word: "storage"},
		{word: "service"},
		{word: "lease"},
		{word: "publish"},
		{word: "lend"},
		{word: "recall"},
		{word: "wish"},
		{word: "probe"},
		{word: "temporary"},
		{word: "lasting"},
		{word: "stray"},
		{word: "clean", allowed: []string{"dclean"}},
		{word: "reset", allowed: []string{"dock-reset", "dock reset", "dreset"}},
		{word: "power", allowed: []string{"power cut", "power-cut", "power source", "low power mode"}},
		{word: "place"},
		{word: "Resources", exact: true},
		{word: "Tasks", exact: true},
		{word: "space"},
		{word: "sandbox"},
		{word: "production"},
		{word: "OrbStack", exact: true},
		{word: "a check"},
		{word: "VM engine"},
		{word: "NAT", exact: true},
		{word: "tailnet"},
	}
}

// value: the fields of a page that name its machinery, never read: ids,
// kinds, the calls and the inputs an action fixes.
func unread() []string {
	return []string{"id", "key", "kind", "call", "page", "row", "schema", "mark", "reason", "fold", "role", "show", "state", "on", "at", "value"}
}

// value: the fields whose whole value is machinery: the rows an action
// acts on, a call's input, a program's arguments.
func unreadTrees() []string { return []string{"rows", "in", "argv"} }

func visible(v any, skip bool, out *[]string) {
	switch x := v.(type) {
	case string:
		if !skip {
			*out = append(*out, x)
		}
	case []any:
		for _, it := range x {
			visible(it, skip, out)
		}
	case map[string]any:
		for k, it := range x {
			_, text := it.(string)
			visible(it, skip || slices.Contains(unreadTrees(), k) || text && slices.Contains(unread(), k), out)
		}
	}
}

func bannedIn(texts []string) []string {
	var found []string
	for _, b := range bannedWords() {
		pattern := `\b` + regexp.QuoteMeta(b.word) + `\b`
		if !b.exact {
			pattern = `(?i)` + pattern
		}
		re := regexp.MustCompile(pattern)
		for _, t := range texts {
			for _, a := range b.allowed {
				t = regexp.MustCompile(`(?i)`+regexp.QuoteMeta(a)).ReplaceAllString(t, "")
			}
			if re.MatchString(t) {
				found = append(found, b.word)
				break
			}
		}
	}
	return found
}

// invariant: the allowlist of rule words: the page fixtures that still say
// a retired word, and the words. It only shrinks; a line no longer true
// fails the gate until it is removed.
func bannedTolerated() map[string][]string {
	return map[string][]string{
		"fleet":         {"fleet", "probe", "tailnet"},
		"home":          {"config", "fleet", "host", "place", "storage", "tailnet", "tools"},
		"home-all":      {"config", "fleet", "host", "place", "storage", "tailnet", "tools"},
		"kitchen-sink":  {"fleet", "host", "lend", "place", "power", "service", "storage", "stray", "tailnet", "tool"},
		"menubar":       {"fleet", "host", "power", "reset"},
		"outcome-power": {"power"},
		"power":         {"power", "reset"},
		"run":           {"daemon", "reset"},
	}
}

func bannedFindings(root string) (map[string][]string, error) {
	dir := filepath.Join(root, "src", "ui", "testdata", "pages")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, e := range entries {
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var doc any
		if err := json.Unmarshal(body, &doc); err != nil {
			return nil, err
		}
		var texts []string
		visible(doc, false, &texts)
		family := strings.SplitN(e.Name(), ".", 2)[0]
		for _, w := range bannedIn(texts) {
			if !slices.Contains(out[family], w) {
				out[family] = append(out[family], w)
			}
		}
	}
	for _, words := range out {
		sort.Strings(words)
	}
	return out, nil
}

func TestBannedWords(t *testing.T) {
	found, err := bannedFindings(repoRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	tolerated := bannedTolerated()
	for family, words := range found {
		for _, w := range words {
			if !slices.Contains(tolerated[family], w) {
				t.Errorf("src/ui/testdata/pages/%s.*.json says %q, a word SPEC §2.2 retired", family, w)
			}
		}
	}
	for family, words := range tolerated {
		for _, w := range words {
			if !slices.Contains(found[family], w) {
				t.Errorf("the allowlist tolerates %q in %s, which no longer says it: remove the line", w, family)
			}
		}
	}
}

func TestBannedWordsBite(t *testing.T) {
	page := map[string]any{
		"id":       "fleet",
		"context":  map[string]any{"kind": "host"},
		"sections": []any{map[string]any{"title": "The fleet", "items": []any{map[string]any{"row": map[string]any{"label": "Dock reset", "cells": []any{map[string]any{"text": "storage"}}}}}}},
	}
	var texts []string
	visible(page, false, &texts)
	if got := bannedIn(texts); !slices.Equal(got, []string{"fleet", "storage"}) {
		t.Errorf("a planted page says %v, want [fleet storage]: the id, the kind and Dock reset are not read", got)
	}
}
