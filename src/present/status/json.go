package status

import (
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/status"
	presentconfig "github.com/jterrazz/studio-station/src/present/config"
	presentstorage "github.com/jterrazz/studio-station/src/present/storage"
	presenttailnet "github.com/jterrazz/studio-station/src/present/tailnet"
	presenttools "github.com/jterrazz/studio-station/src/present/tools"
)

const schema = "station.status/v1"

type snapshotJSON struct {
	Schema      string                   `json:"schema"`
	GeneratedAt string                   `json:"generated_at"`
	Station     stationJSON              `json:"station"`
	Machine     machineJSON              `json:"machine"`
	System      systemJSON               `json:"system"`
	Environment environmentJSON          `json:"environment"`
	Workspace   workspaceJSON            `json:"workspace"`
	Tools       []presenttools.ToolJSON  `json:"tools"`
	Config      []presentconfig.ItemJSON `json:"config"`
	Setup       setupJSON                `json:"setup"`
	Identity    []checkJSON              `json:"identity"`
	Daemons     []checkJSON              `json:"daemons"`
	Gateways    []checkJSON              `json:"gateways"`
	Tailnet     presenttailnet.Object    `json:"tailnet"`
}

type stationJSON struct {
	Version string     `json:"version"`
	Commit  string     `json:"commit"`
	Daemon  daemonJSON `json:"daemon"`
}

type daemonJSON struct {
	State   string `json:"state"`
	Running bool   `json:"running"`
	Version string `json:"version"`
}

type machineJSON struct {
	Name     *string `json:"name"`
	Kind     string  `json:"kind"`
	Hostname string  `json:"hostname"`
	OS       string  `json:"os"`
	Arch     string  `json:"arch"`
	User     string  `json:"user"`
	Shell    string  `json:"shell"`
}

type processJSON struct {
	PID   int     `json:"pid"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type memoryJSON struct {
	TotalBytes int64         `json:"total_bytes"`
	Top        []processJSON `json:"top"`
}

type systemJSON struct {
	Uptime  string        `json:"uptime"`
	Battery string        `json:"battery"`
	CPU     []processJSON `json:"cpu"`
	Memory  memoryJSON    `json:"memory"`
}

type checkJSON struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Value  string `json:"value"`
	Detail string `json:"detail"`
}

type portJSON struct {
	PID  int    `json:"pid"`
	Name string `json:"name"`
	Port string `json:"port"`
}

type environmentJSON struct {
	Network  []checkJSON `json:"network"`
	Ports    []portJSON  `json:"ports"`
	Security []checkJSON `json:"security"`
	Updates  []checkJSON `json:"updates"`
}

type repositoryJSON struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Dirty  int    `json:"dirty"`
}

type dependencyJSON struct {
	Path     string `json:"path"`
	Manager  string `json:"manager"`
	Outdated int    `json:"outdated"`
}

type dockerJSON struct {
	State         string `json:"state"`
	Containers    int    `json:"containers"`
	Images        int    `json:"images"`
	Volumes       int    `json:"volumes"`
	DanglingBytes int64  `json:"dangling_bytes"`
	TotalBytes    int64  `json:"total_bytes"`
}

type diskJSON struct {
	Name  string `json:"name"`
	Level string `json:"level"`
	Bytes int64  `json:"bytes"`
}

type workspaceJSON struct {
	Repositories []repositoryJSON           `json:"repositories"`
	Dependencies []dependencyJSON           `json:"dependencies"`
	Docker       dockerJSON                 `json:"docker"`
	Disk         []diskJSON                 `json:"disk"`
	Strays       []presentstorage.StrayJSON `json:"strays"`
}

type stepJSON struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

type setupJSON struct {
	State string     `json:"state"`
	Steps []stepJSON `json:"steps"`
}

// invariant: every array is built with make, so an empty one encodes as [], never null.
func jsonOf(s status.Snapshot) snapshotJSON {
	body := snapshotJSON{
		Schema:      schema,
		GeneratedAt: s.GeneratedAt.UTC().Format(time.RFC3339),
		Station:     stationJSON{Version: s.Station.Version, Commit: s.Station.Commit, Daemon: daemonOf(s.Station.Daemon)},
		Machine: machineJSON{
			Kind: string(s.Machine.Kind), Hostname: s.Machine.Hostname, OS: s.Machine.OS,
			Arch: s.Machine.Arch, User: s.Machine.User, Shell: s.Machine.Shell,
		},
		System: systemJSON{
			Uptime: s.System.Uptime, Battery: s.System.Battery,
			CPU:    processes(s.System.Sample.CPU),
			Memory: memoryJSON{TotalBytes: s.System.MemoryTotal, Top: processes(s.System.Sample.Memory)},
		},
		Environment: environmentJSON{
			Network:  checks(s.Environment.Network),
			Ports:    make([]portJSON, 0, len(s.Environment.Ports)),
			Security: checks(s.Environment.Security),
			Updates:  checks(s.Environment.Updates),
		},
		Workspace: workspaceJSON{
			Repositories: make([]repositoryJSON, 0, len(s.Workspace.Repositories)),
			Dependencies: make([]dependencyJSON, 0, len(s.Workspace.Dependencies)),
			Docker: dockerJSON{
				State: string(s.Workspace.Docker.State), Containers: s.Workspace.Docker.Containers,
				Images: s.Workspace.Docker.Images, Volumes: s.Workspace.Docker.Volumes,
				DanglingBytes: s.Workspace.Docker.DanglingBytes, TotalBytes: s.Workspace.Docker.TotalBytes,
			},
			Disk:   make([]diskJSON, 0, len(s.Workspace.Disk)),
			Strays: presentstorage.Strays(s.Workspace.Strays),
		},
		Tools:    make([]presenttools.ToolJSON, 0, len(s.Tools)),
		Config:   make([]presentconfig.ItemJSON, 0, len(s.Config)),
		Setup:    setupJSON{State: string(s.Setup.State), Steps: make([]stepJSON, 0, len(s.Setup.Steps))},
		Identity: checks(s.Identity),
		Daemons:  checks(s.Daemons),
		Gateways: checks(s.Gateways),
		Tailnet:  presenttailnet.ObjectOf(s.Tailnet),
	}
	if s.Machine.Registered {
		name := s.Machine.Name
		body.Machine.Name = &name
	}
	for _, p := range s.Environment.Ports {
		body.Environment.Ports = append(body.Environment.Ports, portJSON{PID: p.PID, Name: p.Name, Port: p.Port})
	}
	for _, r := range s.Workspace.Repositories {
		body.Workspace.Repositories = append(body.Workspace.Repositories, repositoryJSON{Path: r.Path, Branch: r.Branch, Dirty: r.Dirty})
	}
	for _, d := range s.Workspace.Dependencies {
		body.Workspace.Dependencies = append(body.Workspace.Dependencies, dependencyJSON{Path: d.Path, Manager: d.Manager, Outdated: d.Outdated})
	}
	for _, d := range s.Workspace.Disk {
		body.Workspace.Disk = append(body.Workspace.Disk, diskJSON{Name: d.Name, Level: string(d.Level), Bytes: d.Bytes})
	}
	for _, t := range s.Tools {
		body.Tools = append(body.Tools, presenttools.ToolObject(t))
	}
	for _, c := range s.Config {
		body.Config = append(body.Config, presentconfig.ItemObject(c))
	}
	for _, step := range s.Setup.Steps {
		body.Setup.Steps = append(body.Setup.Steps, stepJSON{Name: step.Name, State: string(step.State), Detail: step.Detail})
	}
	return body
}

func checks(in []status.Check) []checkJSON {
	out := make([]checkJSON, 0, len(in))
	for _, c := range in {
		out = append(out, checkJSON{Name: c.Name, State: string(c.State), Value: c.Value, Detail: c.Detail})
	}
	return out
}

func processes(in []status.Process) []processJSON {
	out := make([]processJSON, 0, len(in))
	for _, p := range in {
		out = append(out, processJSON{PID: p.PID, Name: p.Name, Value: p.Value})
	}
	return out
}

func daemonOf(c status.Check) daemonJSON {
	d := daemonJSON{State: string(c.State), Running: c.State == catalog.StateOK || strings.HasPrefix(c.Value, "runs ")}
	if d.State == "" {
		d.State = string(catalog.StateAbsent)
	}
	if v, ok := strings.CutPrefix(c.Value, "running "); ok {
		d.Version = v
	} else if v, ok := strings.CutPrefix(c.Value, "runs "); ok {
		d.Version, _, _ = strings.Cut(v, ",")
	}
	return d
}
