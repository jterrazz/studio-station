package setup

import (
	"context"
	"errors"
	"github.com/jterrazz/studio-station/src/setup"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"time"

	accountscontract "github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/core/store"
	machinescontract "github.com/jterrazz/studio-station/src/machines/contract"
	portscontract "github.com/jterrazz/studio-station/src/ports/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/domain/shell"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	stepName   = "name"
	stepZshrc  = "zshrc"
	stepLayout = "layout"
)

var machineName = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

type Registrar interface {
	Here() (string, bool)
	Register(name string) error
}

type DaemonInstaller interface {
	Install(ctx context.Context) (Step, error)
}

type Options struct {
	Here catalog.MachineKind
	// value: the preferences' file (a v1 config.json converts only while it
	// is absent), station's state folder and the user's bin.
	PreferencesFile, State, Bin string
	Executable                  string
	Detached                    bool
	Tools                       []catalog.Tool
	Expand                      storage.Expander
	Files                       map[catalog.ToolFile]string
	Configurer                  Configurer
	Installer                   Installer
	AppStoreCLI                 func() string
}

type Prefs struct {
	Name      setupcontract.NamePrefs
	Identity  setupcontract.IdentityPrefs
	Hosts     machinescontract.MacsPrefs
	VMUser    machinescontract.VMUserPrefs
	ShareList accountscontract.ShareListPrefs
	Services  portscontract.PortsPrefs
	// value: every store station keeps, each brought to its version by the
	// layout step.
	Stores []store.Migrator
}

type Service struct {
	h      system.Host
	p      Prefs
	vault  Vault
	reg    Registrar
	daemon DaemonInstaller
	o      Options
}

type Vault interface {
	Forget(ctx context.Context, name string) error
}

func New(h system.Host, p Prefs, v Vault, reg Registrar, d DaemonInstaller, o Options) *Service {
	return &Service{h: h, p: p, vault: v, reg: reg, daemon: d, o: o}
}

type Request struct {
	Name        string
	Interactive bool
	AskName     func() (string, error)
}

type (
	Step   = setup.Step
	Report = setup.Report
)

func ValidName(name string) bool { return machineName.MatchString(name) }

func (s *Service) isHost() bool { return s.o.Here == catalog.Host }

func (s *Service) CheckName(name string) error {
	if name == "" {
		return nil
	}
	if !ValidName(name) {
		return &BadNameError{Name: name}
	}
	if !s.isHost() {
		return ErrNameOnVM
	}
	if here := s.currentHere(); here != "" && here != name {
		return &NameMismatchError{Here: here, Asked: name}
	}
	return nil
}

// invariant: a host whose settings.json is not written yet but whose v1
// config.json names it is already registered: the layout step will say so.
func (s *Service) currentHere() string {
	if here, ok := s.reg.Here(); ok {
		return here
	}
	if _, err := s.h.FS.Stat(s.settingsPath()); err == nil {
		return ""
	}
	data, err := s.h.FS.ReadFile(s.v1Path())
	if err != nil {
		return ""
	}
	converted, err := settings.ConvertV1(data, s.h.Env.User())
	if err != nil {
		return ""
	}
	return converted.Here
}

func (s *Service) DefaultName(ctx context.Context) string {
	raw := s.h.Env.Hostname()
	if out, err := s.h.Run.Output(ctx, system.Cmd{Name: "scutil", Args: []string{"--get", "LocalHostName"}, Timeout: 2 * time.Second}); err == nil && out.ExitCode == 0 && strings.TrimSpace(out.Stdout) != "" {
		raw = strings.TrimSpace(out.Stdout)
	}
	return sanitize(raw)
}

func sanitize(raw string) string {
	label, _, _ := strings.Cut(strings.ToLower(raw), ".")
	label = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, label)
	label = strings.Trim(label, "-")
	if len(label) > 40 {
		label = strings.TrimRight(label[:40], "-")
	}
	return label
}

func (s *Service) Run(ctx context.Context, r Request) (Report, error) {
	if err := s.CheckName(r.Name); err != nil {
		return Report{}, err
	}
	if !s.isHost() {
		zshrc, err := s.zshrc()
		relocate, relocateErr := s.relocate(ctx, r.Interactive)
		return Report{Steps: []Step{zshrc, relocate}}, errors.Join(err, relocateErr)
	}
	migrated := s.migrateSettings()
	name, nameErr := s.name(r, migrated != nil)
	if nameErr != nil && !isStepFailure(nameErr) {
		return Report{}, nameErr
	}
	zshrc, zshrcErr := s.zshrc()
	layout, layoutErr := s.layout(ctx, migrated)
	relocate, relocateErr := s.relocate(ctx, r.Interactive)
	steps := append(append([]Step{name, zshrc}, layout...), relocate)
	toolsErr, daemonErr := error(nil), error(nil)
	if relocateErr == nil {
		var tools Step
		tools, toolsErr = s.tools(ctx, r.Interactive)
		steps = append(steps, tools)
	}
	if s.daemon != nil {
		var daemon Step
		daemon, daemonErr = s.daemon.Install(ctx)
		steps = append(steps, daemon)
	}
	return Report{Steps: steps}, errors.Join(nameErr, zshrcErr, layoutErr, relocateErr, toolsErr, daemonErr)
}

func isStepFailure(err error) bool {
	var failed *StepFailedError
	return errors.As(err, &failed)
}

func (s *Service) name(r Request, blocked bool) (Step, error) {
	if here, ok := s.reg.Here(); ok {
		return Step{Name: stepName, State: catalog.StateOK, Detail: here}, nil
	}
	if blocked {
		return Step{Name: stepName, State: catalog.StateAbsent, Detail: "waits for the layout step"}, nil
	}
	name := r.Name
	if name == "" && r.Interactive && r.AskName != nil {
		asked, err := r.AskName()
		if err != nil {
			return Step{}, err
		}
		name = asked
	}
	if name == "" {
		return Step{Name: stepName, State: catalog.StateAbsent, Detail: "not set: station setup --name <name>"}, nil
	}
	if !ValidName(name) {
		return Step{}, &BadNameError{Name: name}
	}
	if err := s.reg.Register(name); err != nil {
		return failed(stepName, err)
	}
	return Step{Name: stepName, State: catalog.StateOK, Detail: name, Changed: true}, nil
}

func failed(step string, cause error) (Step, error) {
	return Step{Name: step, State: catalog.StateBroken, Detail: cause.Error()}, &StepFailedError{Step: step, Cause: cause}
}

func (s *Service) zshrcPath() string { return path.Join(s.h.Env.Home(), ".zshrc") }

func (s *Service) zshrc() (Step, error) {
	current, err := s.h.FS.ReadFile(s.zshrcPath())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return failed(stepZshrc, err)
	}
	updated, changed := shell.UpsertBlock(string(current))
	if changed {
		perm := fs.FileMode(0o644)
		if info, statErr := s.h.FS.Stat(s.zshrcPath()); statErr == nil {
			perm = info.Mode().Perm()
		}
		if err := s.h.FS.WriteFile(s.zshrcPath(), []byte(updated), perm); err != nil {
			return failed(stepZshrc, err)
		}
	}
	return Step{Name: stepZshrc, State: catalog.StateOK, Changed: changed}, nil
}

func (s *Service) Check(_ context.Context) (Report, error) {
	var steps []Step
	if s.isHost() {
		if here, ok := s.reg.Here(); ok {
			steps = append(steps, Step{Name: stepName, State: catalog.StateOK, Detail: here})
		} else {
			steps = append(steps, Step{Name: stepName, State: catalog.StateAbsent, Detail: "station setup --name <name>"})
		}
	}
	zshrc := Step{Name: stepZshrc, State: catalog.StateOK}
	if data, err := s.h.FS.ReadFile(s.zshrcPath()); err != nil || !shell.HasBlock(string(data)) {
		zshrc = Step{Name: stepZshrc, State: catalog.StateAbsent, Detail: "station setup"}
	}
	steps = append(steps, zshrc)
	if s.isHost() {
		layout := Step{Name: stepLayout, State: catalog.StateOK}
		if s.holdsStationFiles() {
			layout = Step{Name: stepLayout, State: catalog.StateDegraded, Detail: "~/.studio still holds station's files: station setup"}
		}
		steps = append(steps, layout, s.toolsCheck())
	}
	return Report{Steps: steps}, nil
}
