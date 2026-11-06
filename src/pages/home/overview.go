package home

import (
	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/domain/storage"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/settings"
)

type (
	Overview  = overview.Snapshot[apps.Report, settings.Report, storage.StrayReport]
	Workspace = overview.Workspace[storage.StrayReport]
	Reading   = activity.Reading
)
