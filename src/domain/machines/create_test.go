package machines

import (
	"errors"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
)

func TestBridgedIsForMacOSImagesOnly(t *testing.T) {
	linux, macos := catalog.Image{Name: "linux", Kind: catalog.Linux}, catalog.Image{Name: "macos", Kind: catalog.MacOS}
	for _, c := range []struct {
		name   string
		img    catalog.Image
		asked  string
		want   string
		bridge bool
	}{
		{"a VM is behind its Mac by default", linux, "", NetworkNAT, false},
		{"a macOS VM may be bridged", macos, Bridged, Bridged, false},
		{"a linux VM is refused bridged", linux, Bridged, "", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := networkFor(c.img, c.asked)
			var bad *BadNetworkError
			if c.bridge {
				if !errors.As(err, &bad) || !bad.Bridge {
					t.Fatalf("networkFor(%s, %q) = %q, %v; want the bridged refusal", c.img.Name, c.asked, got, err)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("networkFor(%s, %q) = %q, %v; want %q", c.img.Name, c.asked, got, err, c.want)
			}
		})
	}
}
