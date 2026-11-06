package calls

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/images"
	"github.com/jterrazz/studio-station/src/machines/contract"
)

const (
	sshReady   = 60 * time.Second
	stopWait   = 120 * time.Second
	recipeWait = 30 * time.Minute
	pollEvery  = time.Second
	adminUser  = "admin"
	adminPass  = "admin"
	remoteBin  = "/tmp/station"
)

func (s *Service) recipeSHA(img catalog.Image) (string, error) {
	h := sha256.New()
	for _, file := range []string{img.Recipe, img.Cleanup, img.Firstboot} {
		data, err := catalog.Recipes.ReadFile(file)
		if err != nil {
			return "", err
		}
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Service) selection(ctx context.Context, names []string) ([]catalog.Image, error) {
	if len(names) > 0 {
		var out []catalog.Image
		for _, name := range names {
			img, err := s.image(name)
			if err != nil {
				return nil, err
			}
			if img.Recipe == "" {
				return nil, &images.NoRecipeError{Image: name}
			}
			if !slices.ContainsFunc(out, func(i catalog.Image) bool { return i.Name == name }) {
				out = append(out, img)
			}
		}
		return out, nil
	}
	var out []catalog.Image
	for _, img := range s.list {
		if img.Recipe == "" {
			continue
		}
		sha, err := s.recipeSHA(img)
		if err != nil {
			return nil, err
		}
		latest, err := s.Latest(ctx, img.Name)
		var missing *images.NotBuiltError
		switch {
		case errors.As(err, &missing):
			out = append(out, img)
		case err != nil:
			return nil, err
		case latest.RecipeSHA != sha:
			out = append(out, img)
		}
	}
	return out, nil
}

func (s *Service) Build(ctx context.Context, r images.BuildRequest, progress func(images.Step)) ([]images.Record, error) {
	wanted, err := s.selection(ctx, r.Images)
	if err != nil || len(wanted) == 0 {
		return nil, err
	}
	if need := s.needsBinary(r.Binary); need != nil {
		return nil, need
	}
	if _, err = s.e.Version(ctx); err != nil {
		return nil, &images.NoEngineError{Hypervisor: s.e.Name(), Cause: err}
	}
	binary, err := s.binary(ctx, r.Binary)
	if err != nil {
		return nil, err
	}
	if progress == nil {
		progress = func(images.Step) {}
	}
	var built []images.Record
	for _, img := range wanted {
		rec, err := s.buildOne(ctx, img, binary, progress)
		if err != nil {
			return built, err
		}
		built = append(built, rec)
	}
	return built, nil
}

type session struct {
	s   *Service
	vm  string
	at  netip.AddrPort
	key string
}

func (s *Service) buildOne(ctx context.Context, img catalog.Image, binary string, progress func(images.Step)) (images.Record, error) {
	vm := buildPrefix + img.Name
	failed := func(step string, cause error) (images.Record, error) {
		// invariant: a failed build leaves no VM behind; the cleanup survives a cancelled ctx.
		clean := context.WithoutCancel(ctx)
		_ = s.e.Shutdown(clean, vm, 10*time.Second)
		_ = s.e.Delete(clean, vm)
		return images.Record{}, &images.BuildFailedError{Image: img.Name, Step: step, Cause: cause}
	}
	version, err := s.nextVersion(img.Name)
	if err != nil {
		return images.Record{}, err
	}
	sha, err := s.recipeSHA(img)
	if err != nil {
		return images.Record{}, err
	}
	recipe, err := catalog.Recipes.ReadFile(img.Recipe)
	if err != nil {
		return images.Record{}, err
	}
	cleanup, err := catalog.Recipes.ReadFile(img.Cleanup)
	if err != nil {
		return images.Record{}, err
	}
	key, pub, err := s.imageKey(ctx)
	if err != nil {
		return images.Record{}, &images.BuildFailedError{Image: img.Name, Step: "making the image key", Cause: err}
	}
	if err = s.dropLeftover(ctx, vm); err != nil {
		return images.Record{}, &images.BuildFailedError{Image: img.Name, Step: "removing the earlier build VM", Cause: err}
	}

	progress(images.Step{Label: "pulling " + img.Base})
	if err = s.e.Fetch(ctx, s.location(img.Base), nil); err != nil {
		return failed("pulling the base", err)
	}
	progress(images.Step{Label: "cloning " + vm})
	if err = s.e.Clone(ctx, s.location(img.Base), vm); err != nil {
		return failed("cloning the base", err)
	}
	progress(images.Step{Label: "booting " + vm})
	if _, err = s.e.Boot(ctx, vm, contract.BootOptions{}); err != nil {
		return failed("booting the build VM", err)
	}
	addr, err := s.e.Address(ctx, vm, sshReady)
	if err != nil {
		return failed("waiting for the build VM's address", err)
	}
	ss := session{s: s, vm: vm, at: netip.AddrPortFrom(addr, s.guestPort()), key: key}

	progress(images.Step{Label: "copying station into " + vm})
	if err := ss.copy(ctx, binary); err != nil {
		return failed("copying the station binary", err)
	}
	progress(images.Step{Label: "running the " + img.Name + " recipe"})
	env := "VM_USER=" + quote(s.o.User) + " IMAGE_KEY=" + quote(strings.TrimSpace(pub))
	if err := ss.admin(ctx, string(recipe), env); err != nil {
		return failed("the recipe", err)
	}
	progress(images.Step{Label: "removing the base's admin user"})
	if err := ss.cleanup(ctx, string(cleanup), s.o.User); err != nil {
		return failed("the cleanup", err)
	}
	progress(images.Step{Label: "waiting for " + vm + " to power off"})
	if err := s.waitStopped(ctx, vm); err != nil {
		return failed("waiting for the build VM to power off", err)
	}
	_ = s.e.Shutdown(ctx, vm, 10*time.Second)
	progress(images.Step{Label: "recording " + img.Name + " " + version})
	final := DiskName(img.Name, version)
	if err := s.e.Rename(ctx, vm, final); err != nil {
		return failed("renaming the build VM", err)
	}
	rec := images.Record{Name: img.Name, Version: version, Base: img.Base, Station: s.o.Version, RecipeSHA: sha, BuiltAt: s.m.Now().UTC()}
	if disks, err := s.e.Disks(ctx); err == nil {
		for _, d := range disks {
			if d.VM == final {
				rec.SizeGB = int(d.Bytes / 1e9)
			}
		}
	}
	if err := s.writeRecord(rec); err != nil {
		return images.Record{}, &images.BuildFailedError{Image: img.Name, Step: "writing the image record", Cause: err}
	}
	return rec, nil
}

func (s *Service) dropLeftover(ctx context.Context, vm string) error {
	disks, err := s.e.Disks(ctx)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(disks, func(d contract.Disk) bool { return d.VM == vm }) {
		_ = s.e.Shutdown(ctx, vm, 10*time.Second)
		return s.e.Delete(ctx, vm)
	}
	return nil
}

func (s *Service) waitStopped(ctx context.Context, vm string) error {
	for range int(stopWait / pollEvery) {
		disks, err := s.e.Disks(ctx)
		if err != nil {
			return err
		}
		for _, d := range disks {
			if d.VM == vm && d.State == contract.DiskStopped {
				return nil
			}
		}
		if err := s.m.Sleep(ctx, pollEvery); err != nil {
			return err
		}
	}
	return fmt.Errorf("still running after %d s", int(stopWait/time.Second))
}

func (s *Service) KeyPath() string { return s.o.Key }

func (s *Service) imageKey(ctx context.Context) (key, pub string, err error) {
	key = s.KeyPath()
	if !s.m.Exists(key) {
		if made := s.m.MakeKey(ctx, key); made != nil {
			return "", "", made
		}
	}
	data, err := s.m.ReadFile(key + ".pub")
	if err != nil {
		return "", "", err
	}
	return key, string(data), nil
}

func (s *Service) guestPort() uint16 {
	if s.o.GuestPort > 0 && s.o.GuestPort <= math.MaxUint16 {
		return uint16(s.o.GuestPort)
	}
	return 22
}

// invariant: a build VM is a fresh clone, so its guest is dialed with no host
// key kept; the base's admin account is reached by its password until the
// recipe installs the image key, and only then as vm_user.
func (ss session) dial(ctx context.Context, user string, password bool) (contract.Guest, error) {
	t := contract.GuestTarget{VM: ss.vm, Address: ss.at, User: user, Key: ss.key}
	if password {
		t.Key, t.Password = "", adminPass
	}
	for range int(sshReady / pollEvery) {
		g, err := ss.s.guests.Dial(ctx, t)
		if err == nil {
			return g, nil
		}
		if err := ss.s.m.Sleep(ctx, pollEvery); err != nil {
			return nil, err
		}
	}
	return nil, errors.New("the build VM did not answer in " + sshReady.String())
}

func (ss session) copy(ctx context.Context, binary string) error {
	data, err := ss.s.m.ReadFile(binary)
	if err != nil {
		return err
	}
	g, err := ss.dial(ctx, adminUser, true)
	if err != nil {
		return err
	}
	defer func() { _ = g.Close() }()
	return g.Put(ctx, remoteBin, data, 0o755)
}

func (ss session) admin(ctx context.Context, script, env string) error {
	g, err := ss.dial(ctx, adminUser, true)
	if err != nil {
		return err
	}
	defer func() { _ = g.Close() }()
	ctx, cancel := context.WithTimeout(ctx, recipeWait)
	defer cancel()
	out, err := g.Exec(ctx, contract.GuestCommand{Line: "sudo env " + env + " bash -s", Stdin: strings.NewReader(script)})
	if err != nil {
		return err
	}
	if out.Code != 0 {
		return outputError(out)
	}
	return nil
}

func (ss session) cleanup(ctx context.Context, script, user string) error {
	g, err := ss.dial(ctx, user, false)
	if err != nil {
		return err
	}
	defer func() { _ = g.Close() }()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	out, err := g.Exec(ctx, contract.GuestCommand{Line: "sudo bash -s", Stdin: strings.NewReader(script)})
	// invariant: the cleanup powers the VM off under its own session, so the
	// session ends with 0 or is cut by the poweroff; any other code is a
	// failed cleanup.
	if err != nil && ctx.Err() != nil {
		return err
	}
	if err == nil && out.Code != 0 {
		return outputError(out)
	}
	return nil
}

func outputError(out contract.GuestResult) error {
	tail := strings.TrimSpace(out.Stderr)
	if tail == "" {
		tail = strings.TrimSpace(out.Stdout)
	}
	lines := strings.Split(tail, "\n")
	tail = strings.Join(lines[max(0, len(lines)-5):], "\n")
	return fmt.Errorf("exited %d: %s", out.Code, tail)
}

func quote(v string) string { return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'" }
