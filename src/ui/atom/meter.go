package atom

import (
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
