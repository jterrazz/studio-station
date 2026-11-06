package home

import (
	"github.com/jterrazz/studio-station/src/activity"
	"github.com/jterrazz/studio-station/src/apps"
	"github.com/jterrazz/studio-station/src/disk"
	"github.com/jterrazz/studio-station/src/overview"
	"github.com/jterrazz/studio-station/src/settings"
)

type (
	Overview  = overview.Snapshot[apps.Report, settings.Report, disk.MisplacedReport]
	Workspace = overview.Workspace[disk.MisplacedReport]
	Reading   = activity.Reading
)
