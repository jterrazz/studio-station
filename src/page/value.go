package page

import "time"

func Text(s string) Value {
	if s == "" {
		return None()
	}
	return Value{Kind: KindText, Text: s}
}

func Count(n int64, unit string) Value { return Value{Kind: KindCount, N: n, Unit: unit} }

func Size(bytes int64) Value { return Value{Kind: KindSize, N: bytes} }

func Ratio(used, total int64) Value { return Value{Kind: KindRatio, N: used, Of: total} }

func Percent(tenths int64) Value { return Value{Kind: KindPercent, N: tenths} }

func Version(s string) Value {
	if s == "" {
		return None()
	}
	return Value{Kind: KindVersion, Text: s}
}

func Upgrade(from, to string) Value {
	// value: a version and the newer one that waits for it (`2.97.0 → 2.98.0`);
	// the pipe keeps the version alone, the field it always had.
	if to == "" || from == "" {
		return Version(from)
	}
	return Value{Kind: KindVersion, Text: from, Label: to}
}

func Duration(seconds int64) Value { return Value{Kind: KindDuration, N: seconds} }

func Age(t, now time.Time) Value {
	return Value{Kind: KindAge, At: t.UTC().Format(time.RFC3339), N: int64(now.Sub(t).Seconds())}
}

func Until(t, now time.Time) Value {
	// value: the time left until t, as seen at now (a page never reads the
	// clock): negative once t has passed.
	return Value{Kind: KindUntil, At: t.UTC().Format(time.RFC3339), N: int64(t.Sub(now).Seconds())}
}

func URL(s string) Value { return Value{Kind: KindURL, Text: s} }

func Identity(s string) Value { return Value{Kind: KindIdentity, Text: s} }

func Path(s string) Value { return Value{Kind: KindPath, Text: s} }

func Port(n int64) Value { return Value{Kind: KindPort, N: n} }

func PortMap(guest, host int64) Value { return Value{Kind: KindPortMap, N: guest, Of: host} }

func Range(low, high int64) Value { return Value{Kind: KindRange, N: low, Of: high} }

func Command(s string) Value { return Value{Kind: KindCommand, Text: s} }

func List(vs ...Value) Value {
	if len(vs) == 0 {
		return None()
	}
	return Value{Kind: KindList, Items: vs}
}

func Texts(words ...string) Value {
	vs := make([]Value, 0, len(words))
	for _, w := range words {
		if w != "" {
			vs = append(vs, Text(w))
		}
	}
	return List(vs...)
}

func Levels(labeled ...Value) Value { return Value{Kind: KindLevels, Items: labeled} }

func Labeled(label string, v Value) Value {
	v.Label = label
	return v
}

func On() Value { return Value{Kind: KindOn} }

func Off() Value { return Value{Kind: KindOff} }

func Unknown() Value { return Value{Kind: KindUnknown} }

func Pending() Value { return Value{Kind: KindPending} }

func None() Value { return Value{Kind: KindNone} }

func Stale(v Value) Value {
	v.Stale = true
	return v
}

func (v Value) IsNone() bool { return v.Kind == "" || v.Kind == KindNone }

func Cmd(command string) *Hint {
	return &Hint{Command: command, External: !isStation(command)}
}

func KeyHint(command, key string) *Hint {
	h := Cmd(command)
	h.Key = key
	return h
}

func isStation(command string) bool {
	return len(command) > len("station ") && command[:len("station ")] == "station "
}
