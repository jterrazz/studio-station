package output

import (
	"errors"
	"io"
	"sort"
	"sync"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"

	"github.com/jterrazz/studio-station/src/fail"
	"github.com/jterrazz/studio-station/src/present"
	"github.com/jterrazz/studio-station/src/theme"
)

type Writer struct {
	stdout  Stream
	stderr  Stream
	env     map[string]string
	environ []string
}

func NewWriter(stdout, stderr Stream, env map[string]string) *Writer {
	environ := make([]string, 0, len(env))
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}
	sort.Strings(environ)
	return &Writer{stdout: stdout, stderr: stderr, env: env, environ: environ}
}

func (w *Writer) Answer(a present.Answer, wantJSON bool) error {
	c := Detect(w.stdout, w.env, wantJSON)
	return For(c, theme.New(c.Dark)).Render(w.colored(w.stdout, c), a)
}

func (w *Writer) Raw(p []byte) error {
	_, err := w.stdout.W.Write(p)
	return err
}

func (w *Writer) Failure(err error) {
	if err == nil {
		return
	}
	var e *fail.Error
	if !errors.As(err, &e) {
		e = &fail.Error{Message: err.Error(), Code: fail.CodeFailure}
	}
	if e.Message == "" {
		return
	}
	c := Detect(w.stderr, w.env, false)
	label, hint := "Error:", "Hint:"
	if c.Mode == TTY {
		t := theme.New(c.Dark)
		label, hint = t.Error.Render(label), t.Muted.Render(hint)
	}
	block := label + " " + e.Message + "\n"
	if e.Hint != "" {
		block += hint + " " + e.Hint + "\n"
	}
	_, _ = io.WriteString(w.colored(w.stderr, c), block)
}

func (w *Writer) Menu(render func(io.Writer)) {
	render(w.stderr.W)
}

func (w *Writer) Wait(label string) (stop func()) {
	c := Detect(w.stderr, w.env, false)
	if c.Mode != TTY {
		return func() {}
	}
	style := theme.New(c.Dark).Muted
	out := w.colored(w.stderr, c)
	var mu sync.Mutex
	var ticker *time.Ticker
	drawn, stopped := false, false
	done := make(chan struct{})
	frame := 0
	frames := []rune(spinner)
	draw := func() {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			return
		}
		drawn = true
		_, _ = io.WriteString(out, "\r"+style.Render(string(frames[frame%len(frames)])+" "+label)+ansi.EraseLineRight)
		frame++
	}
	timer := time.AfterFunc(300*time.Millisecond, func() {
		draw()
		mu.Lock()
		if stopped {
			mu.Unlock()
			return
		}
		ticker = time.NewTicker(100 * time.Millisecond)
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

const spinner = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"

func (w *Writer) colored(s Stream, c Context) io.Writer {
	if c.Mode != TTY {
		return s.W
	}
	return colorprofile.NewWriter(s.W, w.environ)
}
