package macos

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/platform/local"
	"github.com/jterrazz/studio-station/src/settings"
	"github.com/jterrazz/studio-station/src/settings/contract"
)

const (
	updatesPlist = "/Library/Preferences/com.apple.SoftwareUpdate.plist"
	staleUpdates = 7 * 24 * time.Hour
	plistBudget  = 2 * time.Second
)

type Runner interface {
	Output(ctx context.Context, c local.Cmd) (local.Output, error)
}

type Files interface {
	Stat(path string) (fs.FileInfo, error)
}

type Clock interface{ Now() time.Time }

type Options struct {
	// value: the system root the machine-wide preferences are read under.
	Root string
}

type Inspector struct {
	run   Runner
	files Files
	clock Clock
	o     Options
}

var _ contract.Inspector = (*Inspector)(nil)

func New(run Runner, files Files, clock Clock, o Options) *Inspector {
	return &Inspector{run: run, files: files, clock: clock, o: o}
}

type protection struct {
	cmd local.Cmd
	// value: what the tool prints, on stdout or stderr, when the protection is on.
	on string
}

func protections() map[catalog.Fact]protection {
	return map[catalog.Fact]protection{
		catalog.FactFileVault:  {local.Command("fdesetup", "status"), "FileVault is On"},
		catalog.FactFirewall:   {local.Command("socketfilterfw", "--getglobalstate"), "enabled"},
		catalog.FactSIP:        {local.Command("csrutil", "status"), "enabled"},
		catalog.FactGatekeeper: {local.Command("spctl", "--status"), "enabled"},
	}
}

var errUnknownFact = errors.New("no such fact on a Mac")

func (i *Inspector) Read(ctx context.Context, f catalog.Fact) (settings.Fact, error) {
	if f == catalog.FactSystemUpdates {
		return i.updates(ctx)
	}
	p, ok := protections()[f]
	if !ok {
		return settings.Unread(), errUnknownFact
	}
	r, err := i.ask(ctx, p.cmd)
	if err != nil {
		return settings.Fact{}, err
	}
	switch {
	case !r.ok():
		return settings.Unread(), nil
	case strings.Contains(r.out.Stdout+r.out.Stderr, p.on):
		return settings.Fact{State: catalog.StateOK, Value: "on"}, nil
	}
	return settings.Fact{State: catalog.StateAbsent, Value: "off"}, nil
}

type reply struct {
	out   local.Output
	found bool
}

func (r reply) ok() bool { return r.found && r.out.ExitCode == 0 }

// invariant: a tool that is missing or fails is an answer (not ok); only a
// read the deadline ended first is an error.
func (i *Inspector) ask(ctx context.Context, c local.Cmd) (reply, error) {
	out, err := i.run.Output(ctx, c)
	if errors.Is(err, local.ErrTimeout) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return reply{}, local.ErrTimeout
	}
	if err != nil {
		out = local.Output{ExitCode: -1}
	}
	return reply{found: !errors.Is(err, local.ErrNotFound), out: out}, nil
}

// invariant: the updates come from the list the background check wrote to
// its plist (milliseconds); the network call runs only when that list is
// missing or older than a week.
func (i *Inspector) updates(ctx context.Context) (settings.Fact, error) {
	plist := path.Join("/", i.o.Root, updatesPlist)
	if _, err := i.files.Stat(plist); err == nil {
		at, atErr := i.ask(ctx, local.Cmd{Name: "plutil", Args: []string{"-extract", "LastSuccessfulDate", "raw", "-o", "-", plist}, Timeout: plistBudget})
		list, listErr := i.ask(ctx, local.Cmd{Name: "plutil", Args: []string{"-extract", "RecommendedUpdates", "raw", "-o", "-", plist}, Timeout: plistBudget})
		checked, err := time.Parse(time.RFC3339, strings.TrimSpace(at.out.Stdout))
		pending, countErr := strconv.Atoi(strings.TrimSpace(list.out.Stdout))
		age := i.clock.Now().Sub(checked)
		if atErr == nil && listErr == nil && at.ok() && list.ok() && err == nil && countErr == nil && age < staleUpdates {
			detail := "checked " + ago(age) + " ago"
			if pending == 0 {
				return settings.Fact{State: catalog.StateOK, Value: "up to date", Detail: detail}, nil
			}
			return settings.Fact{State: catalog.StateDegraded, Value: strconv.Itoa(pending) + " pending", Detail: detail}, nil
		}
	}
	r, err := i.ask(ctx, local.Command("softwareupdate", "-l"))
	if err != nil {
		return settings.Fact{}, err
	}
	return softwareUpdate(r), nil
}

// outside: softwareupdate prints its verdict on stderr, and lists each update as `* Label: ...`.
func softwareUpdate(r reply) settings.Fact {
	if !r.ok() {
		return settings.Unread()
	}
	pending := 0
	for _, line := range strings.Split(r.out.Stdout+r.out.Stderr, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "* ") {
			pending++
		}
	}
	if pending == 0 {
		return settings.Fact{State: catalog.StateOK, Value: "up to date"}
	}
	return settings.Fact{State: catalog.StateDegraded, Value: strconv.Itoa(pending) + " pending", Detail: "softwareupdate -l"}
}

func ago(d time.Duration) string {
	switch {
	case d < time.Hour:
		return strconv.Itoa(max(int(d.Minutes()), 0)) + "m"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h"
	}
	return strconv.Itoa(int(d.Hours()/24)) + "d"
}
