package tools

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/tools"
)

type ToolJSON struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Method   string   `json:"method"`
	Places   []string `json:"places"`
	State    string   `json:"state"`
	Version  string   `json:"version"`
	Managed  bool     `json:"managed"`
	Provides []string `json:"provides"`
	Detail   string   `json:"detail"`
	Use      string   `json:"use"`
}

type CatalogueJSON struct {
	Schema string     `json:"schema"`
	Tools  []ToolJSON `json:"tools"`
}

func CatalogueContract(reports []tools.Report) CatalogueJSON {
	body := CatalogueJSON{Schema: "station.tools/v1", Tools: make([]ToolJSON, 0, len(reports))}
	for _, r := range reports {
		body.Tools = append(body.Tools, ToolObject(r))
	}
	return body
}

func ToolObject(r tools.Report) ToolJSON {
	kinds := r.Tool.Places()
	places := make([]string, 0, len(kinds))
	for _, k := range kinds {
		places = append(places, string(k))
	}
	detail := r.Detail
	if detail == "" {
		detail = r.Live
	}
	return ToolJSON{
		Name:     r.Tool.Name,
		Category: string(r.Tool.Category),
		Method:   string(r.Method),
		Places:   places,
		State:    string(r.State),
		Version:  r.Version,
		Managed:  r.Managed,
		Provides: append([]string{}, r.Tool.Provides...),
		Detail:   detail,
		Use:      use(r.Tool),
	}
}

func use(t catalog.Tool) string {
	if t.System {
		return "system"
	}
	return "exploration"
}
