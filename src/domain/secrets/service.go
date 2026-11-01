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

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/settings"
	"github.com/jterrazz/studio-station/src/system"
)

const (
	ClaudeToken = "operator:claude:oauth-token"
	stepBudget  = 2 * time.Minute
)

var (
	sessionName = regexp.MustCompile(`^session(:[a-z0-9-]+)+$`)
)

type Options struct{ Here catalog.MachineKind }

type VMs interface {
	IsVM(ctx context.Context, name string) (bool, error)
	RecordLent(name string, names []string) error
	RecordRecalled(name string, names []string) error
	Holding(names []string) (map[string][]string, error)
}

type Service struct {
	run   system.Runner
	store settings.Store
	vms   VMs
	here  catalog.MachineKind
}

func New(h system.Host, store settings.Store, vms VMs, o Options) *Service {
	return &Service{run: h.Run, store: store, vms: vms, here: o.Here}
}

type Report struct {
	LendList []string
	Lent     map[string][]string
	Kept     []string
}

type LendReport struct {
	To    string
	Names []string
}

type RecallReport struct {
	From     map[string][]string
	Warnings []string
}

func (s *Service) List(_ context.Context) (Report, error) {
	current, err := s.store.Load()
	if err != nil {
		return Report{}, err
	}
	lent, err := s.vms.Holding(nil)
	if err != nil {
		return Report{}, err
	}
	return Report{LendList: append([]string{}, current.LendList...), Lent: lent, Kept: []string{}}, nil
}

func (s *Service) Allow(names []string) error {
	for _, name := range names {
		if name != ClaudeToken && !sessionName.MatchString(name) {
			return &NotSessionError{Name: name}
		}
	}
	current, err := s.store.Load()
	if err != nil {
		return err
	}
	for _, name := range names {
		if !slices.Contains(current.LendList, name) {
			current.LendList = append(current.LendList, name)
		}
	}
	return s.store.Save(current)
}

func (s *Service) LendList() []string {
	current, err := s.store.Load()
	if err != nil {
		return nil
	}
	return current.LendList
}

func (s *Service) Held(names []string) (map[string][]string, error) {
	return s.vms.Holding(names)
}

func (s *Service) CheckDisallow(names []string) error {
	current, err := s.store.Load()
	if err != nil {
		return err
	}
	return onList(current.LendList, names)
}

func (s *Service) Disallow(ctx context.Context, names []string) (RecallReport, error) {
	current, err := s.store.Load()
	if err != nil {
		return RecallReport{}, err
	}
	if listErr := onList(current.LendList, names); listErr != nil {
		return RecallReport{}, listErr
	}
	held, err := s.vms.Holding(names)
	if err != nil {
		return RecallReport{}, err
	}
	current.LendList = slices.DeleteFunc(current.LendList, func(n string) bool { return slices.Contains(names, n) })
	if err := s.store.Save(current); err != nil {
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

func (s *Service) Lend(ctx context.Context, to string, names []string) (LendReport, error) {
	names, err := s.lendPlan(ctx, to, names)
	if err != nil {
		return LendReport{}, err
	}
	probe, err := s.run.Output(ctx, system.Cmd{Name: "ssh", Args: []string{to, "command", "-v", "studio"}, Timeout: stepBudget})
	switch {
	case err != nil:
		return LendReport{}, &StepError{VM: to, Step: "checking for studio", Cause: err}
	case probe.ExitCode == 255:
		return LendReport{}, &StepError{VM: to, Step: "reaching it over ssh", Cause: errors.New(strings.TrimSpace(probe.Stderr))}
	case probe.ExitCode != 0:
		return LendReport{}, &NoStudioError{VM: to}
	}
	exported, err := s.run.Output(ctx, system.Cmd{Name: "studio", Args: append([]string{"credentials", "export"}, names...), Timeout: stepBudget})
	if err != nil || exported.ExitCode != 0 {
		return LendReport{}, &StepError{VM: to, Step: "exporting the secrets", Cause: failure(err, exported)}
	}
	// invariant: the exported values travel from the first process's output to the
	// second's stdin and nowhere else: never argv, never a file on the host.
	imported, err := s.run.Output(ctx, system.Cmd{Name: "ssh", Args: append([]string{to}, vmStudio("import")...), Stdin: exported.Stdout, Timeout: stepBudget})
	if err != nil || imported.ExitCode != 0 {
		return LendReport{}, &StepError{VM: to, Step: "importing the secrets", Cause: failure(err, imported)}
	}
	if err := s.vms.RecordLent(to, names); err != nil {
		return LendReport{}, err
	}
	return LendReport{To: to, Names: names}, nil
}

func (s *Service) checkRecall(ctx context.Context, from string, names []string) ([]string, error) {
	if err := s.checkVM(ctx, from); err != nil {
		return nil, err
	}
	held, err := s.vms.Holding(nil)
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
	current, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		names = current.LendList
	}
	if len(names) == 0 {
		return nil, &EmptyLendListError{}
	}
	if err := onList(current.LendList, names); err != nil {
		return nil, err
	}
	return dedupe(names), nil
}

func (s *Service) checkVM(ctx context.Context, name string) error {
	isVM, err := s.vms.IsVM(ctx, name)
	if err != nil {
		return err
	}
	if isVM {
		return nil
	}
	current, err := s.store.Load()
	if err != nil {
		return err
	}
	if _, host := current.Hosts[name]; host || name == current.Here {
		return &NotAVMError{Name: name}
	}
	return &UnknownVMError{Name: name}
}

func (s *Service) recallFrom(ctx context.Context, vm string, names []string) error {
	args := append([]string{vm}, vmStudio(append([]string{"delete"}, names...)...)...)
	out, err := s.run.Output(ctx, system.Cmd{Name: "ssh", Args: args, Timeout: stepBudget})
	if err != nil || out.ExitCode != 0 {
		return &StepError{VM: vm, Step: "deleting the copies", Cause: failure(err, out)}
	}
	return s.vms.RecordRecalled(vm, names)
}

func vmStudio(args ...string) []string {
	return append([]string{"env", "STUDIO_CREDENTIALS_BACKEND=file", "studio", "credentials"}, args...)
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
