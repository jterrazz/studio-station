package machines

import (
	"strconv"

	"github.com/jterrazz/studio-station/src/domain/machines"
	"github.com/jterrazz/studio-station/src/present"
)

const createdSchema = "station.machines.created/v1"

type createdJSON struct {
	Schema      string            `json:"schema"`
	Name        string            `json:"name"`
	Host        string            `json:"host"`
	Lifetime    string            `json:"lifetime"`
	Image       string            `json:"image"`
	Version     string            `json:"version"`
	CPUs        int               `json:"cpus"`
	MemoryBytes int64             `json:"memory_bytes"`
	DiskBytes   int64             `json:"disk_bytes"`
	IP          string            `json:"ip"`
	Network     string            `json:"network"`
	Services    []createdService  `json:"services"`
	Env         map[string]string `json:"env"`
}

type createdService struct {
	GuestPort int    `json:"guest_port"`
	HostPort  int    `json:"host_port"`
	Profile   string `json:"profile"`
	URL       string `json:"url"`
}

func Created(c machines.Created, context string) present.Answer {
	// invariant: the name is the second field of the piped row, so `cut -f2`
	// names the VM a script just made; each profile's URL follows on its own row.
	vm, r := c.VM, c.VM.Resources
	rows := make([]present.Row, 0, 1+len(c.Services))
	rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
		{Text: vm.Name, Tone: present.Strong},
		{Text: vm.Host, Tone: present.Muted},
		{Text: string(vm.Lifetime)},
		{Text: vm.Image + " " + vm.Version, Tone: present.Muted},
		{Text: plural(r.CPUs, "cpu") + " · " + memory(r.MemoryBytes), Tone: present.Muted},
	}})
	body := createdJSON{
		Schema: createdSchema, Name: vm.Name, Host: vm.Host, Lifetime: string(vm.Lifetime), Image: vm.Image,
		Version: vm.Version, CPUs: r.CPUs, MemoryBytes: r.MemoryBytes, DiskBytes: r.DiskBytes, IP: vm.IP,
		Network: vm.Network, Services: []createdService{}, Env: c.Env,
	}
	if body.Env == nil {
		body.Env = map[string]string{}
	}
	for _, s := range c.Services {
		body.Services = append(body.Services, createdService{GuestPort: s.GuestPort, HostPort: s.HostPort, Profile: s.Profile, URL: s.URL})
		rows = append(rows, present.Row{Mark: present.MarkOK, Cells: []present.Cell{
			{Text: s.Profile, Tone: present.Strong},
			{Text: strconv.Itoa(s.GuestPort), Tone: present.Muted},
			{Text: "→ " + s.URL},
		}})
	}
	return present.Answer{
		Doc: present.Document{
			Command: "station machines create",
			Context: context,
			Blocks:  []present.Block{present.Table{Rows: rows}},
			Hints:   []string{"ssh " + vm.Name},
		},
		JSON: body,
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
