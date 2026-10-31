package images

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/domain/images"
	"github.com/jterrazz/studio-station/src/present"
)

type imageJSON struct {
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

type listJSON struct {
	Schema string      `json:"schema"`
	Images []imageJSON `json:"images"`
}

func List(reports []images.Report, context string) present.Answer {
	body := listJSON{Schema: "station.images/v1", Images: make([]imageJSON, 0, len(reports))}
	rows := make([]present.Row, 0, len(reports))
	for _, r := range reports {
		vms := append([]string{}, r.VMs...)
		body.Images = append(body.Images, imageJSON{
			Name:      r.Record.Name,
			Version:   r.Record.Version,
			Base:      r.Record.Base,
			Station:   r.Record.Station,
			RecipeSHA: r.Record.RecipeSHA,
			BuiltAt:   r.Record.BuiltAt.UTC().Format("2006-01-02T15:04:05Z"),
			SizeBytes: int64(r.Record.SizeGB) * 1_000_000_000,
			Newest:    r.Newest,
			VMs:       vms,
		})
		mark, note := present.MarkOK, ""
		if !r.Newest {
			mark, note = present.MarkAbsent, "older: station clean old-images"
		}
		rows = append(rows, present.Row{Mark: mark, Cells: []present.Cell{
			{Text: r.Record.Name, Tone: present.Strong},
			{Text: r.Record.Version},
			{Text: present.Bytes(int64(r.Record.SizeGB) * 1_000_000_000)},
			{Text: count(len(r.VMs), "VM")},
			{Text: note, Tone: present.Muted},
		}})
	}
	if len(rows) == 0 {
		return present.Answer{
			Doc: present.Document{
				Command: "station images",
				Context: context,
				Blocks:  []present.Block{present.Notice{Tone: present.Muted, Text: "none built"}},
				Hints:   []string{"station images build"},
			},
			JSON: body,
		}
	}
	return present.Answer{
		Doc: present.Document{
			Command: "station images",
			Context: context,
			Blocks:  []present.Block{present.Table{Rows: rows}},
			Hints:   []string{"station images build"},
		},
		JSON: body,
	}
}

func Built(records []images.Record, context string) present.Answer {
	rows := make([]present.Row, 0, len(records))
	for _, r := range records {
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: r.Name, Tone: present.Strong},
			{Text: r.Version},
			{Text: "built", Tone: present.Muted},
		}})
	}
	if len(rows) == 0 {
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{{Text: "nothing to build"}}})
	}
	return present.Answer{Doc: present.Document{
		Command: "station images build",
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}

func Deleted(records []images.Record, context string) present.Answer {
	rows := make([]present.Row, 0, len(records))
	for _, r := range records {
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: r.Name, Tone: present.Strong},
			{Text: r.Version},
			{Text: "deleted", Tone: present.Muted},
		}})
	}
	return present.Answer{Doc: present.Document{
		Command: "station images delete",
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}
