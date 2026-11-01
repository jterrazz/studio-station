package host

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/jterrazz/studio-station/src/system"
)

type disk struct{}

func (disk) Open(name string) (fs.File, error)            { return os.Open(name) }
func (disk) ReadFile(path string) ([]byte, error)         { return os.ReadFile(path) }
func (disk) Stat(path string) (fs.FileInfo, error)        { return os.Stat(path) }
func (disk) ReadDir(path string) ([]fs.DirEntry, error)   { return os.ReadDir(path) }
func (disk) Glob(pattern string) ([]string, error)        { return filepath.Glob(pattern) }
func (disk) Remove(path string) error                     { return os.Remove(path) }
func (disk) RemoveAll(path string) error                  { return os.RemoveAll(path) }
func (disk) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }
func (disk) Chmod(path string, perm fs.FileMode) error    { return os.Chmod(path, perm) }
func (disk) Chown(path string, uid, gid int) error        { return os.Chown(path, uid, gid) }

func (disk) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}

func (disk) WalkDir(root string, fn fs.WalkDirFunc) error { return filepath.WalkDir(root, fn) }

func (disk) DirSize(path string) int64 {
	var total int64
	_ = filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func (disk) Rename(from, to string) error {
	err := os.Rename(from, to)
	if errors.Is(err, syscall.EXDEV) {
		return errors.Join(system.ErrCrossDevice, err)
	}
	return err
}
