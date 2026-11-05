package atom

import (
	"strings"

	"github.com/jterrazz/studio-station/src/ui/token"
)

func Spark(samples []int) string {
	steps := []rune(token.Spark)
	out := make([]rune, 0, len(samples))
	for _, s := range samples {
		i := min(max(s, 0), 100) * (len(steps) - 1) / 100
		out = append(out, steps[i])
	}
	return string(out)
}

func Graph(samples []int, width int) string {
	// invariant: SPEC §6.4: a graph fits its width, each column the mean of
	// the points it covers, the newest at its right; fewer points than
	// columns draw one column each.
	if width <= 0 || len(samples) == 0 {
		return ""
	}
	if len(samples) <= width {
		return Spark(samples)
	}
	columns := make([]int, 0, width)
	for c := range width {
		from, to := c*len(samples)/width, (c+1)*len(samples)/width
		sum := 0
		for _, s := range samples[from:to] {
			sum += s
		}
		columns = append(columns, sum/max(to-from, 1))
	}
	return Spark(columns)
}

func Bar(used, total int64, width int) string {
	// invariant: the bar's shape carries the share, never its colour alone:
	// full blocks for the used part, light ones for the rest.
	if width <= 0 || total <= 0 {
		return ""
	}
	on := int(min(max(used, 0), total) * int64(width) / total)
	return strings.Repeat(token.GaugeOn, on) + strings.Repeat(token.GaugeOff, width-on)
}
