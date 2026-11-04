package fake

import (
	"testing"
	"time"

	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/activity/contract/contracttest"
)

func TestMeter(t *testing.T) {
	contracttest.Meter(t, func(*testing.T) contracttest.MeterSubject {
		busy := activity.Sample{
			CPU:    []activity.Process{{PID: 2, Name: "node", Value: 12}, {PID: 1, Name: "station", Value: 40}},
			Memory: []activity.Process{{PID: 2, Name: "node", Value: 2e9}},
		}
		return contracttest.MeterSubject{Meter: NewMeter(busy, time.Hour, "100% charged", 16e9), Silent: NewMeter(activity.Sample{}, 0, "", 0)}
	})
}
