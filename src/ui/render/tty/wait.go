package tty

import (
	"io"
	"sync"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/atom"
	"github.com/jterrazz/studio-station/src/ui/token"
)

func ErrorBlock(e page.Error, colored, dark bool) string {
	label, hint := "Error:", "Hint:"
	if colored {
		st := token.New(dark)
		label, hint = st.Error.Render(label), st.Muted.Render(hint)
	}
	block := label + " " + e.Message + "\n"
	if e.Hint != "" {
		block += hint + " " + e.Hint + "\n"
	}
	return block
}

func Wait(out io.Writer, label string, dark bool) (stop func()) {
	style := token.New(dark).Muted
	var mu sync.Mutex
	var ticker *time.Ticker
	drawn, stopped := false, false
	done := make(chan struct{})
	frame := 0
	frames := []rune(token.Spinner)
	draw := func() {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			return
		}
		drawn = true
		_, _ = io.WriteString(out, "\r"+style.Render(string(frames[frame%len(frames)])+atom.Spaces(1)+label)+ansi.EraseLineRight)
		frame++
	}
	timer := time.AfterFunc(token.WaitDelay, func() {
		draw()
		mu.Lock()
		if stopped {
			mu.Unlock()
			return
		}
		ticker = time.NewTicker(token.SpinnerFrame)
		mu.Unlock()
		go func() {
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					draw()
				}
			}
		}()
	})
	return func() {
		timer.Stop()
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			return
		}
		stopped = true
		close(done)
		if ticker != nil {
			ticker.Stop()
		}
		if drawn {
			_, _ = io.WriteString(out, "\r"+ansi.EraseEntireLine)
		}
	}
}
