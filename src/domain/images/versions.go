package images

import "context"

func (s *Service) Latest(_ context.Context, image string) (Record, error) {
	all, err := s.records(image)
	if err != nil {
		return Record{}, err
	}
	if len(all) == 0 {
		return Record{}, &NotBuiltError{Image: image}
	}
	return all[0], nil
}
