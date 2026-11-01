package images

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

type BuildRequest struct {
	Images []string
	Binary string
}

type Step struct{ Label string }

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
			if !slices.ContainsFunc(out, func(i catalog.Image) bool { return i.Name == name }) {
				out = append(out, img)
			}
		}
		return out, nil
	}
	var out []catalog.Image
	for _, img := range s.list {
		sha, err := s.recipeSHA(img)
		if err != nil {
			return nil, err
		}
		latest, err := s.Latest(ctx, img.Name)
		var missing *NotBuiltError
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

func (s *Service) Build(ctx context.Context, r BuildRequest, progress func(Step)) ([]Record, error) {
	if err := s.checkHost(); err != nil {
		return nil, err
	}
	wanted, err := s.selection(ctx, r.Images)
	if err != nil || len(wanted) == 0 {
		return nil, err
	}
	if need := s.needsBinary(r.Binary); need != nil {
		return nil, need
	}
	if _, err = s.e.Version(ctx); err != nil {
		return nil, &NoEngineError{Cause: err}
	}
	binary, err := s.binary(ctx, r.Binary)
	if err != nil {
		return nil, err
	}
	if progress == nil {
		progress = func(Step) {}
	}
	var built []Record
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
	s       *Service
	ip      string
	key     string
	askpass string
}

func (s *Service) buildOne(ctx context.Context, img catalog.Image, binary string, progress func(Step)) (Record, error) {
	vm := buildPrefix + img.Name
	// invariant: the console only serves the run of this build, so no build leaves it behind.
	defer func() { _ = s.h.FS.Remove(s.consolePath(img.Name)) }()
	failed := func(step string, cause error) (Record, error) {
		// invariant: a failed build leaves no VM behind; the cleanup survives a cancelled ctx.
		clean := context.WithoutCancel(ctx)
		_ = s.e.Stop(clean, vm, 10*time.Second)
		_ = s.e.Delete(clean, vm)
		return Record{}, &BuildFailedError{Image: img.Name, Step: step, Cause: cause}
	}
	version, err := s.nextVersion(img.Name)
	if err != nil {
		return Record{}, err
	}
	sha, err := s.recipeSHA(img)
	if err != nil {
		return Record{}, err
	}
	recipe, err := catalog.Recipes.ReadFile(img.Recipe)
	if err != nil {
		return Record{}, err
	}
	cleanup, err := catalog.Recipes.ReadFile(img.Cleanup)
	if err != nil {
		return Record{}, err
	}
	key, pub, err := s.imageKey(ctx)
	if err != nil {
		return Record{}, &BuildFailedError{Image: img.Name, Step: "making the image key", Cause: err}
	}
	askpass, err := s.askpass()
	if err != nil {
		return Record{}, &BuildFailedError{Image: img.Name, Step: "writing the ssh helper", Cause: err}
	}
	if err = s.dropLeftover(ctx, vm); err != nil {
		return Record{}, &BuildFailedError{Image: img.Name, Step: "removing the earlier build VM", Cause: err}
	}

	progress(Step{Label: "pulling " + img.Base})
	if err = s.e.Pull(ctx, img.Base); err != nil {
		return failed("pulling the base", err)
	}
	progress(Step{Label: "cloning " + vm})
	if err = s.e.Clone(ctx, img.Base, vm); err != nil {
		return failed("cloning the base", err)
	}
	progress(Step{Label: "booting " + vm})
	if err = s.boot(vm, img.Name); err != nil {
		return failed("booting the build VM", err)
	}
	ip, err := s.e.IP(ctx, vm, sshReady)
	if err != nil {
		return failed("waiting for the build VM's address", err)
	}
	ss := session{s: s, ip: ip, key: key, askpass: askpass}

	progress(Step{Label: "copying station into " + vm})
	if err := ss.copy(ctx, binary); err != nil {
		return failed("copying the station binary", err)
	}
	progress(Step{Label: "running the " + img.Name + " recipe"})
	env := "VM_USER=" + quote(s.o.User) + " IMAGE_KEY=" + quote(strings.TrimSpace(pub))
	if err := ss.admin(ctx, string(recipe), env); err != nil {
		return failed("the recipe", err)
	}
	progress(Step{Label: "removing the base's admin user"})
	if err := ss.cleanup(ctx, string(cleanup), s.o.User); err != nil {
		return failed("the cleanup", err)
	}
	progress(Step{Label: "waiting for " + vm + " to power off"})
	if err := s.waitStopped(ctx, vm); err != nil {
		return failed("waiting for the build VM to power off", err)
	}
	_ = s.e.Stop(ctx, vm, 10*time.Second)
	progress(Step{Label: "recording " + img.Name + " " + version})
	final := TartName(img.Name, version)
	if err := s.e.Rename(ctx, vm, final); err != nil {
		return failed("renaming the build VM", err)
	}
	rec := Record{Name: img.Name, Version: version, Base: img.Base, Station: s.o.Version, RecipeSHA: sha, BuiltAt: s.h.Clock.Now().UTC()}
	if vms, err := s.e.List(ctx); err == nil {
		for _, info := range vms {
			if info.Name == final {
				rec.SizeGB = info.SizeGB
			}
		}
	}
	if err := s.writeRecord(rec); err != nil {
		return Record{}, &BuildFailedError{Image: img.Name, Step: "writing the image record", Cause: err}
	}
	return rec, nil
}

func (s *Service) dropLeftover(ctx context.Context, vm string) error {
	vms, err := s.e.List(ctx)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(vms, func(i system.VMInfo) bool { return i.Name == vm }) {
		_ = s.e.Stop(ctx, vm, 10*time.Second)
		return s.e.Delete(ctx, vm)
	}
	return nil
}

func (s *Service) consolePath(image string) string {
	return path.Join(s.o.Paths.State, "consoles", buildPrefix+image)
}

func (s *Service) boot(vm, image string) error {
	console := s.consolePath(image)
	logs := path.Join(s.o.Paths.State, "logs")
	if err := s.h.FS.MkdirAll(path.Dir(console), 0o700); err != nil {
		return err
	}
	if err := s.h.FS.MkdirAll(logs, 0o700); err != nil {
		return err
	}
	// invariant: tart opens --serial-path for reading and writing and refuses to run when the file is missing.
	if err := s.h.FS.WriteFile(console, nil, 0o600); err != nil {
		return err
	}
	argv := s.e.RunArgv(vm, console)
	_, err := s.h.Run.Start(system.Cmd{Name: argv[0], Args: argv[1:]}, path.Join(logs, buildPrefix+image+".log"))
	return err
}

func (s *Service) waitStopped(ctx context.Context, vm string) error {
	for range int(stopWait / pollEvery) {
		vms, err := s.e.List(ctx)
		if err != nil {
			return err
		}
		for _, info := range vms {
			if info.Name == vm && info.State == system.VMStopped {
				return nil
			}
		}
		if err := s.h.Clock.Sleep(ctx, pollEvery); err != nil {
			return err
		}
	}
	return fmt.Errorf("still running after %d s", int(stopWait/time.Second))
}

func (s *Service) KeyPath() string { return path.Join(s.o.Paths.State, "images", "key") }

func (s *Service) imageKey(ctx context.Context) (key, pub string, err error) {
	key = s.KeyPath()
	if _, statErr := s.h.FS.Stat(key); statErr != nil {
		if mkErr := s.h.FS.MkdirAll(path.Dir(key), 0o700); mkErr != nil {
			return "", "", mkErr
		}
		var out system.Output
		out, err = s.h.Run.Output(ctx, system.Cmd{Name: "ssh-keygen", Args: []string{"-q", "-t", "ed25519", "-N", "", "-C", "station-image", "-f", key}})
		if err != nil {
			return "", "", err
		}
		if out.ExitCode != 0 {
			return "", "", fmt.Errorf("ssh-keygen exited %d: %s", out.ExitCode, strings.TrimSpace(out.Stderr))
		}
	}
	data, err := s.h.FS.ReadFile(key + ".pub")
	if err != nil {
		return "", "", err
	}
	return key, string(data), nil
}

func (s *Service) askpass() (string, error) {
	file := path.Join(s.o.Paths.State, "images", "askpass")
	if err := s.h.FS.MkdirAll(path.Dir(file), 0o700); err != nil {
		return "", err
	}
	if err := s.h.FS.WriteFile(file, []byte("#!/bin/sh\necho "+adminPass+"\n"), 0o700); err != nil {
		return "", err
	}
	return file, s.h.FS.Chmod(file, 0o700)
}

func natOptions() []string {
	return []string{"-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-o", "LogLevel=ERROR"}
}

func (ss session) passwordEnv() []string {
	return []string{"SSH_ASKPASS=" + ss.askpass, "SSH_ASKPASS_REQUIRE=force"}
}

func (ss session) passwordOptions() []string {
	return append(natOptions(), "-o", "BatchMode=no", "-o", "PreferredAuthentications=password", "-o", "PubkeyAuthentication=no", "-o", "NumberOfPasswordPrompts=1")
}

func (ss session) keyOptions(user string) []string {
	return append(natOptions(), "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-i", ss.key, user+"@"+ss.ip)
}

func (ss session) copy(ctx context.Context, binary string) error {
	argv := append(ss.passwordOptions(), binary, adminUser+"@"+ss.ip+":"+remoteBin)
	for range int(sshReady / pollEvery) {
		out, err := ss.s.h.Run.Output(ctx, system.Cmd{Name: "scp", Args: argv, Env: ss.passwordEnv(), Timeout: 5 * time.Minute})
		if err != nil {
			return err
		}
		if out.ExitCode == 0 {
			return nil
		}
		if out.ExitCode != 255 {
			return outputError("scp", out)
		}
		if err := ss.s.h.Clock.Sleep(ctx, pollEvery); err != nil {
			return err
		}
	}
	return errors.New("sshd did not answer in " + sshReady.String())
}

func (ss session) admin(ctx context.Context, script, env string) error {
	argv := append(ss.passwordOptions(), adminUser+"@"+ss.ip, "sudo", "env", env, "bash", "-s")
	out, err := ss.s.h.Run.Output(ctx, system.Cmd{Name: "ssh", Args: argv, Env: ss.passwordEnv(), Stdin: script, Timeout: recipeWait})
	if err != nil {
		return err
	}
	if out.ExitCode != 0 {
		return outputError("ssh", out)
	}
	return nil
}

func (ss session) cleanup(ctx context.Context, script, user string) error {
	argv := append(ss.keyOptions(user), "sudo", "bash", "-s")
	out, err := ss.s.h.Run.Output(ctx, system.Cmd{Name: "ssh", Args: argv, Stdin: script, Timeout: 5 * time.Minute})
	if err != nil {
		return err
	}
	// invariant: the cleanup powers the VM off under its own session, so ssh answers 0 or 255 (the connection cut by the poweroff); any other code is a failed cleanup.
	if out.ExitCode != 0 && out.ExitCode != 255 {
		return outputError("ssh", out)
	}
	return nil
}

func outputError(tool string, out system.Output) error {
	tail := strings.TrimSpace(out.Stderr)
	if tail == "" {
		tail = strings.TrimSpace(out.Stdout)
	}
	lines := strings.Split(tail, "\n")
	tail = strings.Join(lines[max(0, len(lines)-5):], "\n")
	return fmt.Errorf("%s exited %d: %s", tool, out.ExitCode, tail)
}

func quote(v string) string { return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'" }
