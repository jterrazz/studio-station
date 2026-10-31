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

func (s *Service) Old(_ context.Context) ([]Record, error) {
	var old []Record
	for _, img := range s.list {
		all, err := s.records(img.Name)
		if err != nil {
			return nil, err
		}
		if len(all) > 1 {
			old = append(old, all[1:]...)
		}
	}
	return old, nil
}
