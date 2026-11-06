package wire

import (
	"context"
	"testing"
)

func TestQuittingTheTUIStopsTheReadsItStartedAndWaitsForThem(t *testing.T) {
	var r reads
	ctx, done, ok := r.begin(context.Background())
	if !ok {
		t.Fatal("a read the screen starts is refused before the TUI quits")
	}
	ended := make(chan struct{})
	go func() {
		r.end()
		close(ended)
	}()
	<-ctx.Done()
	select {
	case <-ended:
		t.Fatal("the TUI's exit did not wait for the read it stopped")
	default:
	}
	done()
	<-ended
	if _, _, ok := r.begin(context.Background()); ok {
		t.Error("a read starts after the TUI quit")
	}
}
