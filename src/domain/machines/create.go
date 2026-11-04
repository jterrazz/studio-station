package machines

import (
	"context"
	"errors"
	model "github.com/jterrazz/studio-station/src/machines"
	"strconv"
	"strings"
	"time"

	imagescontract "github.com/jterrazz/studio-station/src/images/contract"
	"github.com/jterrazz/studio-station/src/machines/contract"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const memoryCap = 75

var ErrDaemonDown = errors.New("the engine is not running")

type DaemonDownError struct {
	Host      string
	Installed bool
}

func (e *DaemonDownError) Error() string {
	if e.Installed {
		return "the engine is stopped on " + e.Host
	}
	return "the engine is not installed on " + e.Host
}

type BadNetworkError struct {
	Value  string
	Bridge bool
}

func (e *BadNetworkError) Error() string {
	if e.Bridge {
		return "--network bridged is for macOS images: a linux workspace stays behind its Mac"
	}
	return "unknown network \"" + e.Value + "\": nat or bridged"
}

type (
	CreateRequest = model.CreateRequest
	Created       = model.Created
	Step          = model.Step
)

type BadLifetimeError struct{ Value string }

func (e *BadLifetimeError) Error() string {
	return "unknown lifetime \"" + e.Value + "\": throwaway or permanent"
}

type UnknownImageError struct{ Image string }

func (e *UnknownImageError) Error() string { return "unknown image \"" + e.Image + "\"" }

type NoImageError struct{ Image string }

func (e *NoImageError) Error() string { return "image " + e.Image + " is not built here" }

type NoEngineError struct {
	Hypervisor string
	Cause      error
}

func (e *NoEngineError) Error() string { return e.Hypervisor + " is not installed or does not answer" }

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

func ParseResources(cpus int, memory, disk string) (Resources, error) {
	// invariant: --memory counts in binary units and --disk in decimal ones, as
	// the hypervisor takes them; a size says its unit, and none keeps the default.
	memoryBytes, err := parseSize("memory", memory, 1024)
	if err != nil {
		return Resources{}, err
	}
	diskBytes, err := parseSize("disk", disk, 1000)
	return Resources{CPUs: cpus, MemoryBytes: memoryBytes, DiskBytes: diskBytes}, err
}

type SizeNeedsUnitError struct{ Flag, Value string }

func (e *SizeNeedsUnitError) Error() string { return "\"" + e.Value + "\" needs a unit" }

func parseSize(flag, value string, base int64) (int64, error) {
	if value == "" {
		return 0, nil
	}
	text, scale := strings.TrimSuffix(strings.ToUpper(strings.TrimSpace(value)), "B"), int64(0)
	if text != "" {
		if power := strings.IndexByte("KMGT", text[len(text)-1]); power >= 0 {
			scale = base
			for range power {
				scale *= base
			}
			text = text[:len(text)-1]
		}
	}
	if _, err := strconv.ParseFloat(text, 64); err == nil && scale == 0 {
		return 0, &SizeNeedsUnitError{Flag: flag, Value: value}
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
	case "":
		r.Lifetime = Throwaway
	case Throwaway, Permanent:
	default:
		return r, &BadLifetimeError{Value: string(r.Lifetime)}
	}
	img, ok := imageOf(r.Image)
	if !ok {
		return r, &UnknownImageError{Image: r.Image}
	}
	switch r.Network {
	case "":
		r.Network = NetworkNAT
	case NetworkNAT:
	case Bridged:
		if img.Kind != catalog.MacOS {
			return r, &BadNetworkError{Value: r.Network, Bridge: true}
		}
	default:
		return r, &BadNetworkError{Value: r.Network}
	}
	if r.Name != "" && !validName(r.Name) {
		return r, &BadNameError{Name: r.Name}
	}
	if _, err := s.engine.Version(ctx); err != nil {
		return r, &NoEngineError{Hypervisor: s.engine.Name(), Cause: err}
	}
	if _, _, err := s.img.Latest(ctx, r.Image); err != nil {
		return r, err
	}
	held, on, err := s.claims(ctx)
	if err != nil {
		return r, err
	}
	switch {
	case r.Name == "":
		r.Name = allocateName(r.Image, held)
	case !nameFree(r.Name, held):
		return r, &NameTakenError{Name: r.Name, VM: held[r.Name] == claimVM, Host: held[r.Name] == claimHost, On: on[r.Name]}
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
	if err := s.EngineUp(ctx); err != nil {
		return r, err
	}
	return r, nil
}

func (s *Service) EngineUp(ctx context.Context) error {
	_, missing := s.h.FS.Stat(catalog.EnginePlist(s.h.Env.Home()))
	down := &DaemonDownError{Host: "This Mac", Installed: missing == nil}
	if s.daemon == nil {
		return down
	}
	if _, err := s.daemon.Ping(ctx); err != nil {
		if errors.Is(err, ErrDaemonDown) {
			return down
		}
		return err
	}
	return nil
}

// value: every name All machines holds, and for a VM's name the Mac it is
// on, worded as the owner reads it: This Mac, or that Mac's short name.
func (s *Service) claims(ctx context.Context) (held map[string]claim, on map[string]string, err error) {
	fleet, err := s.Fleet(ctx, false)
	if err != nil {
		return nil, nil, err
	}
	held, on = map[string]claim{}, map[string]string{}
	for _, h := range fleet.Hosts {
		held[h.Name] = claimHost
		mac := "This Mac"
		if !h.Here {
			mac = strings.TrimPrefix(h.Name, s.o.User+"-")
		}
		for _, vm := range h.VMs {
			held[vm.Name], on[vm.Name] = claimVM, mac
		}
	}
	cfg, err := s.prefs()
	if err != nil {
		return nil, nil, err
	}
	for name := range cfg.Hosts {
		held[name] = claimHost
	}
	return held, on, nil
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

func (s *Service) Create(ctx context.Context, r CreateRequest, progress func(Step)) (Created, error) {
	// invariant: the profiles' ports are reserved before the VM first runs, and
	// their rendered variables go into the record, fixed for the VM's life; any
	// failure after the clone rolls every made thing back.
	r, err := s.CheckCreate(ctx, r)
	if err != nil {
		return Created{}, err
	}
	say := func(label string) {
		if progress != nil {
			progress(Step{Label: label})
		}
	}
	cfg, err := s.prefs()
	if err != nil {
		return Created{}, err
	}
	img, _ := imageOf(r.Image)
	disk, version, err := s.img.Latest(ctx, r.Image)
	if err != nil {
		return Created{}, err
	}
	rec := VMRecord{
		Name: r.Name, Image: r.Image, Version: version, Lifetime: r.Lifetime, Network: r.Network,
		Resources: r.Resources, CreatedAt: s.h.Clock.Now().UTC().Truncate(time.Second),
	}
	if r.Lifetime == Permanent {
		rec.Want = WantRunning
	}
	made := &made{}
	fail := func(step string, cause error) (Created, error) {
		s.undoCreate(ctx, r.Name, made)
		return Created{}, &CreateFailedError{Name: r.Name, Step: step, Cause: cause}
	}

	say("cloning " + disk)
	if err = s.engine.Clone(ctx, imagescontract.ImageLocation{Hypervisor: s.engine.Name(), Ref: disk}, r.Name); err != nil {
		return fail("the clone", err)
	}
	made.vm = true
	if err = s.engine.Resize(ctx, r.Name, r.Resources.Size()); err != nil {
		return fail("the resize", err)
	}
	if err = s.writeRecord(rec); err != nil {
		return fail("the record", err)
	}
	made.record = true
	say("reserving its services")
	made.leases = true
	reserved, env, err := s.reserveProfiles(ctx, r.Name)
	if err != nil {
		return fail("the reservations", err)
	}
	rec.Env = env
	if err = s.writeRecord(rec); err != nil {
		return fail("the record", err)
	}
	say("starting it")
	made.running = true
	if err = s.daemon.Do(ctx, "run", r.Name); err != nil {
		return fail("the start", err)
	}
	say("first boot")
	addr, err := s.engine.Address(ctx, r.Name, ipWait)
	if err != nil {
		return fail("the first boot", err)
	}
	ip := addr.String()
	if err := s.firstBoot(ctx, img, r.Name, r.Lifetime); err != nil {
		return fail("the first boot", err)
	}
	if fleet, err := s.Fleet(ctx, false); err == nil {
		_ = s.SSHBlocks(ctx, fleet)
	}
	return Created{
		VM: VM{
			Name: r.Name, Image: r.Image, Version: version, Host: s.here(cfg), Kind: img.Kind, Lifetime: r.Lifetime,
			State: Running, Resources: r.Resources, Network: r.Network, IP: ip, CreatedAt: rec.CreatedAt,
		},
		Services: reserved,
		Env:      env,
	}, nil
}

func (s *Service) reserveProfiles(ctx context.Context, name string) (reserved []Reserved, env map[string]string, err error) {
	for _, p := range catalog.Profiles {
		got, err := s.daemon.Reserve(ctx, name, p.Port, p.Name)
		if err != nil {
			return nil, nil, err
		}
		reserved = append(reserved, got)
	}
	return reserved, ProfileEnv(reserved), nil
}

func ProfileEnv(reserved []Reserved) map[string]string {
	env := map[string]string{}
	publicHost := ""
	for _, r := range reserved {
		publicHost = r.PublicHost
		for _, p := range catalog.Profiles {
			if p.Name != r.Profile {
				continue
			}
			for _, e := range p.Env {
				env[e.Name] = render(e.Value, r)
			}
		}
	}
	if publicHost != "" {
		env[catalog.PublicHostEnv.Name] = render(catalog.PublicHostEnv.Value, Reserved{PublicHost: publicHost})
	}
	return env
}

func render(template string, r Reserved) string {
	return strings.NewReplacer("{url}", r.URL, "{host}", r.PublicHost, "{port}", strconv.Itoa(r.HostPort)).Replace(template)
}

type made struct{ vm, record, leases, running bool }

// invariant: the rollback is best effort and runs every step whatever fails, so a
// half-made VM leaves as little as it can; what it reports is the create's failure.
func (s *Service) undoCreate(ctx context.Context, name string, m *made) {
	if m.running {
		_ = s.daemon.Do(ctx, "forget", name)
	}
	if m.leases && s.rel != nil {
		_ = s.rel.ReleaseVM(ctx, name)
	}
	if m.vm {
		if s.engineState(ctx, name) == contract.DiskRunning {
			_ = s.engine.Shutdown(ctx, name, hardStopWait)
		}
		_ = s.engine.Delete(ctx, name)
	}
	s.removeFiles(name, m.record)
}

func (s *Service) removeFiles(name string, record bool) {
	for _, f := range []string{s.LogPath(name), s.KnownHostsPath(name)} {
		_ = s.h.FS.Remove(f)
	}
	if record {
		_ = s.recs.Delete(context.Background(), name)
	}
}

func (s *Service) Save(r VMRecord) error { return s.writeRecord(r) }

func (s *Service) writeRecord(r VMRecord) error { return s.recs.Put(context.Background(), storedOf(r)) }
