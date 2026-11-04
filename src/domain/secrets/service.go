package secrets

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/core/store"
	machinescontract "github.com/jterrazz/studio-station/src/machines/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	ClaudeToken  = catalog.ClaudeTokenName
	stepBudget   = 2 * time.Minute
	ExportBudget = stepBudget
)

type Options struct {
	Here catalog.MachineKind
	// value: whether a VM of this Mac runs now, as its engine says.
	Running func(ctx context.Context, vm string) (bool, error)
	// value: whether a running VM of this Mac keeps accounts, by
	// accounts.SharedInto over the kind the VM itself says it is.
	Receives func(ctx context.Context, vm string) (bool, error)
	// value: the file a failed export's cause is kept in, never shown raw.
	Log string
}

type Guest interface {
	// invariant: a command inside a VM rides its engine's one connection to it.
	GuestExec(ctx context.Context, vm, line, stdin string) (system.Output, error)
}

type Stores struct {
	ShareList contract.ShareListPrefs
	Hosts     machinescontract.MacsPrefs
	Name      setupcontract.NamePrefs
	Shares    contract.ShareStore
	Machines  machinescontract.MachineStore
}

type Service struct {
	h      system.Host
	clock  system.Clock
	guest  Guest
	st     Stores
	source contract.SecretSource
	ids    contract.Identities
	o      Options
}

func New(h system.Host, guest Guest, st Stores, source contract.SecretSource, ids contract.Identities, o Options) *Service {
	return &Service{h: h, clock: h.Clock, guest: guest, st: st, source: source, ids: ids, o: o}
}

func (s *Service) ShareList() []string {
	list, err := s.st.ShareList.Get()
	if err != nil {
		return nil
	}
	return list
}

func (s *Service) Shares(ctx context.Context) (map[string][]accounts.Share, error) {
	return s.st.Shares.List(ctx)
}

func (s *Service) here() string {
	name, err := s.st.Name.Get()
	if err != nil || name == "" {
		return "This Mac"
	}
	return name
}

func (s *Service) CheckShare(ctx context.Context, account, vm string) (secret string, err error) {
	// invariant: a share is refused before anything is read or copied: an
	// unknown account, one the share list does not hold, a VM that may not
	// receive it, an account not signed in here.
	if s.o.Here != catalog.Host {
		return "", &OffHostError{Here: s.o.Here}
	}
	if !accounts.Known(account) {
		return "", &accounts.UnknownError{Account: account}
	}
	secret, ok := accounts.SecretOf(account)
	list, err := s.st.ShareList.Get()
	if err != nil {
		return "", err
	}
	if !ok || !accounts.Shareable(secret, list) {
		return "", &accounts.NotShareableError{Account: account}
	}
	if err := s.CheckReceiver(ctx, vm); err != nil {
		return "", err
	}
	if s.ids != nil {
		for _, p := range catalog.Providers {
			if p.ID != account {
				continue
			}
			if id, err := s.ids.Read(ctx, p); err != nil || id.State != catalog.StateOK {
				return "", &accounts.NotSignedInError{Account: account, Mac: "This Mac"}
			}
		}
	}
	return secret, nil
}

func (s *Service) CheckFrom(ctx context.Context, account, vm, from string) error {
	// invariant: a share is its VM's own Mac's: asked from another Mac, the
	// VM is judged first, so a server's refusal stands, then it is refused.
	if from == "" || from == s.here() {
		return nil
	}
	if err := s.CheckReceiver(ctx, vm); err != nil {
		return err
	}
	return &accounts.OtherMacError{Account: account, VM: vm, Mac: s.here()}
}

func (s *Service) CheckReceiver(ctx context.Context, vm string) error {
	// invariant: a share lands only in a running VM this Mac keeps that
	// keeps accounts: a server keeps its own secrets.
	if err := s.checkVM(ctx, vm); err != nil {
		return err
	}
	if s.o.Running != nil {
		running, err := s.o.Running(ctx, vm)
		if err != nil {
			return err
		}
		if !running {
			return &accounts.VMStoppedError{VM: vm}
		}
	}
	if s.o.Receives != nil {
		receives, err := s.o.Receives(ctx, vm)
		if err != nil {
			return &StepError{VM: vm, Step: "reaching it", Cause: err}
		}
		if !receives {
			return &accounts.ToServerError{VM: vm}
		}
	}
	return nil
}

func (s *Service) Share(ctx context.Context, account, vm string, term time.Duration) (accounts.Shared, error) {
	if term == 0 {
		term = accounts.DefaultShareTerm
	}
	secret, err := s.CheckShare(ctx, account, vm)
	if err != nil {
		return accounts.Shared{}, err
	}
	probe, err := s.inVM(ctx, vm, "command -v studio", "")
	switch {
	case err != nil:
		return accounts.Shared{}, &StepError{VM: vm, Step: "reaching it", Cause: err}
	case probe.ExitCode != 0:
		return accounts.Shared{}, &NoStudioError{VM: vm}
	}
	exported, err := s.source.Export(ctx, []string{secret})
	if err != nil {
		return accounts.Shared{}, &accounts.UnreadableError{Account: account, Mac: "This Mac", Cause: err, Log: s.keep("share "+account+" with "+vm, err)}
	}
	// invariant: the secret travels from the export's output to the VM's
	// stdin and nowhere else: never argv, never a file on the Mac.
	imported, err := s.inVM(ctx, vm, VMStudio("import"), string(exported))
	if err != nil || imported.ExitCode != 0 {
		return accounts.Shared{}, &StepError{VM: vm, Step: "copying " + accounts.Label(account), Cause: failure(err, imported)}
	}
	at := s.clock.Now().UTC().Truncate(time.Second)
	share := accounts.Share{Name: secret, At: at, Until: at.Add(term)}
	held, err := s.st.Shares.Get(ctx, vm)
	if err != nil {
		return accounts.Shared{}, err
	}
	held = slices.DeleteFunc(held, func(have accounts.Share) bool { return have.Name == secret })
	if err := s.st.Shares.Put(ctx, vm, append(held, share)); err != nil {
		return accounts.Shared{}, err
	}
	return accounts.Shared{Account: account, VM: vm, At: share.At, Until: share.Until}, nil
}

func (s *Service) keep(what string, cause error) string {
	// invariant: a source's own words go to the log and only there; the
	// refusal names the file, shown from home, or nothing when none is kept.
	if s.o.Log == "" {
		return ""
	}
	before, err := s.h.FS.ReadFile(s.o.Log)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return ""
	}
	if err := s.h.FS.MkdirAll(path.Dir(s.o.Log), 0o700); err != nil {
		return ""
	}
	line := s.clock.Now().UTC().Format(time.RFC3339) + " " + what + ": " + cause.Error() + "\n"
	if err := s.h.FS.WriteFile(s.o.Log, append(before, line...), 0o600); err != nil {
		return ""
	}
	if home := s.h.Env.Home(); home != "" && strings.HasPrefix(s.o.Log, home+"/") {
		return "~" + strings.TrimPrefix(s.o.Log, home)
	}
	return s.o.Log
}

func (s *Service) CheckTakeBack(ctx context.Context, account, vm string) (string, error) {
	// invariant: a take-back names an account this Mac shares with that VM;
	// anything else is refused before the VM is asked.
	if !accounts.Known(account) {
		return "", &accounts.UnknownError{Account: account}
	}
	if err := s.checkVM(ctx, vm); err != nil {
		return "", err
	}
	held, err := s.st.Shares.Get(ctx, vm)
	if err != nil {
		return "", err
	}
	secret, _ := accounts.SecretOf(account)
	if !slices.ContainsFunc(held, func(sh accounts.Share) bool { return sh.Name == secret }) {
		return "", &accounts.NotSharedError{Account: account, VM: vm}
	}
	return secret, nil
}

func (s *Service) TakeBack(ctx context.Context, account, vm string) error {
	secret, err := s.CheckTakeBack(ctx, account, vm)
	if err != nil {
		return err
	}
	return s.Expire(ctx, vm, []string{secret})
}

func (s *Service) Expire(ctx context.Context, vm string, secrets []string) error {
	// value: deletes the VM's copies of these secrets, then their shares.
	out, err := s.inVM(ctx, vm, VMStudio(append([]string{"delete"}, secrets...)...), "")
	if err != nil || out.ExitCode != 0 {
		return &StepError{VM: vm, Step: "deleting the copies", Cause: failure(err, out)}
	}
	held, err := s.st.Shares.Get(ctx, vm)
	if err != nil {
		return err
	}
	return s.st.Shares.Put(ctx, vm, slices.DeleteFunc(held, func(sh accounts.Share) bool { return slices.Contains(secrets, sh.Name) }))
}

func (s *Service) Forget(ctx context.Context, vm string) error {
	// value: a deleted VM takes its copies with its disk; its shares go too.
	return s.st.Shares.Delete(ctx, vm)
}

func (s *Service) checkVM(ctx context.Context, name string) error {
	var unreadable *store.UnreadableError
	if _, err := s.st.Machines.Get(ctx, name); err == nil || errors.As(err, &unreadable) {
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	hosts, err := s.st.Hosts.Get()
	if err != nil {
		return err
	}
	if _, host := hosts[name]; host || name == s.here() {
		return &NotAVMError{Name: name}
	}
	return &accounts.UnknownVMError{VM: name}
}

func (s *Service) inVM(ctx context.Context, vm, line, stdin string) (system.Output, error) {
	ctx, cancel := context.WithTimeout(ctx, stepBudget)
	defer cancel()
	return s.guest.GuestExec(ctx, vm, line, stdin)
}

func VMStudio(args ...string) string {
	// value: studio's command line inside a VM, its credential store a file
	// there: import reads the secrets on stdin, delete takes their names.
	return strings.Join(append([]string{"STUDIO_CREDENTIALS_BACKEND=file", "studio", "credentials"}, args...), " ")
}

func failure(err error, out system.Output) error {
	if err != nil {
		return err
	}
	if text := strings.TrimSpace(out.Stderr); text != "" {
		return errors.New(text)
	}
	return fmt.Errorf("exit %d", out.ExitCode)
}
