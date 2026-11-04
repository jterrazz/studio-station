package awake

type Reading struct {
	// value: "open", "closed", or "" on a Mac without a lid.
	Lid string
	// value: "ac" or "battery".
	Source     string
	HasBattery bool
	Percent    int
	Charging   bool
	LowPower   bool
	// value: the thermal pressure level, 0 nominal, 1 moderate, 2 heavy,
	// 3 serious (macOS says trapping), 4 sleeping.
	Thermal int
}

type Event struct {
	// value: "thermal", "source", "lid" or "battery" (its percent).
	Kind string
}

func (r Reading) OnBattery() bool { return r.HasBattery && r.Source == "battery" }

func (r Reading) LidClosedOnBattery() bool { return r.Lid == "closed" && r.OnBattery() }

func Level(n int) string {
	switch n {
	case 0:
		return "nominal"
	case 1:
		return "moderate"
	case ThermalHeavy:
		return "heavy"
	case ThermalEnds:
		return "serious"
	}
	return "sleeping"
}
