package images

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/page"
	"github.com/jterrazz/studio-station/src/pages"
)

type ImageJSON struct {
	Name      string   `json:"name"`
	Version   string   `json:"version"`
	Base      string   `json:"base"`
	Station   string   `json:"station"`
	RecipeSHA string   `json:"recipe_sha"`
	BuiltAt   string   `json:"built_at"`
	SizeBytes int64    `json:"size_bytes"`
	Newest    bool     `json:"newest"`
	VMs       []string `json:"vms"`
}

type ListJSON struct {
	Schema string      `json:"schema"`
	Images []ImageJSON `json:"images"`
}

func ListContract(reports []images.Report) ListJSON {
	body := ListJSON{Schema: "station.images/v1", Images: make([]ImageJSON, 0, len(reports))}
	for _, r := range reports {
		body.Images = append(body.Images, ImageJSON{
			Name:      r.Record.Name,
			Version:   r.Record.Version,
			Base:      r.Record.Base,
			Station:   r.Record.Station,
			RecipeSHA: r.Record.RecipeSHA,
			BuiltAt:   r.Record.BuiltAt.UTC().Format("2006-01-02T15:04:05Z"),
			SizeBytes: int64(r.Record.SizeGB) * 1_000_000_000,
			Newest:    r.Newest,
			VMs:       append([]string{}, r.VMs...),
		})
	}
	return body
}

func Columns() []page.Column {
	return []page.Column{
		{Key: "name", Kind: page.KindText, Keep: 100},
		{Key: "version", Label: "version", Kind: page.KindVersion, Keep: 50},
		{Key: "size", Label: "size", Kind: page.KindSize, Keep: 60},
		{Key: "vms", Label: "VMs", Kind: page.KindCount, Keep: 40},
	}
}

func Section(reports []images.Report) page.Section {
	items := make([]page.Item, 0, len(reports))
	for _, r := range reports {
		row := page.Row{
			ID: r.Record.Name + "@" + r.Record.Version, Label: r.Record.Name, Mark: page.OK,
			Cells:  []page.Value{page.Version(r.Record.Version), page.Size(int64(r.Record.SizeGB) * 1_000_000_000), vms(len(r.VMs))},
			Detail: page.Text(r.Record.Base),
		}
		if !r.Newest {
			row.Mark, row.Reason = page.Absent, page.Outdated
			row.Detail = page.Text("older: station images delete " + r.Record.Name + "@" + r.Record.Version)
		}
		items = append(items, page.Item{Row: &row})
	}
	if len(items) == 0 {
		items = append(items, page.Item{Empty: &page.Empty{Noun: "images", Hint: page.Cmd("station images build")}})
	}
	return page.Section{ID: "images", Title: "Images", Role: page.Banner, Columns: Columns(), Items: items}
}

func vms(n int) page.Value {
	if n == 1 {
		return page.Count(1, "VM")
	}
	return page.Count(int64(n), "VMs")
}

func List(reports []images.Report, here page.Context) page.Page {
	return page.Seal(page.Page{
		ID: "images", Command: "images", Context: here,
		Sections: []page.Section{Section(reports)},
		Hints:    []page.Hint{*page.Cmd("station images build")},
	})
}

func Built(records []images.Record, here page.Context) page.Page {
	return pages.Outcome("images build", here, versioned(), rows(records, "built"), "images to build")
}

func Deleted(records []images.Record, here page.Context) page.Page {
	return pages.Outcome("images delete", here, versioned(), rows(records, "deleted"), "images to delete")
}

func versioned() []page.Column {
	return []page.Column{{Key: "name", Kind: page.KindText, Keep: 100}, {Key: "version", Kind: page.KindVersion, Keep: 50}}
}

func rows(records []images.Record, done string) []page.Row {
	out := make([]page.Row, 0, len(records))
	for _, r := range records {
		row := pages.Done(r.Name+"@"+r.Version, catalog.StateOK, done)
		row.Label = r.Name
		row.Cells = []page.Value{page.Version(r.Version)}
		out = append(out, row)
	}
	return out
}
