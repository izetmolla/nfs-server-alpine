package metrics

import (
	"strings"
	"sync"
	"time"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Point struct {
	At           string `json:"at"` // RFC3339 bucket start
	AuthOK       int    `json:"authOk"`
	AuthFail     int    `json:"authFail"`
	ApplyOK      int    `json:"applyOk"`
	ApplyFail    int    `json:"applyFail"`
	APIRequests  int    `json:"apiRequests"`
	FirewallDeny int    `json:"firewallDeny"`
}

// MinuteBucket kept for API compatibility with older clients.
type MinuteBucket = Point

type RangeInfo struct {
	Preset      string    `json:"preset"`
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	Granularity string    `json:"granularity"` // hour | day | week
}

type Snapshot struct {
	AuthOK       uint64    `json:"authOk"`
	AuthFail     uint64    `json:"authFail"`
	ApplyOK      uint64    `json:"applyOk"`
	ApplyFail    uint64    `json:"applyFail"`
	APIRequests  uint64    `json:"apiRequests"`
	FirewallDeny uint64    `json:"firewallDeny"`
	StartedAt    time.Time `json:"startedAt"`
	Series       []Point   `json:"series"`
	Range        RangeInfo `json:"range"`
	Partial      bool      `json:"partial"`
	CoverageFrom *time.Time `json:"coverageFrom,omitempty"`
	Note         string    `json:"note,omitempty"`
}

type Collector struct {
	mu        sync.Mutex
	db        *gorm.DB
	startedAt time.Time
	// pending deltas for current hour, flushed on timer / Snapshot
	pending models.MetricHour
	curHour time.Time
}

func New(db *gorm.DB) *Collector {
	c := &Collector{
		db:        db,
		startedAt: time.Now().UTC(),
		curHour:   time.Now().UTC().Truncate(time.Hour),
	}
	go c.flushLoop()
	return c
}

func (c *Collector) IncAuthOK()       { c.add(func(b *models.MetricHour) { b.AuthOK++ }) }
func (c *Collector) IncAuthFail()     { c.add(func(b *models.MetricHour) { b.AuthFail++ }) }
func (c *Collector) IncApplyOK()      { c.add(func(b *models.MetricHour) { b.ApplyOK++ }) }
func (c *Collector) IncApplyFail()    { c.add(func(b *models.MetricHour) { b.ApplyFail++ }) }
func (c *Collector) IncAPIRequest()   { c.add(func(b *models.MetricHour) { b.APIRequests++ }) }
func (c *Collector) IncFirewallDeny() { c.add(func(b *models.MetricHour) { b.FirewallDeny++ }) }

func (c *Collector) add(fn func(*models.MetricHour)) {
	now := time.Now().UTC().Truncate(time.Hour)
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.curHour.Equal(now) {
		c.flushLocked()
		c.curHour = now
		c.pending = models.MetricHour{Hour: now}
	}
	if c.pending.Hour.IsZero() {
		c.pending.Hour = now
	}
	fn(&c.pending)
}

func (c *Collector) flushLoop() {
	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	for range t.C {
		c.Flush()
	}
}

func (c *Collector) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.flushLocked()
}

func (c *Collector) flushLocked() {
	if c.db == nil {
		return
	}
	p := c.pending
	if p.Hour.IsZero() {
		return
	}
	if p.AuthOK == 0 && p.AuthFail == 0 && p.ApplyOK == 0 && p.ApplyFail == 0 && p.APIRequests == 0 && p.FirewallDeny == 0 {
		return
	}
	_ = c.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "hour"}},
		DoUpdates: clause.Assignments(map[string]any{
			"auth_ok":       gorm.Expr("auth_ok + ?", p.AuthOK),
			"auth_fail":     gorm.Expr("auth_fail + ?", p.AuthFail),
			"apply_ok":      gorm.Expr("apply_ok + ?", p.ApplyOK),
			"apply_fail":    gorm.Expr("apply_fail + ?", p.ApplyFail),
			"api_requests":  gorm.Expr("api_requests + ?", p.APIRequests),
			"firewall_deny": gorm.Expr("firewall_deny + ?", p.FirewallDeny),
			"updated_at":    time.Now().UTC(),
		}),
	}).Create(&models.MetricHour{
		Hour:         p.Hour,
		AuthOK:       p.AuthOK,
		AuthFail:     p.AuthFail,
		ApplyOK:      p.ApplyOK,
		ApplyFail:    p.ApplyFail,
		APIRequests:  p.APIRequests,
		FirewallDeny: p.FirewallDeny,
		UpdatedAt:    time.Now().UTC(),
	})
	c.pending = models.MetricHour{Hour: c.curHour}
	// prune older than ~14 months
	cutoff := time.Now().UTC().AddDate(-1, -2, 0).Truncate(time.Hour)
	_ = c.db.Where("hour < ?", cutoff).Delete(&models.MetricHour{}).Error
}

type Query struct {
	Preset string
	From   time.Time
	To     time.Time
}

func ResolveRange(preset, fromStr, toStr string, now time.Time) RangeInfo {
	now = now.UTC()
	preset = strings.TrimSpace(strings.ToLower(preset))
	if preset == "" {
		preset = "last_7_days"
	}

	var from, to time.Time
	switch preset {
	case "today":
		from = startOfDay(now)
		to = now
	case "yesterday":
		y := now.AddDate(0, 0, -1)
		from = startOfDay(y)
		to = startOfDay(now).Add(-time.Nanosecond)
	case "this_month":
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		to = now
	case "last_month":
		firstThis := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		from = firstThis.AddDate(0, -1, 0)
		to = firstThis.Add(-time.Nanosecond)
	case "this_year":
		from = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)
		to = now
	case "custom":
		from = parseTime(fromStr, startOfDay(now.AddDate(0, 0, -7)))
		to = parseTime(toStr, now)
		if to.Before(from) {
			from, to = to, from
		}
	default: // last_7_days
		preset = "last_7_days"
		from = startOfDay(now.AddDate(0, 0, -6))
		to = now
	}

	granularity := "day"
	span := to.Sub(from)
	if span <= 48*time.Hour {
		granularity = "hour"
	} else if span > 90*24*time.Hour {
		granularity = "week"
	}

	return RangeInfo{
		Preset:      preset,
		From:        from,
		To:          to,
		Granularity: granularity,
	}
}

func parseTime(raw string, fallback time.Time) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if t, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return t.UTC()
		}
	}
	return fallback
}

func startOfDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func (c *Collector) Snapshot() Snapshot {
	return c.SnapshotRange(ResolveRange("last_7_days", "", "", time.Now().UTC()))
}

func (c *Collector) SnapshotRange(r RangeInfo) Snapshot {
	c.Flush()

	c.mu.Lock()
	started := c.startedAt
	c.mu.Unlock()

	snap := Snapshot{
		StartedAt: started,
		Range:     r,
		Series:    []Point{},
	}

	if c.db == nil {
		snap.Note = "metrics store unavailable"
		return snap
	}

	fromHour := r.From.UTC().Truncate(time.Hour)
	toHour := r.To.UTC().Truncate(time.Hour)

	var rows []models.MetricHour
	_ = c.db.Where("hour >= ? AND hour <= ?", fromHour, toHour).
		Order("hour asc").
		Find(&rows).Error

	byHour := map[int64]models.MetricHour{}
	var coverage *time.Time
	for _, row := range rows {
		byHour[row.Hour.Unix()] = row
		if coverage == nil || row.Hour.Before(*coverage) {
			t := row.Hour
			coverage = &t
		}
		snap.AuthOK += uint64(row.AuthOK)
		snap.AuthFail += uint64(row.AuthFail)
		snap.ApplyOK += uint64(row.ApplyOK)
		snap.ApplyFail += uint64(row.ApplyFail)
		snap.APIRequests += uint64(row.APIRequests)
		snap.FirewallDeny += uint64(row.FirewallDeny)
	}

	// Include unflushed pending for current hour if in range.
	c.mu.Lock()
	pending := c.pending
	c.mu.Unlock()
	if !pending.Hour.IsZero() && !pending.Hour.Before(fromHour) && !pending.Hour.After(toHour) {
		key := pending.Hour.Unix()
		cur := byHour[key]
		cur.Hour = pending.Hour
		cur.AuthOK += pending.AuthOK
		cur.AuthFail += pending.AuthFail
		cur.ApplyOK += pending.ApplyOK
		cur.ApplyFail += pending.ApplyFail
		cur.APIRequests += pending.APIRequests
		cur.FirewallDeny += pending.FirewallDeny
		byHour[key] = cur
		snap.AuthOK += uint64(pending.AuthOK)
		snap.AuthFail += uint64(pending.AuthFail)
		snap.ApplyOK += uint64(pending.ApplyOK)
		snap.ApplyFail += uint64(pending.ApplyFail)
		snap.APIRequests += uint64(pending.APIRequests)
		snap.FirewallDeny += uint64(pending.FirewallDeny)
		if coverage == nil || pending.Hour.Before(*coverage) {
			t := pending.Hour
			coverage = &t
		}
	}

	snap.Series = buildSeries(r, byHour)
	snap.CoverageFrom = coverage
	if coverage == nil {
		snap.Partial = true
		snap.Note = "No metrics collected yet for this period — charts show zeros until activity is recorded."
	} else if coverage.After(r.From.Add(time.Hour)) {
		snap.Partial = true
		snap.Note = "Partial history: collection started " + coverage.Format("2006-01-02 15:04 UTC") + ". Earlier buckets are shown as zero."
	}
	return snap
}

func buildSeries(r RangeInfo, byHour map[int64]models.MetricHour) []Point {
	switch r.Granularity {
	case "hour":
		out := []Point{}
		for t := r.From.UTC().Truncate(time.Hour); !t.After(r.To.UTC()); t = t.Add(time.Hour) {
			row := byHour[t.Unix()]
			out = append(out, Point{
				At:           t.Format(time.RFC3339),
				AuthOK:       row.AuthOK,
				AuthFail:     row.AuthFail,
				ApplyOK:      row.ApplyOK,
				ApplyFail:    row.ApplyFail,
				APIRequests:  row.APIRequests,
				FirewallDeny: row.FirewallDeny,
			})
		}
		return out
	case "week":
		out := []Point{}
		start := startOfDay(r.From)
		// align to Monday UTC
		weekday := int(start.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = start.AddDate(0, 0, -(weekday - 1))
		for t := start; !t.After(r.To.UTC()); t = t.AddDate(0, 0, 7) {
			end := t.AddDate(0, 0, 7)
			p := Point{At: t.Format(time.RFC3339)}
			for h, row := range byHour {
				ht := time.Unix(h, 0).UTC()
				if !ht.Before(t) && ht.Before(end) && !ht.After(r.To.UTC()) {
					p.AuthOK += row.AuthOK
					p.AuthFail += row.AuthFail
					p.ApplyOK += row.ApplyOK
					p.ApplyFail += row.ApplyFail
					p.APIRequests += row.APIRequests
					p.FirewallDeny += row.FirewallDeny
				}
			}
			if !t.After(r.To.UTC()) {
				out = append(out, p)
			}
		}
		return out
	default: // day
		out := []Point{}
		for t := startOfDay(r.From); !t.After(r.To.UTC()); t = t.AddDate(0, 0, 1) {
			end := t.AddDate(0, 0, 1)
			p := Point{At: t.Format(time.RFC3339)}
			for h, row := range byHour {
				ht := time.Unix(h, 0).UTC()
				if !ht.Before(t) && ht.Before(end) {
					p.AuthOK += row.AuthOK
					p.AuthFail += row.AuthFail
					p.ApplyOK += row.ApplyOK
					p.ApplyFail += row.ApplyFail
					p.APIRequests += row.APIRequests
					p.FirewallDeny += row.FirewallDeny
				}
			}
			out = append(out, p)
		}
		return out
	}
}
