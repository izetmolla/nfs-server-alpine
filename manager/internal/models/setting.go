package models

import "time"

type Setting struct {
	Key       string    `json:"key" gorm:"primaryKey"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updatedAt"`
}

const (
	SettingPanelPassword      = "panel_password"
	SettingDefaultClients     = "default_clients"
	SettingDefaultReadOnly    = "default_read_only"
	SettingDefaultSync        = "default_sync"
	SettingDefaultExtraOpts   = "default_extra_options"
	SettingGlobalExtraOptions = "global_extra_options"
	SettingSwaggerEnabled     = "swagger_enabled"
	SettingPanelAllowedCIDRs  = "panel_allowed_cidrs"
)

type AppSettings struct {
	DefaultClients     string `json:"defaultClients"`
	DefaultReadOnly    bool   `json:"defaultReadOnly"`
	DefaultSync        bool   `json:"defaultSync"`
	DefaultExtraOpts   string `json:"defaultExtraOptions"`
	GlobalExtraOptions string `json:"globalExtraOptions"`
	SwaggerEnabled     bool   `json:"swaggerEnabled"`
	PanelAllowedCIDRs  string `json:"panelAllowedCidrs"`
	HasCustomPassword  bool   `json:"hasCustomPassword"`
}

type PublicConfig struct {
	SwaggerEnabled    bool   `json:"swaggerEnabled"`
	PanelAllowedCIDRs string `json:"panelAllowedCidrs"`
}
