package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTheToolbeltProjectionHasNotDriftedFromTheCatalogue(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(here), "..", "..", "..", defaultPath)
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := render(string(current))
	if err != nil {
		t.Fatal(err)
	}
	if rendered != string(current) {
		t.Fatalf("%s is out of sync with src/catalog: run make skills and commit the result", defaultPath)
	}
}
