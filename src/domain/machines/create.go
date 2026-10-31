package machines

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	onlineWait = 60 * time.Second
	onlinePoll = 2 * time.Second
	memoryCap  = 75
)

type CreateRequest struct {
	Image, Name string
	Lifetime    Lifetime
	Resources   system.Resources
}

type Step struct{ Label string }

type NoLifetimeError struct{}

func (*NoLifetimeError) Error() string { return "a VM needs a lifetime" }

type BadLifetimeError struct{ Value string }

func (e *BadLifetimeError) Error() string {
	return "unknown lifetime \"" + e.Value + "\": temporary or lasting"
}

type UnknownImageError struct{ Image string }

func (e *UnknownImageError) Error() string { return "unknown image \"" + e.Image + "\"" }

type NoImageError struct{ Image string }

func (e *NoImageError) Error() string { return "image " + e.Image + " is not built here" }

type NoEngineError struct{ Cause error }

func (*NoEngineError) Error() string { return "tart is not installed or does not answer" }

func (e *NoEngineError) Unwrap() error { return e.Cause }

type TooBigError struct{ MemoryGB, LimitGB int }

func (e *TooBigError) Error() string {
	return strconv.Itoa(e.MemoryGB) + " GB of memory is over this host's limit of " + strconv.Itoa(e.LimitGB) + " GB"
}

type CreateFailedError struct {
	Name, Step string
	Cause      error
}

func (e *CreateFailedError) Error() string {
	text := "creating " + e.Name + " failed at " + e.Step + ", and was rolled back"
	if e.Cause != nil {
		text += ": " + strings.TrimSpace(e.Cause.Error())
	}
	return text
}

func (e *CreateFailedError) Unwrap() error { return e.Cause }

type BadSizeError struct{ Flag, Value string }

func (e *BadSizeError) Error() string { return "--" + e.Flag + " \"" + e.Value + "\" is not a size" }

func ParseResources(cpus int, memory, disk string) (system.Resources, error) {
	// invariant: --memory counts in binary units and --disk in decimal ones, as
	// tart takes them; a bare number is gigabytes, and zero keeps the default.
	memoryBytes, err := parseSize("memory", memory, 1024)
	if err != nil {
		return system.Resources{}, err
	}
	diskBytes, err := parseSize("disk", disk, 1000)
	return system.Resources{CPUs: cpus, MemoryBytes: memoryBytes, DiskBytes: diskBytes}, err
}

func parseSize(flag, value string, base int64) (int64, error) {
	if value == "" {
		return 0, nil
	}
	text, scale := strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(value)), "B"), base*base*base
	if text != "" {
		if power := strings.IndexByte("KMGT", text[len(text)-1]); power >= 0 {
			scale = base
			for range power {
				scale *= base
			}
			text = text[:len(text)-1]
		}
	}
	n, err := strconv.ParseFloat(text, 64)
	if err != nil || n <= 0 {
		return 0, &BadSizeError{Flag: flag, Value: value}
	}
	return int64(n * float64(scale)), nil
}

func imageOf(name string) (catalog.Image, bool) {
	for _, img := range catalog.Images {
		if img.Name == name {
			return img, true
		}
	}
	return catalog.Image{}, false
}

func (s *Service) CheckCreate(ctx context.Context, r CreateRequest) (CreateRequest, error) {
	if err := s.CheckHost(); err != nil {
		return r, err
	}
	switch r.Lifetime {
	case Temporary, Lasting:
	case "":
		return r, &NoLifetimeError{}
	default:
		return r, &BadLifetimeError{Value: string(r.Lifetime)}
	}
	img, ok := imageOf(r.Image)
	if !ok {
		return r, &UnknownImageError{Image: r.Image}
	}
	if r.Name != "" && !validName(r.Name) {
		return r, &BadNameError{Name: r.Name}
	}
	if _, err := s.engine.Version(ctx); err != nil {
		return r, &NoEngineError{Cause: err}
	}
	if _, _, err := s.img.Latest(ctx, r.Image); err != nil {
		return r, err
	}
	records, nodes, err := s.claims(ctx)
	if err != nil {
		return r, err
	}
	switch {
	case r.Name == "":
		r.Name = allocateName(r.Image, records, nodes)
	case !nameFree(r.Name, records, nodes):
		return r, &NameTakenError{Name: r.Name, VM: true}
	}
	if r.Resources.CPUs == 0 {
		r.Resources.CPUs = img.Defaults.CPUs
	}
	if r.Resources.MemoryBytes == 0 {
		r.Resources.MemoryBytes = img.Defaults.MemoryBytes
	}
	if r.Resources.DiskBytes == 0 {
		r.Resources.DiskBytes = img.Defaults.DiskBytes
	}
	if limit, ok := s.memoryLimit(ctx); ok && r.Resources.MemoryBytes > limit {
		return r, &TooBigError{MemoryGB: int(r.Resources.MemoryBytes >> 30), LimitGB: int(limit >> 30)}
	}
	return r, nil
}

func (s *Service) claims(ctx context.Context) (records map[string]bool, nodes map[string]Node, err error) {
	fleet, err := s.Fleet(ctx, false)
	if err != nil {
		return nil, nil, err
	}
	records = map[string]bool{}
	for _, h := range fleet.Hosts {
		records[h.Name] = true
		for _, vm := range h.VMs {
			records[vm.Name] = true
		}
	}
	cfg, err := s.store.Load()
	if err != nil {
		return nil, nil, err
	}
	for name := range cfg.Hosts {
		records[name] = true
	}
	return records, s.nodes(ctx), nil
}

func (s *Service) memoryLimit(ctx context.Context) (int64, bool) {
	out, err := s.h.Run.Output(ctx, system.Cmd{Name: "sysctl", Args: []string{"-n", "hw.memsize"}})
	if err != nil || out.ExitCode != 0 {
		return 0, false
	}
	total, err := strconv.ParseInt(strings.TrimSpace(out.Stdout), 10, 64)
	if err != nil || total <= 0 {
		return 0, false
	}
	return total * memoryCap / 100, true
}

func (s *Service) Create(ctx context.Context, r CreateRequest, progress func(Step)) (VM, error) {
	// invariant: the key is minted first, so a refused mint creates nothing; any
	// failure after it rolls every made thing back.
	r, err := s.CheckCreate(ctx, r)
	if err != nil {
		return VM{}, err
	}
	say := func(label string) {
		if progress != nil {
			progress(Step{Label: label})
		}
	}
	cfg, err := s.store.Load()
	if err != nil {
		return VM{}, err
	}
	img, _ := imageOf(r.Image)
	tartName, version, err := s.img.Latest(ctx, r.Image)
	if err != nil {
		return VM{}, err
	}
	say("minting a tailnet key")
	key, err := s.tn.Mint(ctx)
	if err != nil {
		return VM{}, err
	}
	rec := VMRecord{
		Name: r.Name, Image: r.Image, Version: version, Lifetime: r.Lifetime,
		Resources: r.Resources, CreatedAt: s.h.Clock.Now().UTC().Truncate(time.Second),
	}
	user := s.vmUser(cfg)
	made := &made{}
	fail := func(step string, cause error) (VM, error) {
		s.undoCreate(ctx, r.Name, made)
		return VM{}, &CreateFailedError{Name: r.Name, Step: step, Cause: cause}
	}

	say("cloning " + tartName)
	if err = s.engine.Clone(ctx, tartName, r.Name); err != nil {
		return fail("the clone", err)
	}
	made.vm = true
	if err = s.engine.Resize(ctx, r.Name, r.Resources); err != nil {
		return fail("the resize", err)
	}
	if err = s.writeRecord(rec); err != nil {
		return fail("the record", err)
	}
	made.record = true
	say("starting its agent")
	made.agent = true
	if err = s.installAgent(ctx, r.Name, r.Lifetime); err != nil {
		return fail("the launchd agent", err)
	}
	say("first boot")
	ip, err := s.engine.IP(ctx, r.Name, ipWait)
	if err != nil {
		return fail("the first boot", err)
	}
	nodeID, err := s.firstBoot(ctx, img, ip, user, r.Name, r.Lifetime, key)
	if err != nil {
		return fail("the first boot", err)
	}
	made.nodeID = nodeID
	say("joining the tailnet")
	node, err := s.awaitOnline(ctx, r.Name)
	if err != nil {
		return fail("the tailnet join", err)
	}
	rec.NodeID = nodeID
	if err := s.writeRecord(rec); err != nil {
		return fail("the record", err)
	}
	if fleet, err := s.Fleet(ctx, false); err == nil {
		_ = s.SSHBlocks(ctx, fleet)
	}
	return VM{
		Name: r.Name, Image: r.Image, Version: version, Host: s.here(cfg), Kind: img.Kind, Lifetime: r.Lifetime,
		State: Running, Resources: r.Resources, Node: node, CreatedAt: rec.CreatedAt,
	}, nil
}

type made struct {
	vm, record, agent bool
	nodeID            string
}

// invariant: the rollback is best effort and runs every step whatever fails, so a
// half-made VM leaves as little as it can; what it reports is the create's failure.
func (s *Service) undoCreate(ctx context.Context, name string, m *made) {
	if m.nodeID != "" {
		_ = s.tn.Remove(ctx, m.nodeID)
	}
	if m.agent {
		_, _ = s.launchctl(ctx, "bootout", s.agentTarget(name))
	}
	if m.vm {
		if s.engineState(ctx, name) == system.VMRunning {
			_ = s.engine.Stop(ctx, name, hardStopWait)
		}
		_ = s.engine.Delete(ctx, name)
	}
	s.removeFiles(name, m.record)
}

func (s *Service) removeFiles(name string, record bool) {
	files := []string{s.plistPath(name), s.logPath(name), s.consolePath(name), path.Join(s.knownHostsDir(), name)}
	if record {
		files = append(files, s.recordPath(name))
	}
	for _, f := range files {
		_ = s.h.FS.Remove(f)
	}
}

func (s *Service) awaitOnline(ctx context.Context, name string) (Node, error) {
	for waited := time.Duration(0); ; waited += onlinePoll {
		if node, ok := s.nodes(ctx)[name]; ok && node.Online {
			return node, nil
		}
		if waited >= onlineWait {
			return Node{}, errors.New(name + " did not come online on the tailnet in " + onlineWait.String())
		}
		if err := s.h.Clock.Sleep(ctx, onlinePoll); err != nil {
			return Node{}, err
		}
	}
}

func (s *Service) recordPath(name string) string { return path.Join(s.recordsDir(), name+".json") }

func (s *Service) writeRecord(r VMRecord) error {
	if err := s.h.FS.MkdirAll(s.recordsDir(), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(recordFile{
		Name: r.Name, Image: r.Image, Version: r.Version, NodeID: r.NodeID, Lifetime: r.Lifetime,
		CPUs: r.Resources.CPUs, MemoryBytes: r.Resources.MemoryBytes, DiskBytes: r.Resources.DiskBytes,
		Lent: r.Lent, CreatedAt: r.CreatedAt,
	}, "", "  ")
	if err != nil {
		return err
	}
	return s.h.FS.WriteFile(s.recordPath(r.Name), append(data, '\n'), 0o600)
}
