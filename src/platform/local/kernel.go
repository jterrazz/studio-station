package local

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/jterrazz/studio-station/src/platform/darwin"
)

type Kernel interface {
	Counters() (darwin.Counters, error)
	Processes() (darwin.Procs, error)
	Process(pid int) (darwin.Procs, error)
	Files(pid int) ([]string, error)
}

var errNoKernel = errors.New("no kernel tables here")

func NewKernel(vars map[string]string) Kernel {
	if dir := vars["STATION_KERNEL_STUB"]; filepath.IsAbs(dir) {
		return &stubKernel{dir: dir}
	}
	return darwin.Kernel{}
}

// value: the test seam STATION_KERNEL_STUB names: recorded kernel tables,
// counters.json read at every call, and procs.json a list of process tables
// the readings walk through, the last one repeating; a directory that does
// not exist is a machine whose kernel answers nothing.
type stubKernel struct {
	dir  string
	mu   sync.Mutex
	next int
}

func (s *stubKernel) Counters() (darwin.Counters, error) {
	var c darwin.Counters
	err := s.read("counters.json", &c)
	return c, err
}

func (s *stubKernel) Processes() (darwin.Procs, error) {
	var readings []darwin.Procs
	if err := s.read("procs.json", &readings); err != nil || len(readings) == 0 {
		return darwin.Procs{}, errNoKernel
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	at := min(s.next, len(readings)-1)
	s.next++
	return readings[at], nil
}

func (s *stubKernel) Process(pid int) (darwin.Procs, error) {
	all, err := s.Processes()
	if err != nil {
		return darwin.Procs{}, err
	}
	for _, p := range all.List {
		if p.PID == pid {
			return darwin.Procs{At: all.At, List: []darwin.Proc{p}}, nil
		}
	}
	return darwin.Procs{}, errNoKernel
}

func (s *stubKernel) Files(int) ([]string, error) { return nil, errNoKernel }

func (s *stubKernel) read(name string, into any) error {
	data, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return errNoKernel
	}
	return json.Unmarshal(data, into)
}
