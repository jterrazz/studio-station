package token

import "time"

const (
	Margin      = 1
	Indent      = 2
	Gap         = 2
	TreeStep    = 3
	MarkWidth   = 2
	LabelMin    = 8
	LabelMax    = 28
	NarrowWidth = 80
	SplitWidth  = 110
	PaneWidth   = 44
	MinWidth    = 40
	ModalWidth  = 60

	SpinnerFrame = 80 * time.Millisecond
	LiveFrame    = 50 * time.Millisecond
	WaitDelay    = 300 * time.Millisecond
)
