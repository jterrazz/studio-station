package zshblock

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/settings"
	"github.com/jterrazz/studio-station/src/settings/contract"
)

const where = "zsh block"

type Files interface {
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, perm fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(path string) error
}

type Options struct {
	// value: the folder of the scripts station's zsh block evaluates, one
	// per setting.
	Dir string
	// value: station's own copies of the scripts, the embedded dotfiles.
	Ours fs.FS
}

type Applier struct {
	files Files
	o     Options
}

var _ contract.Applier = (*Applier)(nil)

func New(files Files, o Options) *Applier { return &Applier{files: files, o: o} }

func (*Applier) Kind() catalog.EffectKind { return catalog.EffectZshFunctions }

func Script(dir, setting string) string {
	// value: the script a setting's functions live in, which `station shell
	// zsh` appends to the block it prints.
	return path.Join(dir, setting+".zsh")
}

func (a *Applier) of(it catalog.Setting) (want []byte, file string, err error) {
	z, ok := it.Effect.(catalog.ZshFunctions)
	if !ok {
		return nil, "", errors.New(it.Name + " is not shell functions station defines")
	}
	want, err = fs.ReadFile(a.o.Ours, z.From)
	return want, Script(a.o.Dir, it.Name), err
}

func (a *Applier) Observe(_ context.Context, it catalog.Setting) (settings.Observation, error) {
	want, file, err := a.of(it)
	if err != nil {
		return settings.Observation{}, err
	}
	have, err := a.files.ReadFile(file)
	o := settings.Observation{Where: where}
	switch {
	case errors.Is(err, fs.ErrNotExist):
		o.State, o.Diff = catalog.StateAbsent, settings.UnifiedDiff(string(want), "", "station's "+where, where)
		return o, nil
	case err != nil:
		return settings.Observation{}, err
	}
	sum := sha256.Sum256(have)
	o.Digest = "sha256:" + hex.EncodeToString(sum[:])
	if bytes.Equal(want, have) {
		o.State, o.Detail = catalog.StateOK, where+": "+strings.Join(functions(it), ", ")
		return o, nil
	}
	o.State, o.Detail = catalog.StateDegraded, "differs from station's: station settings apply "+it.Name
	o.Diff = settings.UnifiedDiff(string(want), string(have), "station's "+where, where)
	return o, nil
}

func functions(it catalog.Setting) []string {
	z, _ := it.Effect.(catalog.ZshFunctions)
	return z.Functions
}

func (a *Applier) Apply(_ context.Context, it catalog.Setting, _ map[string]string) (settings.Change, error) {
	want, file, err := a.of(it)
	if err != nil {
		return settings.Change{}, err
	}
	if err := a.files.MkdirAll(path.Dir(file), 0o750); err != nil {
		return settings.Change{}, err
	}
	if err := a.files.WriteFile(file, want, 0o600); err != nil {
		return settings.Change{}, err
	}
	return settings.Change{Steps: []settings.ChangeStep{{Label: where, State: catalog.StateOK, Detail: strings.Join(functions(it), ", ") + " added"}}}, nil
}

func (a *Applier) Undo(_ context.Context, it catalog.Setting) (settings.Change, error) {
	// invariant: the functions leave the block only while they are still
	// exactly station's: off never destroys a change of the owner's.
	want, file, err := a.of(it)
	if err != nil {
		return settings.Change{}, err
	}
	have, err := a.files.ReadFile(file)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return settings.Change{}, nil
	case err != nil:
		return settings.Change{}, err
	case !bytes.Equal(want, have):
		return settings.Change{Steps: []settings.ChangeStep{{Label: where, State: catalog.StateOK, Detail: "yours, kept"}}}, nil
	}
	if err := a.files.Remove(file); err != nil {
		return settings.Change{}, err
	}
	return settings.Change{Steps: []settings.ChangeStep{{Label: where, State: catalog.StateOK, Detail: strings.Join(functions(it), ", ") + " removed"}}}, nil
}
