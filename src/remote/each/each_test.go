package each_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/jterrazz/studio-station/src/remote/each"
)

func TestEveryMacAnswersAsItComesAndAFastOneIsNotKeptWaiting(t *testing.T) {
	answered := make(chan struct{})
	results := each.Do(t.Context(), []string{"mac-mini", "studio"}, func(_ context.Context, mac string) (string, error) {
		if mac == "mac-mini" {
			<-answered
		}
		return mac, nil
	})
	first := <-results
	close(answered)
	second := <-results
	if first.Value != "studio" || second.Value != "mac-mini" {
		t.Errorf("the answers came %q then %q, want the studio first while the mini still thinks", first.Value, second.Value)
	}
}

func TestAMacSlowerThanItsBudgetIsItsOwnFindingNotTheOthersDelay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		got := each.All(t.Context(), []string{"mac-mini", "studio"}, func(ctx context.Context, mac string) (string, error) {
			if mac == "mac-mini" {
				<-ctx.Done()
				return "", ctx.Err()
			}
			return mac, nil
		})
		if !errors.Is(got["mac-mini"].Err, context.DeadlineExceeded) || got["studio"].Value != "studio" {
			t.Errorf("a mini that never answers and a quick studio: %+v, want the mini out of its budget and the studio answered", got)
		}
	})
}

func TestAtMostFourMacsAreAskedAtOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var asking atomic.Int32
		release := make(chan struct{})
		done := make(chan struct{})
		go func() {
			each.All(t.Context(), []string{"a", "b", "c", "d", "e", "f", "g"}, func(_ context.Context, mac string) (string, error) {
				asking.Add(1)
				<-release
				asking.Add(-1)
				return mac, nil
			})
			close(done)
		}()
		synctest.Wait()
		if n := asking.Load(); n != each.Width {
			t.Errorf("%d Macs were asked at once, want %d", n, each.Width)
		}
		close(release)
		<-done
	})
}
