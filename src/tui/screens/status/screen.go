package statusscreen

import (
	"context"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/jterrazz/studio-station/src/domain/status"
	"github.com/jterrazz/studio-station/src/output"
	"github.com/jterrazz/studio-station/src/present"
	presentstatus "github.com/jterrazz/studio-station/src/present/status"
	"github.com/jterrazz/studio-station/src/theme"
	"github.com/jterrazz/studio-station/src/tui/kit"
)

const (
	every   = 2 * time.Second
	history = 24
	barCols = 12
	sparks  = "▁▂▃▄▅▆▇█"
)

type Status interface {
	Watch(ctx context.Context) <-chan status.Snapshot
	Forget()
	Sample(ctx context.Context) (status.Sample, error)
}

type loaded struct {
	snapshot status.Snapshot
	gen      int
	more     <-chan status.Snapshot
	final    bool
}

type sampled struct {
	sample status.Sample
	err    error
}

type tick struct{}

type Model struct {
	ctx      context.Context
	status   Status
	describe kit.Describe
	failure  kit.Status
	theme    theme.Theme
	snapshot status.Snapshot
	loaded   bool
	sample   status.Sample
	rx, tx   []float64
	visible  bool
	ticking  bool
	offset   int
	spinner  spinner.Model
	gen      int
	stop     *context.CancelFunc
	loading  bool
}

func New(ctx context.Context, s Status, describe kit.Describe, th theme.Theme) Model {
	return Model{
		ctx:      ctx,
		status:   s,
		describe: describe,
		theme:    th,
		visible:  true,
		ticking:  true,
		loading:  true,
		stop:     new(context.CancelFunc),
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot), spinner.WithStyle(th.Accent)),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.receive(false), next())
}

func (m Model) watch(fresh bool) (Model, tea.Cmd) {
	m.gen, m.loading = m.gen+1, true
	return m, m.receive(fresh)
}

// invariant: Init runs on a copy, so the running watch's cancel is held where
// every copy of the screen sees it.
func (m Model) receive(fresh bool) tea.Cmd {
	if *m.stop != nil {
		(*m.stop)()
	}
	ctx, stop := context.WithCancel(m.ctx) //nolint:gosec // outside: held in *m.stop, called when the next watch starts.
	*m.stop = stop
	gen, s := m.gen, m.status
	return func() tea.Msg {
		if fresh {
			s.Forget()
		}
		return receive(gen, s.Watch(ctx))()
	}
}

func receive(gen int, ch <-chan status.Snapshot) tea.Cmd {
	return func() tea.Msg {
		snapshot, open := <-ch
		return loaded{snapshot: snapshot, gen: gen, more: ch, final: !open}
	}
}

func (m Model) measure() tea.Cmd {
	return func() tea.Msg {
		sample, err := m.status.Sample(m.ctx)
		return sampled{sample: sample, err: err}
	}
}

func next() tea.Cmd {
	return tea.Tick(every, func(time.Time) tea.Msg { return tick{} })
}

func (m Model) Focus() kit.Focus { return kit.FocusNormal }

func (m Model) Busy() bool { return false }

func (m Model) Loading() bool { return m.loading }

func (m Model) Keys() []key.Binding {
	return []key.Binding{kit.Bind("↑↓", "scroll", "up", "down", "k", "j"), kit.Bind("r", "reload", "r")}
}

func (m Model) Update(msg tea.Msg) (kit.Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.BackgroundColorMsg:
		m.theme = theme.New(msg.IsDark())
		m.spinner.Style = m.theme.Accent
	case loaded:
		if msg.gen != m.gen {
			return m, nil
		}
		if msg.final {
			m.loading = false
			return m, nil
		}
		if err := msg.snapshot.Err(); err != nil {
			text, hint := m.describe(err)
			m.failure = kit.Status{Mark: present.MarkBroken, Text: text, Hint: hint}
			return m, receive(msg.gen, msg.more)
		}
		m.failure = kit.Status{}
		m.snapshot, m.loaded = msg.snapshot, true
		m = m.record(msg.snapshot.System.Sample)
		return m, receive(msg.gen, msg.more)
	case sampled:
		if msg.err == nil {
			m = m.record(msg.sample)
		}
	case tick:
		if !m.visible {
			m.ticking = false
			return m, nil
		}
		return m, tea.Batch(m.measure(), next())
	case kit.Shown:
		m.visible = msg.Visible
		if m.visible && !m.ticking {
			m.ticking = true
			return m, tea.Batch(m.measure(), next())
		}
	case kit.Reload:
		return m.watch(true)
	case spinner.TickMsg:
		if !m.loaded {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			m.offset = max(m.offset-1, 0)
		case "down", "j":
			m.offset++
		case "g", "home":
			m.offset = 0
		}
	}
	return m, nil
}

// invariant: the counters a sample carries are totals since boot, so a rate needs
// the previous sample; the first one draws no rate.
func (m Model) record(s status.Sample) Model {
	if !m.sample.At.IsZero() && s.At.After(m.sample.At) {
		dt := s.At.Sub(m.sample.At).Seconds()
		m.rx = push(m.rx, float64(s.NetRx-m.sample.NetRx)/dt)
		m.tx = push(m.tx, float64(s.NetTx-m.sample.NetTx)/dt)
	}
	m.sample = s
	return m
}

func push(series []float64, v float64) []float64 {
	series = append(series, max(v, 0))
	if len(series) > history {
		series = series[len(series)-history:]
	}
	return series
}

func (m Model) View(width, height int) string {
	t := m.theme
	switch {
	case m.failure.Text != "":
		return strings.Join([]string{
			" " + kit.MarkText(t, present.MarkBroken) + " " + t.Bad.Render(m.failure.Text),
			"   " + t.Muted.Render(m.failure.Hint),
			"   " + t.Muted.Render("press r to retry"),
		}, "\n")
	case !m.loaded:
		return " " + m.spinner.View() + " " + t.Muted.Render("reading this machine")
	}
	live := m.live(width)
	body := strings.Split(strings.TrimRight(output.TTYBody(presentstatus.Snapshot(m.snapshot, "").Doc, width, t), "\n"), "\n")
	room := max(height-lipgloss.Height(live)-1, 1)
	start := min(m.offset, max(len(body)-room, 0))
	end := min(start+room, len(body))
	return live + "\n\n" + strings.Join(body[start:end], "\n")
}

func (m Model) live(width int) string {
	t := m.theme
	half := max((width-2)/2, 24)
	cpu := m.top("CPU", m.sample.CPU, func(p status.Process) (float64, string) {
		return p.Value / 100, strconv.FormatFloat(p.Value, 'f', 1, 64) + "%"
	}, half)
	total := float64(m.snapshot.System.MemoryTotal)
	memory := m.top("Memory", m.sample.Memory, func(p status.Process) (float64, string) {
		share := 0.0
		if total > 0 {
			share = p.Value / total
		}
		return share, present.Bytes(int64(p.Value))
	}, half)
	strip := lipgloss.JoinHorizontal(lipgloss.Top, cpu, memory)
	net := " " + t.Title.Render("Net") + "  " +
		t.Muted.Render("down ") + t.Plain.Render(rate(m.rx)) + " " + t.Accent.Render(spark(m.rx)) + "  " +
		t.Muted.Render("up ") + t.Plain.Render(rate(m.tx)) + " " + t.Accent.Render(spark(m.tx)) + "  " +
		t.Muted.Render("gpu ") + t.Plain.Render(strconv.FormatFloat(m.sample.GPU, 'f', 0, 64)+"%")
	return strip + "\n" + net
}

func (m Model) top(title string, ps []status.Process, measure func(status.Process) (float64, string), width int) string {
	t := m.theme
	lines := []string{" " + t.Title.Render(title)}
	nameWidth := max(width-barCols-14, 8)
	for i, p := range ps {
		if i == 5 {
			break
		}
		share, label := measure(p)
		filled := int(min(max(share, 0), 1)*barCols + 0.5)
		bar := t.Accent.Render(strings.Repeat("█", filled)) + t.Subtle.Render(strings.Repeat("░", barCols-filled))
		name := kit.Truncate(p.Name, nameWidth)
		lines = append(lines, "   "+t.Strong.Render(name)+strings.Repeat(" ", nameWidth-lipgloss.Width(name))+" "+bar+" "+t.Muted.Render(label))
	}
	return lipgloss.NewStyle().Width(width).Render(strings.Join(lines, "\n"))
}

func rate(series []float64) string {
	if len(series) == 0 {
		return "-"
	}
	return present.Bytes(int64(series[len(series)-1])) + "/s"
}

func spark(series []float64) string {
	peak := 0.0
	for _, v := range series {
		peak = max(peak, v)
	}
	levels := []rune(sparks)
	var b strings.Builder
	for _, v := range series {
		level := 0
		if peak > 0 {
			level = int(v / peak * float64(len(levels)-1))
		}
		b.WriteRune(levels[level])
	}
	return b.String()
}
