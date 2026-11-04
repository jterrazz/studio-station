//go:build !world

package wire

import (
	"context"

	"github.com/spf13/cobra"

	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/engine"
	"github.com/jterrazz/studio-station/src/machines/contract"
	remotecontract "github.com/jterrazz/studio-station/src/remote/contract"
	"github.com/jterrazz/studio-station/src/system"
)

// invariant: the release build knows no world: openWorld answers none, so
// every seam below stays the real adapter's and no world command exists.
type (
	worldRun struct{}
	worldMac struct{}
)

func openWorld(map[string]string) (*worldRun, error) { return nil, nil }

func (*worldRun) here() *worldMac                { return nil }
func (*worldRun) farFrom(func(*worldMac) farMac) {}
func (*worldRun) command() *cobra.Command        { return nil }
func (*worldRun) close(context.Context) error    { return nil }

func (*worldMac) host(h system.Host) system.Host        { return h }
func (*worldMac) hypervisor(string) contract.Hypervisor { return nil }
func (*worldMac) preferences() preferences              { return nil }
func (*worldMac) machineStore() machineStore            { return nil }
func (*worldMac) vault() accountscontract.Vault         { return nil }
func (*worldMac) notifier() engine.Notifier             { return nil }
func (*worldMac) guests() engine.Guests                 { return nil }
func (*worldMac) engineFrom(func() *engine.Daemon)      {}
func (*worldMac) macs() remotecontract.EndpointDialer   { return nil }
