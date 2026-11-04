package main

import (
	"github.com/jterrazz/studio-station/src/cmd/generate/pages"
	"github.com/jterrazz/studio-station/src/page"
	pagejson "github.com/jterrazz/studio-station/src/ui/render/json"
)

const (
	pagesDir   = "src/ui/testdata/pages"
	streamPath = "src/ui/testdata/stream/menubar.jsonl"
	eventsPath = "src/ui/testdata/stream/events.jsonl"
)

// invariant: a fixture is written only after the page passes Seal's checks, so
// no renderer is proved against a page the binary could not build.
func pageFixtures() map[string][]byte {
	out := map[string][]byte{}
	for _, f := range pages.All() {
		if err := page.Check(f.Page); err != nil {
			panic(f.Name + ": " + err.Error())
		}
		out[f.Name+".json"] = pagejson.Render(f.Page)
	}
	return out
}
