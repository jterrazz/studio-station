package page

import (
	"errors"
	"strconv"
)

var strict bool

func Strict(on bool) { strict = on }

func Seal(p Page) Page {
	pending := false
	for i := range p.Sections {
		counts := map[Mark]int{}
		waits := false
		for _, it := range p.Sections[i].Items {
			waits = tally(it, counts) || waits
		}
		pending = pending || waits
		// value: an uncounted section keeps the counts it was given: none, or
		// those of the larger set it sums up (Home's Tools and Config).
		if p.Sections[i].Uncounted && !waits {
			continue
		}
		p.Sections[i].Counts = nil
		if p.Sections[i].Role == Summary {
			continue
		}
		for _, m := range Marks() {
			if counts[m] > 0 {
				p.Sections[i].Counts = append(p.Sections[i].Counts, Tally{Mark: m, N: counts[m]})
			}
		}
	}
	switch {
	case p.Error != nil:
		p.State = Failed
	case pending:
		p.State = Loading
	default:
		p.State = Ready
	}
	if err := Check(p); err != nil && strict {
		panic(err)
	}
	return p
}

func tally(it Item, counts map[Mark]int) (pending bool) {
	switch {
	case it.Row != nil:
		counts[it.Row.Mark]++
		return rowPending(*it.Row)
	case it.Group != nil:
		counts[it.Group.Head.Mark]++
		pending = rowPending(it.Group.Head)
		for _, child := range it.Group.Items {
			pending = tally(child, counts) || pending
		}
		return pending
	case it.Fact != nil:
		return it.Fact.Value.Kind == KindPending
	}
	return false
}

func rowPending(r Row) bool {
	if r.Mark == Awaiting || r.Detail.Kind == KindPending {
		return true
	}
	for _, c := range r.Cells {
		if c.Kind == KindPending {
			return true
		}
	}
	return false
}

func Check(p Page) error {
	ids := map[string]bool{}
	for _, s := range p.Sections {
		if s.ID == "" || ids[s.ID] {
			return errors.New("section " + strconv.Quote(s.ID) + ": an ID unique in the page is required")
		}
		ids[s.ID] = true
		if s.Title == "" {
			return errors.New("section " + s.ID + ": a title is required")
		}
		judged := len(s.Columns) == 0 || s.Columns[0].Kind != KindListing
		rows := map[string]bool{}
		for _, it := range s.Items {
			if err := checkItem(s, it, judged, rows, 1); err != nil {
				return errors.New("section " + s.ID + ": " + err.Error())
			}
		}
	}
	for _, h := range p.Hints {
		if err := checkHint(&h); err != nil {
			return err
		}
	}
	return nil
}

func checkItem(s Section, it Item, judged bool, rows map[string]bool, depth int) error {
	set := 0
	for _, ok := range []bool{it.Row != nil, it.Fact != nil, it.Group != nil, it.Empty != nil, it.Error != nil, it.Note != nil, it.Link != nil} {
		if ok {
			set++
		}
	}
	if set != 1 {
		return errors.New("an item holds exactly one node, not " + strconv.Itoa(set))
	}
	switch {
	case it.Row != nil:
		return checkRow(s, *it.Row, judged, rows)
	case it.Group != nil:
		if depth > 3 {
			return errors.New("a group nests 3 levels at most")
		}
		if err := checkRow(s, it.Group.Head, judged && !it.Group.Heading(), rows); err != nil {
			return err
		}
		for _, child := range it.Group.Items {
			if err := checkItem(s, child, judged, rows, depth+1); err != nil {
				return err
			}
		}
	case it.Empty != nil:
		return checkHint(it.Empty.Hint)
	case it.Error != nil:
		if it.Error.Hint == "" {
			return errors.New("an error carries a hint")
		}
	}
	return nil
}

func checkRow(s Section, r Row, judged bool, rows map[string]bool) error {
	if r.ID == "" || rows[r.ID] {
		return errors.New("row " + strconv.Quote(r.ID) + ": an ID unique in its section is required")
	}
	rows[r.ID] = true
	if judged && s.Role != Summary && r.Mark == Unjudged {
		return errors.New("row " + r.ID + ": a judged section's row carries a mark")
	}
	if s.Role != Summary && (r.Mark == Attention || r.Mark == Broken) && (r.Reason == "" || !explained(r)) {
		return errors.New("row " + r.ID + ": an attention or broken row carries a reason and a detail or a hint")
	}
	if len(s.Columns) > 0 && len(r.Cells) > len(s.Columns)-1 {
		return errors.New("row " + r.ID + ": more cells than columns")
	}
	for i, c := range r.Cells {
		if c.Kind != KindLevels {
			continue
		}
		for _, l := range c.Items {
			if l.Kind != KindSize || !contains(s.Columns[i+1].Levels, l.Label) {
				return errors.New("row " + r.ID + ": a levels cell holds only labeled sizes of its column's levels")
			}
		}
	}
	return checkHint(r.Hint)
}

func explained(r Row) bool {
	if !r.Detail.IsNone() || r.Hint != nil {
		return true
	}
	for _, c := range r.Cells {
		if c.Kind == KindText && c.Text != "" {
			return true
		}
	}
	return false
}

func checkHint(h *Hint) error {
	if h == nil {
		return nil
	}
	if h.Command == "" {
		return errors.New("a hint names its command")
	}
	if !isStation(h.Command) && !h.External {
		return errors.New("hint " + strconv.Quote(h.Command) + " is not station's: it is External")
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
