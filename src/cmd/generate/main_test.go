package main

import (
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
