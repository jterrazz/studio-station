package status

import (
	"github.com/jterrazz/studio-station/src/catalog"
	"github.com/jterrazz/studio-station/src/domain/storage"
)

func disk(report storage.Report) ([]Disk, []storage.StrayReport) {
	disk := []Disk{}
	for _, r := range report.Rows {
		for _, level := range []catalog.Level{catalog.LevelCache, catalog.LevelData} {
			if b := r.Bytes[level]; b > 0 {
				disk = append(disk, Disk{Name: r.Entry.Name, Level: level, Bytes: b})
			}
		}
		if r.Install > 0 {
			disk = append(disk, Disk{Name: r.Entry.Name, Level: levelInstalled, Bytes: r.Install})
		}
	}
	if report.Other > 0 {
		disk = append(disk, Disk{Name: "other", Level: catalog.LevelCache, Bytes: report.Other})
	}
	return disk, append(make([]storage.StrayReport, 0, len(report.Strays)), report.Strays...)
}

const levelInstalled catalog.Level = "installed"
