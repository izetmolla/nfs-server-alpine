package mcpoauth

import (
	"time"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"gorm.io/gorm"
)

// Store persists OAuth clients, codes, and tokens via GORM.
type Store struct {
	DB *gorm.DB
}

func (s *Store) CreateClient(c *models.MCPOAuthClient) error {
	return s.DB.Create(c).Error
}

func (s *Store) FindClient(clientID string) (*models.MCPOAuthClient, error) {
	var c models.MCPOAuthClient
	err := s.DB.Where("client_id = ?", clientID).First(&c).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) ListClients() ([]models.MCPOAuthClient, error) {
	var items []models.MCPOAuthClient
	err := s.DB.Order("id desc").Find(&items).Error
	return items, err
}

func (s *Store) DeleteClient(id uint) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var c models.MCPOAuthClient
		if err := tx.First(&c, id).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&models.MCPToken{}).
			Where("client_id = ? AND revoked_at IS NULL", c.ClientID).
			Update("revoked_at", now).Error; err != nil {
			return err
		}
		return tx.Delete(&c).Error
	})
}

func (s *Store) InsertCode(code *models.MCPAuthCode) error {
	return s.DB.Create(code).Error
}

func (s *Store) TakeCode(code string) (*models.MCPAuthCode, error) {
	var c models.MCPAuthCode
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("code = ?", code).First(&c).Error; err != nil {
			return err
		}
		return tx.Delete(&models.MCPAuthCode{}, "code = ?", code).Error
	})
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) InsertToken(tok *models.MCPToken) error {
	return s.DB.Create(tok).Error
}

func (s *Store) FindByAccessToken(access string) (*models.MCPToken, error) {
	var t models.MCPToken
	err := s.DB.Where("access_token = ?", access).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) FindByRefreshToken(refresh string) (*models.MCPToken, error) {
	var t models.MCPToken
	err := s.DB.Where("refresh_token = ?", refresh).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) FindByAnyToken(token string) (*models.MCPToken, error) {
	var t models.MCPToken
	err := s.DB.Where("access_token = ? OR refresh_token = ?", token, token).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (s *Store) DeleteToken(tok *models.MCPToken) error {
	return s.DB.Delete(tok).Error
}

func (s *Store) TouchLastUsed(id uint) {
	now := time.Now().UTC()
	_ = s.DB.Model(&models.MCPToken{}).Where("id = ?", id).Update("last_used_at", now).Error
}

func (s *Store) RevokeToken(id uint) error {
	now := time.Now().UTC()
	res := s.DB.Model(&models.MCPToken{}).Where("id = ?", id).Update("revoked_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) RevokeByTokenValue(token string) error {
	now := time.Now().UTC()
	return s.DB.Model(&models.MCPToken{}).
		Where("(access_token = ? OR refresh_token = ?) AND revoked_at IS NULL", token, token).
		Update("revoked_at", now).Error
}

// SessionRow is a token joined with client name for the admin UI.
type SessionRow struct {
	ID         uint       `json:"id"`
	ClientName string     `json:"clientName"`
	ClientID   string     `json:"clientId"`
	Scope      string     `json:"scope"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	Revoked    bool       `json:"revoked"`
}

func (s *Store) ListSessions() ([]SessionRow, error) {
	type row struct {
		ID         uint
		ClientID   string
		Scope      string
		CreatedAt  time.Time
		ExpiresAt  time.Time
		LastUsedAt *time.Time
		RevokedAt  *time.Time
		ClientName string
	}
	var rows []row
	err := s.DB.Table("mcp_tokens AS t").
		Select("t.id, t.client_id, t.scope, t.created_at, t.expires_at, t.last_used_at, t.revoked_at, COALESCE(c.client_name, '') AS client_name").
		Joins("LEFT JOIN mcp_oauth_clients AS c ON c.client_id = t.client_id").
		Order("t.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]SessionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, SessionRow{
			ID:         r.ID,
			ClientName: r.ClientName,
			ClientID:   r.ClientID,
			Scope:      r.Scope,
			CreatedAt:  r.CreatedAt,
			ExpiresAt:  r.ExpiresAt,
			LastUsedAt: r.LastUsedAt,
			Revoked:    r.RevokedAt != nil,
		})
	}
	return out, nil
}
