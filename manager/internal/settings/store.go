package settings

import (
	"strconv"
	"strings"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/netutil"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	DB *gorm.DB
}

func (s *Store) Get(key string) (string, bool, error) {
	var row models.Setting
	err := s.DB.First(&row, "key = ?", key).Error
	if err == gorm.ErrRecordNotFound {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return row.Value, true, nil
}

func (s *Store) Set(key, value string) error {
	row := models.Setting{Key: key, Value: value}
	return s.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&row).Error
}

func (s *Store) LoadAppSettings() (models.AppSettings, error) {
	cfg := models.AppSettings{
		DefaultClients:    "*",
		SwaggerEnabled:    true,
		PanelAllowedCIDRs: "0.0.0.0/0",
	}

	if v, ok, err := s.Get(models.SettingDefaultClients); err != nil {
		return cfg, err
	} else if ok && v != "" {
		cfg.DefaultClients = v
	}

	if v, ok, err := s.Get(models.SettingDefaultReadOnly); err != nil {
		return cfg, err
	} else if ok {
		cfg.DefaultReadOnly = v == "true" || v == "1"
	}

	if v, ok, err := s.Get(models.SettingDefaultSync); err != nil {
		return cfg, err
	} else if ok {
		cfg.DefaultSync = v == "true" || v == "1"
	}

	if v, ok, err := s.Get(models.SettingDefaultExtraOpts); err != nil {
		return cfg, err
	} else if ok {
		cfg.DefaultExtraOpts = v
	}

	if v, ok, err := s.Get(models.SettingGlobalExtraOptions); err != nil {
		return cfg, err
	} else if ok {
		cfg.GlobalExtraOptions = v
	}

	if v, ok, err := s.Get(models.SettingSwaggerEnabled); err != nil {
		return cfg, err
	} else if ok {
		cfg.SwaggerEnabled = v == "true" || v == "1"
	}

	if v, ok, err := s.Get(models.SettingPanelAllowedCIDRs); err != nil {
		return cfg, err
	} else if ok && strings.TrimSpace(v) != "" {
		cfg.PanelAllowedCIDRs = v
	}

	if _, ok, err := s.Get(models.SettingPanelPassword); err != nil {
		return cfg, err
	} else {
		cfg.HasCustomPassword = ok
	}

	return cfg, nil
}

func (s *Store) SaveAppSettings(in models.AppSettings) (models.AppSettings, error) {
	clients, err := netutil.NormalizeAndValidateClients(in.DefaultClients)
	if err != nil {
		return models.AppSettings{}, err
	}
	in.DefaultClients = clients
	in.DefaultExtraOpts = sanitizeOptions(in.DefaultExtraOpts)
	in.GlobalExtraOptions = sanitizeOptions(in.GlobalExtraOptions)

	cidrs, err := netutil.NormalizeAndValidateClients(in.PanelAllowedCIDRs)
	if err != nil {
		return models.AppSettings{}, err
	}
	if cidrs == "" {
		cidrs = "0.0.0.0/0"
	}
	in.PanelAllowedCIDRs = cidrs

	pairs := map[string]string{
		models.SettingDefaultClients:     in.DefaultClients,
		models.SettingDefaultReadOnly:    strconv.FormatBool(in.DefaultReadOnly),
		models.SettingDefaultSync:        strconv.FormatBool(in.DefaultSync),
		models.SettingDefaultExtraOpts:   in.DefaultExtraOpts,
		models.SettingGlobalExtraOptions: in.GlobalExtraOptions,
		models.SettingSwaggerEnabled:     strconv.FormatBool(in.SwaggerEnabled),
		models.SettingPanelAllowedCIDRs:  in.PanelAllowedCIDRs,
	}
	for k, v := range pairs {
		if err := s.Set(k, v); err != nil {
			return models.AppSettings{}, err
		}
	}
	return s.LoadAppSettings()
}

func (s *Store) LoadPassword(fallback string) (string, error) {
	v, ok, err := s.Get(models.SettingPanelPassword)
	if err != nil {
		return fallback, err
	}
	if ok && strings.TrimSpace(v) != "" {
		return v, nil
	}
	return fallback, nil
}

func (s *Store) SavePassword(password string) error {
	return s.Set(models.SettingPanelPassword, password)
}

func (s *Store) LoadMCPSettings() (models.MCPSettings, error) {
	cfg := models.MCPSettings{
		Enabled:       false,
		AllowWrite:    false,
		PublicBaseURL: "",
	}
	if v, ok, err := s.Get(models.SettingMCPEnabled); err != nil {
		return cfg, err
	} else if ok {
		cfg.Enabled = v == "true" || v == "1"
	}
	if v, ok, err := s.Get(models.SettingMCPAllowWrite); err != nil {
		return cfg, err
	} else if ok {
		cfg.AllowWrite = v == "true" || v == "1"
	}
	if v, ok, err := s.Get(models.SettingMCPPublicBaseURL); err != nil {
		return cfg, err
	} else if ok {
		cfg.PublicBaseURL = strings.TrimSpace(v)
	}
	return cfg, nil
}

func (s *Store) SaveMCPSettings(in models.MCPSettings) (models.MCPSettings, error) {
	in.PublicBaseURL = strings.TrimRight(strings.TrimSpace(in.PublicBaseURL), "/")
	pairs := map[string]string{
		models.SettingMCPEnabled:       strconv.FormatBool(in.Enabled),
		models.SettingMCPAllowWrite:    strconv.FormatBool(in.AllowWrite),
		models.SettingMCPPublicBaseURL: in.PublicBaseURL,
	}
	for k, v := range pairs {
		if err := s.Set(k, v); err != nil {
			return models.MCPSettings{}, err
		}
	}
	return s.LoadMCPSettings()
}

func (s *Store) PublicConfig() (models.PublicConfig, error) {
	cfg, err := s.LoadAppSettings()
	if err != nil {
		return models.PublicConfig{}, err
	}
	mcp, err := s.LoadMCPSettings()
	if err != nil {
		return models.PublicConfig{}, err
	}
	return models.PublicConfig{
		SwaggerEnabled:    cfg.SwaggerEnabled,
		PanelAllowedCIDRs: cfg.PanelAllowedCIDRs,
		McpEnabled:        mcp.Enabled,
	}, nil
}

func sanitizeOptions(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.ContainsAny(p, " \t\n()\"'") {
			continue
		}
		clean = append(clean, p)
	}
	return strings.Join(clean, ",")
}
