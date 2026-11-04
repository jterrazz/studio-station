package machines

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jterrazz/studio-station/src/catalog"
)

const (
	firstbootWait = 5 * time.Minute
	ipWait        = 60 * time.Second
)

func renderFirstboot(script, name string, l Lifetime) string {
	return strings.NewReplacer("{{name}}", name, "{{lifetime}}", string(l)).Replace(script)
}

func (s *Service) firstBoot(ctx context.Context, img catalog.Image, name string, l Lifetime) error {
	script, err := catalog.Recipes.ReadFile(img.Firstboot)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, firstbootWait)
	defer cancel()
	out, err := s.daemon.GuestExec(ctx, name, "sudo bash -s", renderFirstboot(string(script), name, l))
	switch {
	case err != nil:
		return err
	case out.ExitCode != 0:
		return errors.New("the first boot exited " + strconv.Itoa(out.ExitCode) + ": " + firstLine(out.Stderr))
	}
	return nil
}
