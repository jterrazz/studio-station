package engine

const NoticeRecallFailed = "lend_recall_failed"

type Notification struct {
	Kind, Subject, Text string // value: what the engine tells the owner beyond the open surfaces, in the engine log's words.
}
