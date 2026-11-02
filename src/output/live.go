package output

import (
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/jterrazz/studio-station/src/present"
	"github.com/jterrazz/studio-station/src/theme"
)

func (w *Writer) CanLive() bool { return Detect(w.stdout, w.env, false).Mode == TTY }

func (w *Writer) Live(answers <-chan present.Answer) error {
	// invariant: the live region is redrawn in place on every answer and erased
	// when the reading ends; the final document is then printed once, normally,
	// so the scrollback holds exactly what a non-live read prints.
	c := Detect(w.stdout, w.env, false)
	t := theme.New(c.Dark)
	out := w.colored(w.stdout, c)
	var (
		last  present.Answer
		got   bool
		drawn int
	)
	erase := func() string {
		if drawn == 0 {
			return ""
		}
		return "\r" + ansi.CursorUp(drawn) + ansi.EraseScreenBelow
	}
	for a := range answers {
		last, got = a, true
		lines := liveLines(TTYString(a.Doc, c.Width, t), a.Doc, c.Width, w.stdout.Height, t)
		frame := erase() + ansi.HideCursor + strings.Join(lines, "\n") + "\n" + ansi.ShowCursor
		if _, err := io.WriteString(out, frame); err != nil {
			return err
		}
		drawn = len(lines)
	}
	if e := erase(); e != "" {
		if _, err := io.WriteString(out, e); err != nil {
			return err
		}
	}
	if !got {
		return nil
	}
	return For(c, t).Render(out, last)
}

func liveLines(doc string, d present.Document, width, height int, t theme.Theme) []string {
	lines := strings.Split(strings.TrimRight(doc, "\n"), "\n")
	for i, l := range lines {
		lines[i] = ansi.Truncate(l, max(width-1, 1), "")
	}
	if height <= 1 || len(lines) < height {
		return lines
	}
	room := max(height-2, 1)
	more := "… " + strconv.Itoa(len(lines)-room) + " more lines"
	if n := pendingRows(d.Blocks); n > 0 {
		more = "… " + strconv.Itoa(n) + " rows pending"
	}
	return append(lines[:room], margin+t.Muted.Render(more))
}

func pendingRows(blocks []present.Block) int {
	n := 0
	for _, b := range blocks {
		switch b := b.(type) {
		case present.Section:
			n += pendingRows(b.Blocks)
		case present.Table:
			for _, r := range b.Rows {
				if r.Mark == present.MarkPending {
					n++
				}
			}
		}
	}
	return n
}
