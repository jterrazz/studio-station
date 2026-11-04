package contract

import (
	"context"
	"encoding/json"

	"github.com/jterrazz/studio-station/src/core/call"
	"github.com/jterrazz/studio-station/src/core/machine"
)

type Hello struct {
	Version string `json:"version"`
	// value: the names of the calls this binary serves.
	Calls []string `json:"calls"`
}

type EndpointDialer interface {
	// invariant: dials a Mac, never a VM: a VM's calls ride its Mac's endpoint.
	Dial(ctx context.Context, mac string) (Endpoint, error)
}

type Endpoint interface {
	Hello(ctx context.Context) (Hello, error)
	Invoke(ctx context.Context, target machine.Ref, name string, in json.RawMessage, p call.Progress) (json.RawMessage, error)
	Watch(ctx context.Context, target machine.Ref, name string, in json.RawMessage, p call.Progress) (<-chan call.Update[json.RawMessage], error)
	Close() error
}
