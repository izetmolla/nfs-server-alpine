package models

import "time"

type APIToken struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	Name         string     `json:"name" gorm:"not null"`
	TokenHash    string     `json:"-" gorm:"not null;uniqueIndex"`
	TokenPrefix  string     `json:"tokenPrefix" gorm:"not null"`
	AllowedCIDRs string     `json:"allowedCidrs" gorm:"not null;default:*"`
	Scopes       string     `json:"scopes" gorm:"not null"`
	Enabled      bool       `json:"enabled" gorm:"not null;default:true"`
	ExpiresAt    *time.Time `json:"expiresAt"`
	LastUsedAt   *time.Time `json:"lastUsedAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

const (
	ScopeExportsRead   = "exports:read"
	ScopeExportsWrite  = "exports:write"
	ScopeExportsDelete = "exports:delete"
	ScopeSettingsRead  = "settings:read"
	ScopeSettingsWrite = "settings:write"
	ScopeLogsRead      = "logs:read"
	ScopeTokensAdmin   = "tokens:admin"
)
