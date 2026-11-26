package tokens

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/netutil"
	"gorm.io/gorm"
)

type Store struct {
	DB *gorm.DB
}

type CreateInput struct {
	Name         string     `json:"name"`
	AllowedCIDRs string     `json:"allowedCidrs"`
	Scopes       []string   `json:"scopes"`
	ExpiresAt    *time.Time `json:"expiresAt"`
}

type Created struct {
	Token models.APIToken `json:"token"`
	Raw   string          `json:"rawToken"`
}

func (s *Store) List() ([]models.APIToken, error) {
	var items []models.APIToken
	err := s.DB.Order("id desc").Find(&items).Error
	return items, err
}

func (s *Store) Create(in CreateInput) (*Created, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	cidrs, err := netutil.NormalizeAndValidateClients(in.AllowedCIDRs)
	if err != nil {
		return nil, fmt.Errorf("allowedCidrs: %w", err)
	}
	scopes := normalizeScopes(in.Scopes)
	if len(scopes) == 0 {
		return nil, fmt.Errorf("at least one scope is required")
	}

	raw, err := randomToken()
	if err != nil {
		return nil, err
	}
	hash := hashToken(raw)
	prefix := raw
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}

	item := models.APIToken{
		Name:         name,
		TokenHash:    hash,
		TokenPrefix:  prefix,
		AllowedCIDRs: cidrs,
		Scopes:       strings.Join(scopes, " "),
		Enabled:      true,
		ExpiresAt:    in.ExpiresAt,
	}
	if err := s.DB.Create(&item).Error; err != nil {
		return nil, err
	}
	return &Created{Token: item, Raw: raw}, nil
}

func (s *Store) Update(id uint, enabled *bool, name, cidrs string, scopes []string, expiresAt *time.Time) (*models.APIToken, error) {
	var item models.APIToken
	if err := s.DB.First(&item, id).Error; err != nil {
		return nil, err
	}
	if name = strings.TrimSpace(name); name != "" {
		item.Name = name
	}
	if cidrs != "" {
		normalized, err := netutil.NormalizeAndValidateClients(cidrs)
		if err != nil {
			return nil, err
		}
		item.AllowedCIDRs = normalized
	}
	if scopes != nil {
		sc := normalizeScopes(scopes)
		if len(sc) == 0 {
			return nil, fmt.Errorf("at least one scope is required")
		}
		item.Scopes = strings.Join(sc, " ")
	}
	if enabled != nil {
		item.Enabled = *enabled
	}
	item.ExpiresAt = expiresAt
	if err := s.DB.Save(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) Delete(id uint) error {
	res := s.DB.Delete(&models.APIToken{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) Authenticate(raw, clientIP string) (*models.APIToken, error) {
	if raw == "" {
		return nil, fmt.Errorf("missing token")
	}
	var item models.APIToken
	if err := s.DB.Where("token_hash = ?", hashToken(raw)).First(&item).Error; err != nil {
		return nil, fmt.Errorf("invalid token")
	}
	if !item.Enabled {
		return nil, fmt.Errorf("token disabled")
	}
	if item.ExpiresAt != nil && time.Now().After(*item.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}
	if !netutil.IPAllowed(clientIP, item.AllowedCIDRs) {
		return nil, fmt.Errorf("client IP not allowed for token")
	}
	now := time.Now().UTC()
	_ = s.DB.Model(&item).Update("last_used_at", now).Error
	item.LastUsedAt = &now
	return &item, nil
}

func (s *Store) HasScope(token *models.APIToken, need string) bool {
	if token == nil {
		return false
	}
	for _, sc := range strings.Fields(token.Scopes) {
		if sc == need || sc == models.ScopeTokensAdmin {
			return true
		}
	}
	return false
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "nfs_" + hex.EncodeToString(buf), nil
}

func normalizeScopes(in []string) []string {
	allowed := map[string]struct{}{
		models.ScopeExportsRead:   {},
		models.ScopeExportsWrite:  {},
		models.ScopeExportsDelete: {},
		models.ScopeSettingsRead:  {},
		models.ScopeSettingsWrite: {},
		models.ScopeLogsRead:      {},
		models.ScopeTokensAdmin:   {},
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if _, ok := allowed[s]; !ok {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

