package catalog

type SizeProbe string

const (
	SizeDirs     SizeProbe = "dirs"
	SizeDockerDF SizeProbe = "docker-system-df"
	SizeVMs      SizeProbe = "vms"
	SizeImages   SizeProbe = "images"
)

type Cleanable struct {
	Name, Description string
	Places            []MachineKind
	Requires          string
	Size              SizeProbe
	Dirs              []string
	Steps             [][]string
	EmptyDirs         []string
}

var Cleanables = []Cleanable{
	{Name: "brew", Description: "The Homebrew download cache", Places: macKinds, Requires: "brew", Size: SizeDirs, Dirs: []string{"~/Library/Caches/Homebrew"}, Steps: [][]string{{"brew", "cleanup"}}},
	{Name: "docker", Description: "Stopped containers, dangling images, unused volumes, networks and build cache", Places: everyKind, Requires: "docker", Size: SizeDockerDF, Steps: [][]string{
		{"docker", "container", "prune", "-f"},
		{"docker", "image", "prune", "-f"},
		{"docker", "volume", "prune", "-f"},
		{"docker", "network", "prune", "-f"},
		{"docker", "builder", "prune", "-f"},
	}},
	{Name: "trash", Description: "The system trash", Places: macKinds, Size: SizeDirs, Dirs: []string{"~/.Trash"}, EmptyDirs: []string{"~/.Trash"}},
	{Name: "stale-vms", Description: "Temporary VMs left stopped by a power cut, a host reboot or a crash", Places: hostKind, Size: SizeVMs},
	{Name: "old-images", Description: "Image versions older than the newest, and the base pull cache", Places: hostKind, Size: SizeImages},
}
