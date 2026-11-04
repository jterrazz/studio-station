package fake

import (
	"io/fs"
	"testing"

	"github.com/jterrazz/studio-station/src/machines/contract"
	"github.com/jterrazz/studio-station/src/machines/contract/contracttest"
)

func TestTheFakeGuestKeepsTheContract(t *testing.T) {
	contracttest.Guest(t, func(*testing.T) contracttest.GuestSubject {
		guests := NewGuests()
		vm := guests.Add("dev")
		vm.Serve(contracttest.GuestPort, contracttest.Greeting)
		return contracttest.GuestSubject{
			Dial: func(t *testing.T) contract.Guest {
				g, err := guests.Dial(t.Context(), contract.GuestTarget{VM: "dev"})
				if err != nil {
					t.Fatal(err)
				}
				return g
			},
			Program:  func(line string, a contracttest.Answer) { vm.Program(line, Answer(a)) },
			Received: vm.Received,
			Read: func(t *testing.T, path string) ([]byte, fs.FileMode) {
				f, ok := vm.File(path)
				if !ok {
					t.Fatalf("no file %s in the guest", path)
				}
				return f.Data, f.Mode
			},
			Path: "/tmp/station",
		}
	})
}
