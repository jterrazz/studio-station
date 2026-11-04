package fake

import (
	"testing"

	"github.com/jterrazz/studio-station/src/ports"
	"github.com/jterrazz/studio-station/src/ports/contract"
	"github.com/jterrazz/studio-station/src/ports/contract/contracttest"
)

func TestTheFakeTailnetKeepsTheContract(t *testing.T) {
	contracttest.Tailnet(t, func(*testing.T) contract.Tailnet {
		host := "jterrazz-macbook-pro.tail1234.ts.net"
		return NewTailnet(ports.Self{
			Node:    ports.Node{Name: "jterrazz-macbook-pro", DNSName: host, IP: "100.98.133.14", Online: true},
			Sharing: ports.SharingState{Backend: ports.Running, On: true, Host: host},
		})
	})
}

func TestTheFakeListenersKeepTheContract(t *testing.T) {
	contracttest.Listeners(t, func(*testing.T) contract.Listeners { return NewListeners(contracttest.Scene()...) })
}

func TestTheFakeReservationStoreKeepsTheContract(t *testing.T) {
	contracttest.ReservationStore(t, func(*testing.T) contract.ReservationStore { return NewReservations() })
}

func TestTheFakeDeclarationStoreKeepsTheContract(t *testing.T) {
	contracttest.DeclarationStore(t, func(*testing.T) contract.DeclarationStore { return NewDeclarations() })
}
