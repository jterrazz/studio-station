package engine

import "context"

const NoticeTakeBackFailed = "take_back_failed"

type Notification struct {
	Kind, Subject, Text string // value: what the engine tells the owner beyond the open surfaces, in the engine log's words.
}

type Notifier interface {
	Notify(ctx context.Context, n Notification)
}

func (d *Engine) notify(ctx context.Context, n Notification) {
	if d.notifier != nil {
		d.notifier.Notify(ctx, n)
	}
}
