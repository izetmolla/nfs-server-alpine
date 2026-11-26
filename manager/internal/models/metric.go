package models

import "time"

// MetricHour stores aggregated panel counters per UTC hour.
type MetricHour struct {
	Hour         time.Time `json:"hour" gorm:"primaryKey"`
	AuthOK       int       `json:"authOk"`
	AuthFail     int       `json:"authFail"`
	ApplyOK      int       `json:"applyOk"`
	ApplyFail    int       `json:"applyFail"`
	APIRequests  int       `json:"apiRequests"`
	FirewallDeny int       `json:"firewallDeny"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
