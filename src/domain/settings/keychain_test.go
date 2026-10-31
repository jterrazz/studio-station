package settings_test

import (
	"context"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
	"github.com/jterrazz/studio-station/src/system/systemtest"
)

func TestKeychainWriteQuotesTheValueOnStdinNeverInArgv(t *testing.T) {
	run := systemtest.NewRunner(t)
	run.On("security", "-i").Reply(system.Output{})
	k := settings.NewKeychain(run).On(catalog.Host)

	if err := k.Write(context.Background(), "vm-join-secret", `a "b" \c d`); err != nil {
		t.Fatal(err)
	}
	want := `add-generic-password -U -s station -a "vm-join-secret" -w "a \"b\" \\c d"` + "\n"
	if got := run.Calls[0].Stdin; got != want {
		t.Fatalf("stdin\n%s\nwant\n%s", got, want)
	}
}

func TestKeychainWriteRefusesANewline(t *testing.T) {
	k := settings.NewKeychain(systemtest.NewRunner(t)).On(catalog.Host)
	if err := k.Write(context.Background(), "a", "line\nadd-generic-password -s x"); err == nil {
		t.Fatal("a value holding a newline reached security's stdin")
	}
}

func TestKeychainIsClosedOffAHost(t *testing.T) {
	k := settings.NewKeychain(systemtest.NewRunner(t)).On(catalog.Linux)
	if _, _, err := k.Read(context.Background(), "a"); err != settings.ErrNoKeychain {
		t.Fatalf("read on linux: %v", err)
	}
}
