package dotfile_test

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/platform/local"
	"github.com/jterrazz/studio-station/src/settings/adapter/dotfile"
	"github.com/jterrazz/studio-station/src/settings/contract/contracttest"
)

type memory struct{ files map[string][]byte }

func newMemory() *memory { return &memory{files: map[string][]byte{}} }

func (m *memory) ReadFile(p string) ([]byte, error) {
	if b, ok := m.files[p]; ok {
		return b, nil
	}
	return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
}

func (m *memory) Stat(p string) (fs.FileInfo, error) {
	if _, ok := m.files[p]; ok {
		return fstest.MapFS{"f": {}}.Stat("f")
	}
	return nil, &fs.PathError{Op: "stat", Path: p, Err: fs.ErrNotExist}
}

func (m *memory) WriteFile(p string, b []byte, _ fs.FileMode) error {
	m.files[p] = append([]byte(nil), b...)
	return nil
}

func (*memory) MkdirAll(string, fs.FileMode) error { return nil }
func (*memory) Chmod(string, fs.FileMode) error    { return nil }

func (m *memory) Remove(p string) error {
	if _, ok := m.files[p]; !ok {
		return &fs.PathError{Op: "remove", Path: p, Err: fs.ErrNotExist}
	}
	delete(m.files, p)
	return nil
}

type silent struct{}

func (silent) Output(context.Context, local.Cmd) (local.Output, error) {
	return local.Output{ExitCode: 1}, nil
}

func TestApplier(t *testing.T) {
	contracttest.Applier(t, func(*testing.T) contracttest.Opened {
		files := newMemory()
		ours := fstest.MapFS{"starship/starship.toml": {Data: []byte("format = \"$all\"\n")}}
		item := catalog.ConfigItem{Name: "starship", Effect: catalog.Dotfiles{Files: []catalog.Copy{{From: "starship/starship.toml", To: "~/.config/starship.toml"}}}}
		edit := func(*testing.T) { files.files["/home/u/.config/starship.toml"] = []byte("format = \"mine\"\n") }
		return contracttest.Opened{Applier: dotfile.New(files, silent{}, dotfile.Options{Home: "/home/u", Ours: ours}), Item: item, Edit: edit}
	})
}
