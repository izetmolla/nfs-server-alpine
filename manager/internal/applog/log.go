package applog

import (
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Entry struct {
	ID      uint64    `json:"id"`
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Source  string    `json:"source"`
	Message string    `json:"message"`
	Detail  string    `json:"detail,omitempty"`
}

type Ring struct {
	mu      sync.RWMutex
	entries []Entry
	size    int
	seq     atomic.Uint64
	waiters []chan struct{}
}

type QueryParams struct {
	AfterID  uint64
	Level    string // comma-separated: info,success,warn,error
	Source   string
	Search   string
	Sort     string // time | level | source | id
	Order    string // asc | desc
	Page     int
	PageSize int
}

type QueryResult struct {
	Entries  []Entry        `json:"entries"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
	Levels   map[string]int `json:"levels"`
	Sources  []string       `json:"sources"`
}

func New(size int) *Ring {
	if size < 50 {
		size = 50
	}
	return &Ring{size: size, entries: make([]Entry, 0, size)}
}

func (r *Ring) Info(source, message, detail string) {
	r.add("info", source, message, detail)
}

func (r *Ring) Success(source, message, detail string) {
	r.add("success", source, message, detail)
}

func (r *Ring) Warn(source, message, detail string) {
	r.add("warn", source, message, detail)
}

func (r *Ring) Error(source, message, detail string) {
	r.add("error", source, message, detail)
}

func (r *Ring) add(level, source, message, detail string) {
	entry := Entry{
		ID:      r.seq.Add(1),
		Time:    time.Now().UTC(),
		Level:   level,
		Source:  source,
		Message: message,
		Detail:  detail,
	}

	r.mu.Lock()
	r.entries = append(r.entries, entry)
	if len(r.entries) > r.size {
		r.entries = r.entries[len(r.entries)-r.size:]
	}
	waiters := r.waiters
	r.waiters = nil
	r.mu.Unlock()

	for _, ch := range waiters {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (r *Ring) Since(afterID uint64, limit int) []Entry {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Entry, 0, limit)
	for i := len(r.entries) - 1; i >= 0; i-- {
		if r.entries[i].ID <= afterID {
			break
		}
		out = append(out, r.entries[i])
		if len(out) >= limit {
			break
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func (r *Ring) Query(p QueryParams) QueryResult {
	if p.Page <= 0 {
		p.Page = 1
	}
	if p.PageSize <= 0 || p.PageSize > 200 {
		p.PageSize = 25
	}
	if p.Sort == "" {
		p.Sort = "time"
	}
	if p.Order == "" {
		p.Order = "desc"
	}

	levelsWanted := map[string]struct{}{}
	for _, part := range strings.Split(p.Level, ",") {
		part = strings.TrimSpace(strings.ToLower(part))
		if part != "" {
			levelsWanted[part] = struct{}{}
		}
	}
	search := strings.ToLower(strings.TrimSpace(p.Search))
	sourceFilter := strings.ToLower(strings.TrimSpace(p.Source))

	r.mu.RLock()
	defer r.mu.RUnlock()

	levels := map[string]int{}
	sourceSet := map[string]struct{}{}
	filtered := make([]Entry, 0, len(r.entries))
	for _, e := range r.entries {
		levels[e.Level]++
		sourceSet[e.Source] = struct{}{}
		if p.AfterID > 0 && e.ID <= p.AfterID {
			continue
		}
		if len(levelsWanted) > 0 {
			if _, ok := levelsWanted[e.Level]; !ok {
				continue
			}
		}
		if sourceFilter != "" && strings.ToLower(e.Source) != sourceFilter {
			continue
		}
		if search != "" {
			hay := strings.ToLower(e.Message + " " + e.Detail + " " + e.Source + " " + e.Level)
			if !strings.Contains(hay, search) {
				continue
			}
		}
		filtered = append(filtered, e)
	}

	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, s)
	}
	sort.Strings(sources)

	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		var less bool
		switch p.Sort {
		case "level":
			less = a.Level < b.Level
		case "source":
			less = a.Source < b.Source
		case "id":
			less = a.ID < b.ID
		default:
			less = a.Time.Before(b.Time)
		}
		if p.Order == "asc" {
			return less
		}
		return !less
	})

	total := len(filtered)
	start := (p.Page - 1) * p.PageSize
	if start > total {
		start = total
	}
	end := start + p.PageSize
	if end > total {
		end = total
	}

	return QueryResult{
		Entries:  filtered[start:end],
		Total:    total,
		Page:     p.Page,
		PageSize: p.PageSize,
		Levels:   levels,
		Sources:  sources,
	}
}

func (r *Ring) LevelCounts() map[string]int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := map[string]int{}
	for _, e := range r.entries {
		out[e.Level]++
	}
	return out
}

func (r *Ring) Wait(timeout time.Duration) {
	ch := make(chan struct{}, 1)
	r.mu.Lock()
	r.waiters = append(r.waiters, ch)
	r.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ch:
	case <-timer.C:
	}
}

func (r *Ring) Clear() {
	r.mu.Lock()
	r.entries = r.entries[:0]
	r.mu.Unlock()
	r.Info("system", "log buffer cleared", "")
}
