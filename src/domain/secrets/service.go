package secrets

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/accounts"
	"github.com/jterrazz/studio-station/src/core/store"
	machinescontract "github.com/jterrazz/studio-station/src/machines/contract"
	setupcontract "github.com/jterrazz/studio-station/src/setup/contract"

	"github.com/jterrazz/studio-station/src/accounts/contract"
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	ClaudeToken  = "operator:claude:oauth-token"
	stepBudget   = 2 * time.Minute
	ExportBudget = stepBudget
	DefaultLend  = accounts.DefaultShareTerm
	ShortestLend = time.Minute
	LongestLend  = 7 * 24 * time.Hour
)

var (
	sessionName = regexp.MustCompile(`^session(:[a-z0-9-]+)+$`)
)

type Options struct{ Here catalog.MachineKind }

type Guest interface {
	// invariant: a command inside a VM rides its engine's one connection to it.
	GuestExec(ctx context.Context, vm, line, stdin string) (system.Output, error)
}

type Lend = accounts.Share

type Stores struct {
	ShareList contract.ShareListPrefs
	Hosts     machinescontract.MacsPrefs
	Name      setupcontract.NamePrefs
	Shares    contract.ShareStore
	Machines  machinescontract.MachineStore
}

type Service struct {
	clock  system.Clock
	run    system.Runner
	guest  Guest
	st     Stores
	source contract.SecretSource
	here   catalog.MachineKind
}

func New(h system.Host, guest Guest, st Stores, source contract.SecretSource, o Options) *Service {
	return &Service{clock: h.Clock, run: h.Run, guest: guest, st: st, source: source, here: o.Here}
}

type Report struct {
	LendList []string
	Lent     map[string][]string
	Lends    map[string][]Lend
	Kept     []string
}

type LendReport struct {
	To    string
	Names []string
	At    time.Time
	Until time.Time
}

type RecallReport struct {
	From     map[string][]string
	Warnings []string
}

func (s *Service) List(ctx context.Context) (Report, error) {
	list, err := s.st.ShareList.Get()
	if err != nil {
		return Report{}, err
	}
	lends, err := s.st.Shares.List(ctx)
	if err != nil {
		return Report{}, err
	}
	return Report{LendList: append([]string{}, list...), Lent: names(lends, nil), Lends: lends, Kept: []string{}}, nil
}

func (s *Service) Allow(names []string) error {
	for _, name := range names {
		if name != ClaudeToken && !sessionName.MatchString(name) {
			return &NotSessionError{Name: name}
		}
	}
	list, err := s.st.ShareList.Get()
	if err != nil {
		return err
	}
	for _, name := range names {
		if !slices.Contains(list, name) {
			list = append(list, name)
		}
	}
	return s.st.ShareList.Set(list)
}

func (s *Service) LendList() []string {
	list, err := s.st.ShareList.Get()
	if err != nil {
		return nil
	}
	return list
}

func (s *Service) Held(names []string) (map[string][]string, error) {
	return s.holding(names)
}

func (s *Service) Lends() (map[string][]Lend, error) { return s.st.Shares.List(context.Background()) }

func (s *Service) holding(only []string) (map[string][]string, error) {
	lends, err := s.st.Shares.List(context.Background())
	if err != nil {
		return nil, err
	}
	return names(lends, only), nil
}

func names(lends map[string][]Lend, only []string) map[string][]string {
	held := map[string][]string{}
	for vm, list := range lends {
		for _, l := range list {
			if only == nil || slices.Contains(only, l.Name) {
				held[vm] = append(held[vm], l.Name)
			}
		}
	}
	return held
}

func (s *Service) CheckDisallow(names []string) error {
	list, err := s.st.ShareList.Get()
	if err != nil {
		return err
	}
	return onList(list, names)
}

func (s *Service) Disallow(ctx context.Context, names []string) (RecallReport, error) {
	list, err := s.st.ShareList.Get()
	if err != nil {
		return RecallReport{}, err
	}
	if listErr := onList(list, names); listErr != nil {
		return RecallReport{}, listErr
	}
	held, err := s.holding(names)
	if err != nil {
		return RecallReport{}, err
	}
	if err := s.st.ShareList.Set(slices.DeleteFunc(list, func(n string) bool { return slices.Contains(names, n) })); err != nil {
		return RecallReport{}, err
	}
	report := RecallReport{From: map[string][]string{}}
	for _, vm := range sortedKeys(held) {
		if err := s.recallFrom(ctx, vm, held[vm]); err != nil {
			report.Warnings = append(report.Warnings, err.Error())
			continue
		}
		report.From[vm] = held[vm]
	}
	return report, nil
}

func (s *Service) Lend(ctx context.Context, to string, names []string, term time.Duration) (LendReport, error) {
	if term == 0 {
		term = DefaultLend
	}
	names, err := s.lendPlan(ctx, to, names)
	if err != nil {
		return LendReport{}, err
	}
	probe, err := s.inVM(ctx, to, "command -v studio", "")
	switch {
	case err != nil:
		return LendReport{}, &StepError{VM: to, Step: "reaching it over ssh", Cause: err}
	case probe.ExitCode != 0:
		return LendReport{}, &NoStudioError{VM: to}
	}
	exported, err := s.source.Export(ctx, names)
	if err != nil {
		return LendReport{}, &StepError{VM: to, Step: "exporting the secrets", Cause: err}
	}
	// invariant: the exported values travel from the first process's output to the
	// second's stdin and nowhere else: never argv, never a file on the host.
	imported, err := s.inVM(ctx, to, vmStudio("import"), string(exported))
	if err != nil || imported.ExitCode != 0 {
		return LendReport{}, &StepError{VM: to, Step: "importing the secrets", Cause: failure(err, imported)}
	}
	at := s.clock.Now().UTC().Truncate(time.Second)
	lends := make([]Lend, 0, len(names))
	for _, n := range names {
		lends = append(lends, Lend{Name: n, At: at, Until: at.Add(term)})
	}
	if err := s.recordLent(ctx, to, lends); err != nil {
		return LendReport{}, err
	}
	return LendReport{To: to, Names: names, At: at, Until: at.Add(term)}, nil
}

func (s *Service) checkRecall(ctx context.Context, from string, names []string) ([]string, error) {
	if err := s.checkVM(ctx, from); err != nil {
		return nil, err
	}
	held, err := s.holding(nil)
	if err != nil {
		return nil, err
	}
	have := held[from]
	if len(names) == 0 {
		return have, nil
	}
	for _, name := range names {
		if !slices.Contains(have, name) {
			return nil, &NotLentError{Name: name, VM: from}
		}
	}
	return names, nil
}

func (s *Service) Recall(ctx context.Context, from string, names []string) (RecallReport, error) {
	names, err := s.checkRecall(ctx, from, names)
	if err != nil {
		return RecallReport{}, err
	}
	report := RecallReport{From: map[string][]string{}}
	if len(names) == 0 {
		return report, nil
	}
	if err := s.recallFrom(ctx, from, names); err != nil {
		return RecallReport{}, err
	}
	report.From[from] = names
	return report, nil
}

func (s *Service) lendPlan(ctx context.Context, to string, names []string) ([]string, error) {
	if s.here != catalog.Host {
		return nil, &OffHostError{Here: s.here}
	}
	if err := s.checkVM(ctx, to); err != nil {
		return nil, err
	}
	list, err := s.st.ShareList.Get()
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		names = list
	}
	if len(names) == 0 {
		return nil, &EmptyLendListError{}
	}
	if err := onList(list, names); err != nil {
		return nil, err
	}
	return dedupe(names), nil
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
	here, err := s.st.Name.Get()
	if err != nil {
		return err
	}
	if _, host := hosts[name]; host || name == here {
		return &NotAVMError{Name: name}
	}
	return &UnknownVMError{Name: name}
}

func (s *Service) recallFrom(ctx context.Context, vm string, names []string) error {
	out, err := s.inVM(ctx, vm, vmStudio(append([]string{"delete"}, names...)...), "")
	if err != nil || out.ExitCode != 0 {
		return &StepError{VM: vm, Step: "deleting the copies", Cause: failure(err, out)}
	}
	held, err := s.st.Shares.Get(ctx, vm)
	if err != nil {
		return err
	}
	return s.st.Shares.Put(ctx, vm, slices.DeleteFunc(held, func(l Lend) bool { return slices.Contains(names, l.Name) }))
}

func (s *Service) recordLent(ctx context.Context, vm string, lends []Lend) error {
	held, err := s.st.Shares.Get(ctx, vm)
	if err != nil {
		return err
	}
	for _, l := range lends {
		if i := slices.IndexFunc(held, func(have Lend) bool { return have.Name == l.Name }); i >= 0 {
			held[i] = l
		} else {
			held = append(held, l)
		}
	}
	return s.st.Shares.Put(ctx, vm, held)
}

func (s *Service) inVM(ctx context.Context, vm, line, stdin string) (system.Output, error) {
	ctx, cancel := context.WithTimeout(ctx, stepBudget)
	defer cancel()
	return s.guest.GuestExec(ctx, vm, line, stdin)
}

func vmStudio(args ...string) string {
	return strings.Join(append([]string{"STUDIO_CREDENTIALS_BACKEND=file", "studio", "credentials"}, args...), " ")
}

func onList(list, names []string) error {
	for _, name := range names {
		if !slices.Contains(list, name) {
			return &NotOnLendListError{Name: name}
		}
	}
	return nil
}

func dedupe(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out
}

func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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
