package catalog_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jterrazz/studio-station/src/catalog"
)

func TestEveryToolOfTheCatalogueReadsBackAsItWasSent(t *testing.T) {
	for _, tool := range catalog.Tools {
		raw, err := json.Marshal(tool)
		if err != nil {
			t.Fatalf("%s: %v", tool.Name, err)
		}
		var back catalog.Tool
		if err := json.Unmarshal(raw, &back); err != nil {
			t.Fatalf("%s: %v", tool.Name, err)
		}
		if !reflect.DeepEqual(back, tool) {
			t.Errorf("%s came back as %+v, want %+v", tool.Name, back.Installs, tool.Installs)
		}
	}
}
