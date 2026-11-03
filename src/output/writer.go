package output

import (
	"errors"
	"io"
	"sort"

	"github.com/jterrazz/studio-station/src/fail"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/render/tty"
)

type Writer struct {
	stdout  Stream
	stderr  Stream
	env     map[string]string
	environ []string
	waiting func()
}

func NewWriter(stdout, stderr Stream, env map[string]string) *Writer {
	environ := make([]string, 0, len(env))
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}
	sort.Strings(environ)
	return &Writer{stdout: stdout, stderr: stderr, env: env, environ: environ}
}

func (w *Writer) Raw(p []byte) error {
	w.settle()
	_, err := w.stdout.W.Write(p)
	return err
}

func (w *Writer) CanLive() bool { return Detect(w.stdout, w.env, false).Mode == TTY }

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
	w.settle()
	c := Detect(w.stderr, w.env, false)
	block := tty.ErrorBlock(page.Error{Message: e.Message, Hint: e.Hint}, c.Mode == TTY, c.Dark)
	_, _ = io.WriteString(w.colored(w.stderr, c), block)
}

func (w *Writer) Note(text string) {
	w.settle()
	_, _ = io.WriteString(w.stderr.W, text+"\n")
}

func (w *Writer) Menu(render func(io.Writer)) {
	w.settle()
	render(w.stderr.W)
}

func (w *Writer) Wait(label string) func() {
	c := Detect(w.stderr, w.env, false)
	if c.Mode != TTY {
		return func() {}
	}
	w.settle()
	stop := tty.Wait(w.colored(w.stderr, c), label, c.Dark)
	w.waiting = stop
	return func() {
		stop()
		w.waiting = nil
	}
}

func (w *Writer) settle() {
	// invariant: a spinner is erased before anything else is written, so a
	// slow read never leaves its frame glued to the page that follows it.
	if w.waiting != nil {
		w.waiting()
		w.waiting = nil
	}
}

func (w *Writer) colored(s Stream, c Context) io.Writer {
	if c.Mode != TTY {
		return s.W
	}
	return tty.Colored(s.W, w.environ)
}
