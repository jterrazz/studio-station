package menubar

import (
	"time"

	"github.com/jterrazz/studio-station/src/page"
	pageactivity "github.com/jterrazz/studio-station/src/pages/activity"
)

const ActivityID page.ID = "menubar.activity"

// value: SPEC §6.4's picker, by the word the call's window takes and the
// owner's word for it.
type window struct {
	Flag, Words string
	For         time.Duration
}

func windows() []window {
	return []window{{"6m", "6 min", 6 * time.Minute}, {"1h", "1 h", time.Hour}, {"24h", "24 h", 24 * time.Hour}}
}

func Activity(v pageactivity.View, here page.Context) page.Page {
	// invariant: SPEC §10.2 Activity: the machine's measures of §6.2 as graphs
	// over the window picked (1 h unless picked), then its top processes, in
	// place of the CLI's last values; the graphs' section says its window,
	// which the picker shows picked.
	if v.Graph == 0 {
		v.Graph = time.Hour
	}
	p := pageactivity.Page(v, here)
	p.ID = ActivityID
	kept := p.Sections[:0]
	for _, s := range p.Sections {
		switch s.ID {
		case pageactivity.Graphs:
			s.Title = "Over the last " + windowWords(v.Graph)
			s.Caption = []page.Value{page.Text(windowWords(v.Graph))}
			listed(&s)
		case pageactivity.Processes:
			listed(&s)
		case "values", "top":
			continue
		}
		kept = append(kept, s)
	}
	p.Sections = kept
	return page.Seal(p)
}

func windowWords(d time.Duration) string {
	for _, w := range windows() {
		if w.For == d {
			return w.Words
		}
	}
	return d.String()
}

// invariant: a measure and a process are read, never judged: their
// sections list.
func listed(s *page.Section) {
	s.Columns = append([]page.Column(nil), s.Columns...)
	s.Columns[0].Kind = page.KindListing
}
