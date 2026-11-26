package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/applog"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/reports"
	"gorm.io/gorm"
)

type Watcher struct {
	DB       *gorm.DB
	Log      *applog.Ring
	Interval time.Duration
	Client   *http.Client

	mu sync.Mutex
}

type FullEvent struct {
	Event            string    `json:"event"`
	ExportID         uint      `json:"exportId"`
	Path             string    `json:"path"`
	UsedBytes        uint64    `json:"usedBytes"`
	QuotaBytes       int64     `json:"quotaBytes"`
	UsedPct          float64   `json:"usedPct"`
	RemainingBytes   uint64    `json:"remainingBytes"`
	QuotaPostbackURL string    `json:"quotaPostbackUrl"`
	At               time.Time `json:"at"`
	Message          string    `json:"message"`
}

func NewWatcher(db *gorm.DB, log *applog.Ring, interval time.Duration) *Watcher {
	if interval < 15*time.Second {
		interval = 60 * time.Second
	}
	return &Watcher{
		DB:       db,
		Log:      log,
		Interval: interval,
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *Watcher) Start() {
	go func() {
		// Initial delay so NFS/manager finish booting.
		time.Sleep(5 * time.Second)
		for {
			_, _ = w.Check(true)
			time.Sleep(w.Interval)
		}
	}()
}

// Check evaluates all exports. If notify is true, fires postbacks for newly-full quotas.
func (w *Watcher) Check(notify bool) (reports.QuotaReport, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var exports []models.Export
	if err := w.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return reports.QuotaReport{}, err
	}
	report := reports.CollectQuotas(exports)
	if !notify {
		return report, nil
	}

	byID := map[uint]models.Export{}
	for _, e := range exports {
		byID[e.ID] = e
	}

	for _, item := range report.Exports {
		exp := byID[item.ExportID]
		if item.Unlimited || !item.Exists {
			if exp.QuotaNotifiedAt != nil {
				_ = w.DB.Model(&exp).Update("quota_notified_at", nil).Error
			}
			continue
		}
		if !item.Full {
			if exp.QuotaNotifiedAt != nil {
				_ = w.DB.Model(&exp).Update("quota_notified_at", nil).Error
				if w.Log != nil {
					w.Log.Info("quota", "quota recovered", exp.Path)
				}
			}
			continue
		}
		// Already notified for this full state.
		if exp.QuotaNotifiedAt != nil {
			continue
		}
		url := strings.TrimSpace(exp.QuotaPostbackURL)
		if url == "" {
			if w.Log != nil {
				w.Log.Warn("quota", "quota full (no postback configured)", exp.Path)
			}
			now := time.Now().UTC()
			_ = w.DB.Model(&exp).Update("quota_notified_at", now).Error
			continue
		}
		if err := w.post(exp, item); err != nil {
			if w.Log != nil {
				w.Log.Error("quota", "postback failed", exp.Path+": "+err.Error())
			}
			continue
		}
		now := time.Now().UTC()
		_ = w.DB.Model(&exp).Update("quota_notified_at", now).Error
		if w.Log != nil {
			w.Log.Warn("quota", "quota full — postback sent", exp.Path+" → "+url)
		}
	}

	// Re-read so response includes updated notification timestamps.
	if err := w.DB.Order("fsid asc, id asc").Find(&exports).Error; err != nil {
		return report, nil
	}
	return reports.CollectQuotas(exports), nil
}

func (w *Watcher) post(exp models.Export, item reports.QuotaUsage) error {
	rem := uint64(0)
	if item.RemainingBytes != nil {
		rem = *item.RemainingBytes
	}
	payload := FullEvent{
		Event:            "quota.full",
		ExportID:         exp.ID,
		Path:             exp.Path,
		UsedBytes:        item.UsedBytes,
		QuotaBytes:       exp.QuotaBytes,
		UsedPct:          item.UsedPct,
		RemainingBytes:   rem,
		QuotaPostbackURL: exp.QuotaPostbackURL,
		At:               time.Now().UTC(),
		Message:          "storage export reached its configured quota",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, exp.QuotaPostbackURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "nfs-manager-quota/1.0")
	req.Header.Set("X-NFS-Manager-Event", "quota.full")
	resp, err := w.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("postback HTTP %d", resp.StatusCode)
	}
	return nil
}
