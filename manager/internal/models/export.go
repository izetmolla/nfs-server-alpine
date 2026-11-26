package models

import "time"

type Export struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Path         string    `json:"path" gorm:"not null"`
	Description  string    `json:"description" gorm:"default:''"`
	Clients      string    `json:"clients" gorm:"not null;default:*"`
	AccessMode   string    `json:"accessMode" gorm:"not null;default:rw"`
	CommitMode   string    `json:"commitMode" gorm:"not null;default:async"`
	WriteDelay   string    `json:"writeDelay" gorm:"not null;default:wdelay"`
	SubtreeCheck string    `json:"subtreeCheck" gorm:"not null;default:no_subtree_check"`
	PortMode     string    `json:"portMode" gorm:"not null;default:insecure"`
	LockMode     string    `json:"lockMode" gorm:"not null;default:insecure_locks"`
	HideMode     string    `json:"hideMode" gorm:"not null;default:hide"`
	Crossmnt     bool      `json:"crossmnt" gorm:"not null;default:false"`
	RootSquash   string    `json:"rootSquash" gorm:"not null;default:no_root_squash"`
	AllSquash    string    `json:"allSquash" gorm:"not null;default:no_all_squash"`
	Security     string    `json:"security" gorm:"not null;default:sys"`
	Fsid         uint      `json:"fsid" gorm:"not null;default:0"`
	AnonUID      *int      `json:"anonUid"`
	AnonGID      *int      `json:"anonGid"`
	Enabled      bool      `json:"enabled" gorm:"not null;default:true"`
	// QuotaBytes 0 = unlimited (default). Soft limit based on directory usage.
	QuotaBytes       int64      `json:"quotaBytes" gorm:"not null;default:0"`
	QuotaPostbackURL string     `json:"quotaPostbackUrl" gorm:"default:''"`
	QuotaNotifiedAt  *time.Time `json:"quotaNotifiedAt,omitempty"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`

	// Legacy columns kept for migration compatibility.
	ReadOnly bool `json:"readOnly,omitempty" gorm:"column:read_only;default:false"`
	Sync     bool `json:"sync,omitempty" gorm:"column:sync;default:false"`
}

type ExportDTO struct {
	Export
	ExportLine string `json:"exportLine"`
}

func DefaultExport() Export {
	return Export{
		Clients:      "*",
		AccessMode:   "rw",
		CommitMode:   "async",
		WriteDelay:   "wdelay",
		SubtreeCheck: "no_subtree_check",
		PortMode:     "insecure",
		LockMode:     "insecure_locks",
		HideMode:     "hide",
		Crossmnt:     false,
		RootSquash:   "no_root_squash",
		AllSquash:    "no_all_squash",
		Security:     "sys",
		Fsid:         0,
		Enabled:      true,
	}
}
