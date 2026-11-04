package activity

import (
	"strconv"
	"time"
)

type Process struct {
	PID   int
	Name  string
	Value float64
}

type Sample struct {
	// invariant: never nil: a list nothing answered is empty.
	CPU, Memory  []Process
	NetRx, NetTx int64
	GPU          float64
	At           time.Time
}

const Top = 5

func Uptime(d time.Duration) string {
	// value: the uptime the owner reads: "22d 14h", "3h 5m", "12m".
	minutes := int(d.Minutes())
	days, hours := minutes/(24*60), minutes/60%24
	switch {
	case days > 0:
		return strconv.Itoa(days) + "d " + strconv.Itoa(hours) + "h"
	case hours > 0:
		return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes%60) + "m"
	}
	return strconv.Itoa(minutes) + "m"
}
