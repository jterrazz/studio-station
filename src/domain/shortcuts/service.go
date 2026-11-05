package shortcuts

import "github.com/jterrazz/studio-station/src/system"

type Service struct{ h system.Host }

func New(h system.Host) *Service { return &Service{h: h} }
