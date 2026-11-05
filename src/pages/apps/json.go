package apps

import (
	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/catalog"
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

type AppJSON struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Method   string `json:"method"`
	State    string `json:"state"`
	// value: the version installed, null for an app that is not.
	Installed *string  `json:"installed"`
	Newest    string   `json:"newest,omitempty"`
	LivesOn   string   `json:"lives_on"`
	Provides  []string `json:"provides"`
	Detail    string   `json:"detail"`
}

type ListJSON struct {
	Schema string    `json:"schema"`
	Apps   []AppJSON `json:"apps"`
	// value: how many catalogue apps live on other machines.
	Elsewhere int `json:"not_for_this_machine"`
}

func ListContract(schema string, reports []apps.Report) ListJSON {
	// invariant: every array is built with make, so an empty one encodes as [].
	body := ListJSON{Schema: schema, Apps: make([]AppJSON, 0, len(reports)), Elsewhere: len(apps.Elsewhere(reports))}
	for _, r := range reports {
		detail := r.Detail
		if detail == "" {
			detail = r.Live
		}
		body.Apps = append(body.Apps, AppJSON{
			Name: r.Tool.Name, Category: string(r.Tool.Category), Method: string(r.Method),
			State: string(apps.StateOf(r)), Installed: installed(r.Version), Newest: r.Newest, LivesOn: apps.LivesOnToken(r.Tool.Places()),
			Provides: append([]string{}, r.Tool.Provides...), Detail: detail,
		})
	}
	return body
}

func ToolObject(r apps.Report) ToolJSON {
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

func installed(version string) *string {
	if version == "" {
		return nil
	}
	return &version
}
