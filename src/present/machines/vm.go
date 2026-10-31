package machines

import (
	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/present"
)

const createdSchema = "station.machines.created/v1"

type createdJSON struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Host        string `json:"host"`
	Lifetime    string `json:"lifetime"`
	Image       string `json:"image"`
	Version     string `json:"version"`
	CPUs        int    `json:"cpus"`
	MemoryBytes int64  `json:"memory_bytes"`
	DiskBytes   int64  `json:"disk_bytes"`
	IP          string `json:"ip"`
}

func Created(vm machines.VM, context string) present.Answer {
	// invariant: the name is the second field of the piped row, so `cut -f2`
	// names the VM a script just made.
	r := vm.Resources
	return present.Answer{
		Doc: present.Document{
			Command: "station machines create",
			Context: context,
			Blocks: []present.Block{present.Table{Rows: []present.Row{{Mark: present.MarkOK, Cells: []present.Cell{
				{Text: vm.Name, Tone: present.Strong},
				{Text: vm.Host, Tone: present.Muted},
				{Text: string(vm.Lifetime)},
				{Text: vm.Image + " " + vm.Version, Tone: present.Muted},
				{Text: plural(r.CPUs, "cpu") + " · " + memory(r.MemoryBytes), Tone: present.Muted},
			}}}}},
			Hints: []string{"ssh " + vm.Name},
		},
		JSON: createdJSON{
			Schema: createdSchema, Name: vm.Name, Host: vm.Host, Lifetime: string(vm.Lifetime), Image: vm.Image,
			Version: vm.Version, CPUs: r.CPUs, MemoryBytes: r.MemoryBytes, DiskBytes: r.DiskBytes, IP: vm.Node.IP,
		},
	}
}

func Deleted(r machines.DeleteReport, context string) present.Answer {
	rows := make([]present.Row, 0, 1+len(r.Warnings))
	rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
		{Text: r.Name, Tone: present.Strong},
		{Text: "deleted", Tone: present.Muted},
	}})
	for _, w := range r.Warnings {
		rows = append(rows, present.Row{Mark: present.MarkDegraded, Cells: []present.Cell{
			{Text: r.Name, Tone: present.Strong},
			{Text: w, Tone: present.Warn},
		}})
	}
	return present.Answer{Doc: present.Document{
		Command: "station machines delete " + r.Name,
		Context: context,
		Blocks:  []present.Block{present.Table{Rows: rows}},
	}}
}
