package machines

type Added struct {
	Name, Label string
	// value: the station version the Mac answered with.
	Version string
	// value: what brings the two stations to one version, when they differ.
	Update string
}
