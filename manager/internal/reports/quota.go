package reports

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"golang.org/x/sys/unix"
)

type QuotaUsage struct {
	ExportID         uint       `json:"exportId"`
	Path             string     `json:"path"`
	Enabled          bool       `json:"enabled"`
	Exists           bool       `json:"exists"`
	Unlimited        bool       `json:"unlimited"`
	QuotaBytes       int64      `json:"quotaBytes"`
	UsedBytes        uint64     `json:"usedBytes"`
	RemainingBytes   *uint64    `json:"remainingBytes,omitempty"`
	UsedPct          float64    `json:"usedPct"`
	Full             bool       `json:"full"`
	QuotaPostbackURL string     `json:"quotaPostbackUrl,omitempty"`
	QuotaNotifiedAt  *time.Time `json:"quotaNotifiedAt,omitempty"`
	FSTotalBytes     uint64     `json:"fsTotalBytes"`
	FSUsedBytes      uint64     `json:"fsUsedBytes"`
	FSFreeBytes      uint64     `json:"fsFreeBytes"`
	Error            string     `json:"error,omitempty"`
}

type QuotaReport struct {
	Exports []QuotaUsage `json:"exports"`
	CheckedAt time.Time  `json:"checkedAt"`
}

// DirSize walks path and sums regular file sizes (logical bytes).
func DirSize(root string) (uint64, error) {
	info, err := os.Stat(root)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		if info.Mode().IsRegular() {
			return uint64(info.Size()), nil
		}
		return 0, nil
	}
	var total uint64
	err = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// Skip unreadable entries; keep counting what we can.
			if os.IsPermission(walkErr) {
				return nil
			}
			return walkErr
		}
		if fi == nil || !fi.Mode().IsRegular() {
			return nil
		}
		if fi.Size() > 0 {
			total += uint64(fi.Size())
		}
		return nil
	})
	return total, err
}

func EvaluateExport(exp models.Export) QuotaUsage {
	item := QuotaUsage{
		ExportID:         exp.ID,
		Path:             exp.Path,
		Enabled:          exp.Enabled,
		QuotaBytes:       exp.QuotaBytes,
		Unlimited:        exp.QuotaBytes <= 0,
		QuotaPostbackURL: exp.QuotaPostbackURL,
		QuotaNotifiedAt:  exp.QuotaNotifiedAt,
	}

	if _, err := os.Stat(exp.Path); err != nil {
		item.Exists = false
		item.Error = err.Error()
		return item
	}
	item.Exists = true

	used, err := DirSize(exp.Path)
	if err != nil {
		item.Error = err.Error()
	}
	item.UsedBytes = used

	var st unix.Statfs_t
	if err := unix.Statfs(exp.Path, &st); err == nil {
		bsize := uint64(st.Bsize)
		item.FSTotalBytes = st.Blocks * bsize
		item.FSFreeBytes = st.Bavail * bsize
		if item.FSTotalBytes >= item.FSFreeBytes {
			item.FSUsedBytes = item.FSTotalBytes - item.FSFreeBytes
		}
	}

	if item.Unlimited {
		return item
	}
	q := uint64(exp.QuotaBytes)
	if used >= q {
		item.Full = true
		item.UsedPct = 100
		zero := uint64(0)
		item.RemainingBytes = &zero
	} else {
		rem := q - used
		item.RemainingBytes = &rem
		item.UsedPct = float64(used) * 100 / float64(q)
	}
	return item
}

func CollectQuotas(exports []models.Export) QuotaReport {
	out := QuotaReport{
		Exports:   make([]QuotaUsage, 0, len(exports)),
		CheckedAt: time.Now().UTC(),
	}
	for _, exp := range exports {
		out.Exports = append(out.Exports, EvaluateExport(exp))
	}
	return out
}

func FormatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
