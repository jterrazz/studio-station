package catalog

import "embed"

type ImageDefaults struct {
	CPUs                   int
	MemoryBytes, DiskBytes int64
}

type Image struct {
	Name, Base, Recipe, Cleanup, Firstboot string
	Defaults                               ImageDefaults
	Kind                                   MachineKind
}

//go:embed images
var Recipes embed.FS

const gigabyte = 1000 * 1000 * 1000

var Images = []Image{
	{
		Name:      "linux",
		Base:      "ghcr.io/cirruslabs/ubuntu:24.04",
		Recipe:    "images/linux.sh",
		Cleanup:   "images/linux-cleanup.sh",
		Firstboot: "images/linux-firstboot.sh",
		Defaults:  ImageDefaults{CPUs: 4, MemoryBytes: 8 * gigabyte, DiskBytes: 50 * gigabyte},
		Kind:      Linux,
	},
}
