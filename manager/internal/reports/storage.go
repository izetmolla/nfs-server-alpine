package reports

import (
	"os"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"golang.org/x/sys/unix"
)

type PathUsage struct {
	ExportID   uint    `json:"exportId"`
	Path       string  `json:"path"`
	Enabled    bool    `json:"enabled"`
	Exists     bool    `json:"exists"`
	// UsedBytes is the logical size of files under the export directory (not the whole disk).
	UsedBytes uint64 `json:"usedBytes"`
	// Filesystem capacity for the mount that backs this path (informational).
	FsTotalBytes uint64 `json:"fsTotalBytes"`
	FsUsedBytes  uint64 `json:"fsUsedBytes"`
	FsFreeBytes  uint64 `json:"fsFreeBytes"`
	// TotalBytes / FreeBytes / UsedPct kept for older clients:
	// Total/Free = filesystem; UsedPct = folder used / filesystem total.
	TotalBytes uint64  `json:"totalBytes"`
	FreeBytes  uint64  `json:"freeBytes"`
	UsedPct    float64 `json:"usedPct"`
	Error      string  `json:"error,omitempty"`
}

type StorageReport struct {
	Paths []PathUsage `json:"paths"`
	// UsedBytes is the sum of per-export folder sizes.
	UsedBytes uint64 `json:"usedBytes"`
	// TotalBytes / FreeBytes are deduped filesystem capacity across export mounts.
	TotalBytes uint64  `json:"totalBytes"`
	FreeBytes  uint64  `json:"freeBytes"`
	UsedPct    float64 `json:"usedPct"`
}

func Collect(exports []models.Export) StorageReport {
	report := StorageReport{Paths: make([]PathUsage, 0, len(exports))}
	seenFS := map[unix.Fsid]struct{}{}

	for _, exp := range exports {
		item := PathUsage{ExportID: exp.ID, Path: exp.Path, Enabled: exp.Enabled}
		if _, err := os.Stat(exp.Path); err != nil {
			item.Exists = false
			item.Error = err.Error()
			report.Paths = append(report.Paths, item)
			continue
		}
		item.Exists = true

		used, err := DirSize(exp.Path)
		if err != nil {
			item.Error = err.Error()
		}
		item.UsedBytes = used
		report.UsedBytes += used

		var st unix.Statfs_t
		if err := unix.Statfs(exp.Path, &st); err != nil {
			if item.Error == "" {
				item.Error = err.Error()
			}
			report.Paths = append(report.Paths, item)
			continue
		}

		bsize := uint64(st.Bsize)
		item.FsTotalBytes = st.Blocks * bsize
		item.FsFreeBytes = st.Bavail * bsize
		if item.FsTotalBytes >= item.FsFreeBytes {
			item.FsUsedBytes = item.FsTotalBytes - item.FsFreeBytes
		}
		item.TotalBytes = item.FsTotalBytes
		item.FreeBytes = item.FsFreeBytes
		if item.FsTotalBytes > 0 {
			item.UsedPct = float64(item.UsedBytes) * 100 / float64(item.FsTotalBytes)
		}
		report.Paths = append(report.Paths, item)

		if _, ok := seenFS[st.Fsid]; ok {
			continue
		}
		seenFS[st.Fsid] = struct{}{}
		report.TotalBytes += item.FsTotalBytes
		report.FreeBytes += item.FsFreeBytes
	}

	if report.TotalBytes > 0 {
		report.UsedPct = float64(report.UsedBytes) * 100 / float64(report.TotalBytes)
	}
	return report
}
