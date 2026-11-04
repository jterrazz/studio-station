package jsonfile_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jterrazz/studio-station/src/core/adapter/jsonfile"
	"github.com/jterrazz/studio-station/src/core/contract"
	"github.com/jterrazz/studio-station/src/core/contract/contracttest"
	"github.com/jterrazz/studio-station/src/core/store"
	"github.com/jterrazz/studio-station/src/platform/local"
)

func disk() jsonfile.Files { return local.New(nil, nil, nil, nil).FS }

func TestPreferences(t *testing.T) {
	contracttest.Preferences(t, func(t *testing.T, groups []string) contract.Preferences {
		return jsonfile.NewPreferences(disk(), t.TempDir(), groups)
	})
}

func TestPreferencesFileReadsAsTodaysSettings(t *testing.T) {
	dir := t.TempDir()
	p := jsonfile.NewPreferences(disk(), dir, []string{"here", "hosts", "vm_user"})
	if err := p.Write("vm_user", "tester"); err != nil {
		t.Fatal(err)
	}
	if err := p.Write("here", "macbook"); err != nil {
		t.Fatal(err)
	}
	if err := p.Write("hosts", map[string]struct{}{"macbook": {}}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
	want := "{\n  \"here\": \"macbook\",\n  \"hosts\": {\n    \"macbook\": {}\n  },\n  \"vm_user\": \"tester\"\n}\n"
	if string(got) != want {
		t.Errorf("settings.json =\n%s\nwant\n%s", got, want)
	}
}

func TestPreferencesMigration(t *testing.T) {
	t.Run("a retired key is dropped, every known group kept", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"here":"x","tailnet":{"mode":"auto"}}`), 0o600)
		if err := jsonfile.NewPreferences(disk(), dir, []string{"here"}).Migrate(context.Background()); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(filepath.Join(dir, "settings.json")); string(got) != "{\n  \"here\": \"x\"\n}\n" {
			t.Errorf("settings.json = %s", got)
		}
	})
	t.Run("a file of a newer station is refused, never rewritten", func(t *testing.T) {
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"version":2,"here":"x"}`), 0o600)
		var newer *store.NewerError
		if err := jsonfile.NewPreferences(disk(), dir, []string{"here"}).Migrate(context.Background()); !errors.As(err, &newer) {
			t.Fatalf("Migrate = %v, want a newer version refused", err)
		}
	})
}
