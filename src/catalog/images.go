package catalog

import "embed"

type ImageDefaults struct {
	CPUs                   int
	MemoryBytes, DiskBytes int64
}

type Image struct {
	// value: Recipe, Cleanup and Firstboot empty: station builds no such
	// image; it is only fetched where it is published.
	Name, Base, Recipe, Cleanup, Firstboot string
	Defaults                               ImageDefaults
	Kind                                   MachineKind
}

//go:embed images
var Recipes embed.FS

// invariant: memory counts in binary gigabytes, as macOS and tart count RAM; a
// disk in decimal ones, as tart's --disk-size does.
const (
	gigabyte = 1000 * 1000 * 1000
	gibibyte = 1 << 30
)

var Images = []Image{
	{
		Name:      "linux",
		Base:      "ghcr.io/cirruslabs/ubuntu:24.04",
		Recipe:    "images/linux.sh",
		Cleanup:   "images/linux-cleanup.sh",
		Firstboot: "images/linux-firstboot.sh",
		Defaults:  ImageDefaults{CPUs: 4, MemoryBytes: 8 * gibibyte, DiskBytes: 50 * gigabyte},
		Kind:      Linux,
	},
	{
		Name:     "macos",
		Defaults: ImageDefaults{CPUs: 4, MemoryBytes: 8 * gibibyte, DiskBytes: 80 * gigabyte},
		Kind:     MacOS,
	},
}
