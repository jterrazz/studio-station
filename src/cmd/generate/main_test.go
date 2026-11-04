package main

import (
	"bytes"
	"github.com/jterrazz/studio-station/src/cmd/generate/pages"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func root(t *testing.T) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(here), "..", "..", "..")
}

func TestTheToolbeltProjectionHasNotDriftedFromTheCatalogue(t *testing.T) {
	current, err := os.ReadFile(filepath.Join(root(t), defaultPath))
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := render(string(current))
	if err != nil {
		t.Fatal(err)
	}
	if rendered != string(current) {
		t.Fatalf("%s is out of sync with src/catalog: run make generate and commit the result", defaultPath)
	}
}

func TestTheFootprintPageHasNotDriftedFromTheCatalogue(t *testing.T) {
	current, err := os.ReadFile(filepath.Join(root(t), footprintPath))
	if err != nil {
		t.Fatal(err)
	}
	if footprint() != string(current) {
		t.Fatalf("%s is out of sync with src/catalog: run make generate and commit the result", footprintPath)
	}
}

func TestThePageFixturesHaveNotDriftedFromThePageFunctions(t *testing.T) {
	want := pageFixtures()
	dir := filepath.Join(root(t), pagesDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(want) {
		t.Errorf("%s holds %d fixtures, the page functions build %d: run make generate", pagesDir, len(entries), len(want))
	}
	for name, body := range want {
		current, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || !bytes.Equal(current, body) {
			t.Errorf("%s/%s is out of sync with its page function: run make generate and commit the result", pagesDir, name)
		}
	}
}

func TestTheRecordedStreamHasNotDriftedFromTheDaemon(t *testing.T) {
	current, err := os.ReadFile(filepath.Join(root(t), streamPath))
	if err != nil || !bytes.Equal(current, pages.MenubarStream()) {
		t.Errorf("%s is out of sync with the daemon's encoder: run make generate and commit the result", streamPath)
	}
}

func TestTheRecordedEventsHaveNotDriftedFromTheEngine(t *testing.T) {
	current, err := os.ReadFile(filepath.Join(root(t), eventsPath))
	if err != nil || !bytes.Equal(current, pages.EventsStream()) {
		t.Errorf("%s is out of sync with the engine's events: run make generate and commit the result", eventsPath)
	}
}
