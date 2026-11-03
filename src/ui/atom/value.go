package atom

import (
	"strconv"
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/ui/token"
)

const versionMax = 18

func Plain(v page.Value) string {
	switch v.Kind {
	case page.KindPortMap:
		return strconv.FormatInt(v.N, 10)
	case page.KindList:
		return join(v.Items, Plain)
	case page.KindLevels:
		parts := make([]string, 0, len(v.Items))
		for _, l := range v.Items {
			parts = append(parts, l.Label+" "+Plain(l))
		}
		return strings.Join(parts, token.Sep)
	case page.KindPending:
		return ""
	default:
	}
	return show(v)
}

func Show(v page.Value) string {
	switch v.Kind {
	case page.KindVersion:
		return End(v.Text, versionMax)
	case page.KindPortMap:
		return strconv.FormatInt(v.N, 10) + " " + token.Arrow + " " + strconv.FormatInt(v.Of, 10)
	case page.KindList:
		return join(v.Items, Show)
	case page.KindPending:
		return ""
	default:
	}
	return show(v)
}

func show(v page.Value) string {
	switch v.Kind {
	case page.KindNone, "", page.KindPending, page.KindListing:
		return ""
	case page.KindCount:
		n := strconv.FormatInt(v.N, 10)
		if v.Unit == "" {
			return n
		}
		return n + " " + v.Unit
	case page.KindSize:
		if v.Unit != "" {
			return Bytes(v.N) + " " + v.Unit
		}
		return Bytes(v.N)
	case page.KindRatio:
		used, total := unitOf(v.Of)
		return used.of(v.N) + "/" + total
	case page.KindPercent:
		if v.N < 100 && v.N%10 != 0 {
			return strconv.FormatInt(v.N/10, 10) + "." + strconv.FormatInt(v.N%10, 10) + "%"
		}
		return strconv.FormatInt(v.N/10, 10) + "%"
	case page.KindDuration:
		return Duration(v.N)
	case page.KindAge:
		if v.N > 30*86400 && len(v.At) >= 10 {
			return v.At[:10]
		}
		return Duration(v.N)[:durationHead(v.N)] + " ago"
	case page.KindPort:
		return strconv.FormatInt(v.N, 10)
	case page.KindRange:
		return strconv.FormatInt(v.N, 10) + "-" + strconv.FormatInt(v.Of, 10)
	case page.KindOn:
		return "on"
	case page.KindOff:
		return "off"
	case page.KindUnknown:
		return token.Unknown
	case page.KindLevels:
		return Plain(v)
	case page.KindList:
		return join(v.Items, show)
	case page.KindText, page.KindVersion, page.KindURL, page.KindIdentity, page.KindPath, page.KindCommand, page.KindPortMap:
	}
	return v.Text
}

func join(items []page.Value, f func(page.Value) string) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		if s := f(it); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, token.Sep)
}

func Numeric(k page.Kind) bool {
	switch k {
	case page.KindCount, page.KindSize, page.KindRatio, page.KindPercent, page.KindPort, page.KindPortMap, page.KindDuration:
		return true
	default:
	}
	return false
}

func Floor(k page.Kind) int {
	switch k {
	case page.KindVersion:
		return 8
	case page.KindURL:
		return 24
	case page.KindPath:
		return 12
	case page.KindText, page.KindList, page.KindIdentity:
		return 6
	default:
	}
	return 1 << 10
}

func Fit(v page.Value, width int) string {
	s := Show(v)
	if Width(s) <= width {
		return s
	}
	switch v.Kind {
	case page.KindPath:
		return path(s, width)
	case page.KindURL:
		return url(s, width)
	case page.KindList:
		return list(v.Items, width)
	case page.KindText, page.KindIdentity, page.KindVersion:
		return End(s, width)
	default:
	}
	return s
}

func path(s string, width int) string {
	parts := strings.Split(s, "/")
	if len(parts) > 3 {
		head := parts[0]
		if head == "" {
			head = "/" + parts[1]
		}
		short := head + "/" + token.Ellipsis + "/" + parts[len(parts)-1]
		if Width(short) <= width {
			return short
		}
	}
	return Middle(s, width)
}

func url(s string, width int) string {
	scheme, rest, ok := strings.Cut(s, "://")
	if !ok {
		return Middle(s, width)
	}
	host, _, _ := strings.Cut(rest, "/")
	port := ""
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host, port = host[:i], host[i:]
	}
	label, _, _ := strings.Cut(host, ".")
	short := scheme + "://" + label + token.Ellipsis + port
	if Width(short) <= width {
		return short
	}
	return End(short, width)
}

func list(items []page.Value, width int) string {
	for n := len(items) - 1; n > 0; n-- {
		s := join(items[:n], Show) + " " + token.Ellipsis
		if Width(s) <= width {
			return s
		}
	}
	return End(join(items, Show), width)
}

func Bytes(n int64) string {
	// invariant: decimal units, as Finder, df -H and tart print sizes, so a size
	// station shows reads the same as the one the owner checks it against.
	const unit = 1000
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	value, suffix := float64(n)/unit, 0
	for value >= unit-0.05 && suffix < len(byteSuffixes)-1 {
		value /= unit
		suffix++
	}
	return strconv.FormatFloat(value, 'f', 1, 64) + " " + string(byteSuffixes[suffix]) + "B"
}

const byteSuffixes = "KMGTPE"

type scale struct {
	div  float64
	unit string
}

func (s scale) of(n int64) string { return strconv.FormatFloat(float64(n)/s.div, 'f', 1, 64) }

func unitOf(total int64) (unit scale, text string) {
	s := scale{div: 1, unit: "B"}
	for i, div := 0, 1000.0; i < len(byteSuffixes) && float64(total) >= div; i, div = i+1, div*1000 {
		s = scale{div: div, unit: string(byteSuffixes[i]) + "B"}
	}
	return s, s.of(total) + " " + s.unit
}

func Duration(seconds int64) string {
	units := []struct {
		n    int64
		name string
	}{{86400, "d"}, {3600, "h"}, {60, "m"}, {1, "s"}}
	var parts []string
	rest := seconds
	for _, u := range units {
		if rest >= u.n && len(parts) < 2 {
			parts = append(parts, strconv.FormatInt(rest/u.n, 10)+u.name)
			rest %= u.n
		} else if len(parts) == 1 {
			break
		}
	}
	if len(parts) == 0 {
		return "0s"
	}
	return strings.Join(parts, " ")
}

func durationHead(seconds int64) int {
	d := Duration(seconds)
	if i := strings.IndexByte(d, ' '); i > 0 {
		return i
	}
	return len(d)
}
