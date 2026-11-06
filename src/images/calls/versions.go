package calls

import (
	"context"

	"github.com/jterrazz/studio-station/src/images"
)

func (s *Service) Latest(_ context.Context, image string) (images.Record, error) {
	all, err := s.records(image)
	if err != nil {
		return images.Record{}, err
	}
	if len(all) == 0 {
		return images.Record{}, &images.NotBuiltError{Image: image}
	}
	return all[0], nil
}
