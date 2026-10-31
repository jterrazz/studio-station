package clean

import (
	"context"
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/system"
)

func (s *Service) dockerBytes(ctx context.Context) (available bool, bytes int64) {
	out, err := s.h.Run.Output(ctx, system.Cmd{
		Name:    "docker",
		Args:    []string{"system", "df", "--format", "{{.Size}}"},
		Timeout: dockerDFBudget,
	})
	if err != nil || out.ExitCode != 0 {
		return false, 0
	}
	var sum int64
	for _, line := range strings.Split(out.Stdout, "\n") {
		sum += dockerSize(strings.TrimSpace(line))
	}
	return true, sum
}

// invariant: docker prints decimal units (kB, MB, GB, TB), the same ones present.Bytes prints.
func dockerSize(s string) int64 {
	for _, unit := range []struct {
		suffix string
		scale  float64
	}{{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3}, {"B", 1}} {
		if number, ok := strings.CutSuffix(s, unit.suffix); ok {
			value, err := strconv.ParseFloat(number, 64)
			if err != nil {
				return 0
			}
			return int64(value * unit.scale)
		}
	}
	return 0
}
