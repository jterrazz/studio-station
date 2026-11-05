package zshblock_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings/adapter/zshblock"
	"github.com/jterrazz/studio-station/src/settings/contract/contracttest"
)

type memory struct{ files map[string][]byte }

func (m *memory) ReadFile(p string) ([]byte, error) {
	if b, ok := m.files[p]; ok {
		return b, nil
	}
	return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrNotExist}
}

func (m *memory) WriteFile(p string, b []byte, _ fs.FileMode) error {
	m.files[p] = append([]byte(nil), b...)
	return nil
}

func (*memory) MkdirAll(string, fs.FileMode) error { return nil }

func (m *memory) Remove(p string) error {
	if _, ok := m.files[p]; !ok {
		return &fs.PathError{Op: "remove", Path: p, Err: fs.ErrNotExist}
	}
	delete(m.files, p)
	return nil
}

func TestApplier(t *testing.T) {
	contracttest.Applier(t, func(*testing.T) contracttest.Opened {
		files := &memory{files: map[string][]byte{}}
		ours := fstest.MapFS{"zsh/greet.zsh": {Data: []byte("greet() { echo hi; }\n")}}
		item := catalog.ConfigItem{Name: "greetings", Effect: catalog.ZshFunctions{From: "zsh/greet.zsh", Functions: []string{"greet"}}}
		script := zshblock.Script("/home/u/.config/station/zsh", item.Name)
		edit := func(*testing.T) { files.files[script] = []byte("greet() { echo mine; }\n") }
		return contracttest.Opened{Applier: zshblock.New(files, zshblock.Options{Dir: "/home/u/.config/station/zsh", Ours: ours}), Item: item, Edit: edit}
	})
}
