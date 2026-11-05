package imagesscreen

import (
	"strings"

	"github.com/jterrazz/studio-station/src/page"
	pageimages "github.com/jterrazz/studio-station/src/pages/images"

	"github.com/jterrazz/studio-station/src/surface/tui/screens/gesture"
)

// value: an image row is `<mac>/<name>@<version>`, or a name not built yet.
func parts(r page.Row) (mac, name, version string) {
	id := r.ID
	if m, rest, ok := strings.Cut(id, "/"); ok {
		mac, id = m, rest
	}
	name, version, _ = strings.Cut(id, "@")
	return mac, name, version
}

func Gesture(here page.Context, a page.Action, rows []page.Row) (gesture.Gesture, bool) {
	on := func(mac string, w []string) []string {
		if mac != "" && mac != here.Machine {
			w = append(w, "--on", mac)
		}
		return w
	}
	switch a.Key {
	case pageimages.KeyBuild:
		return gesture.Each(rows, gesture.V("Build", "Building", "Built"), func(r page.Row) []string {
			mac, name, _ := parts(r)
			return on(mac, []string{"images", "build", name})
		}), true
	case pageimages.KeyUpdate:
		return gesture.Each(rows, gesture.V("Update", "Updating", "Updated"), func(r page.Row) []string {
			mac, name, _ := parts(r)
			return on(mac, []string{"images", "update", name})
		}), true
	case pageimages.KeyDelete:
		var g gesture.Gesture
		for _, r := range rows {
			mac, name, version := parts(r)
			if version == "" {
				continue
			}
			g.Steps = append(g.Steps, gesture.V("Delete", "Deleting", "Deleted").Step(name+" "+version, on(mac, []string{"images", "delete", name + "@" + version})))
		}
		return g, len(g.Steps) > 0
	}
	return gesture.Gesture{}, false
}
