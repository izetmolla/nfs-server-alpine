package models

import (
	"encoding/json"
	"strings"
	"time"
)

// MCPOAuthClient is a dynamically registered OAuth client for MCP.
type MCPOAuthClient struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	ClientID     string    `json:"clientId" gorm:"uniqueIndex;size:128;not null"`
	ClientSecret string    `json:"-" gorm:"not null"`
	ClientName   string    `json:"clientName"`
	RedirectURIs string    `json:"redirectUris" gorm:"type:text"` // JSON array or semicolon-joined
	CreatedAt    time.Time `json:"createdAt"`
}

// RedirectURIList parses RedirectURIs as JSON or semicolon-separated values.
func (c *MCPOAuthClient) RedirectURIList() []string {
	raw := strings.TrimSpace(c.RedirectURIs)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var uris []string
		if err := json.Unmarshal([]byte(raw), &uris); err == nil {
			return uris
		}
	}
	parts := strings.Split(raw, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SetRedirectURIs stores redirect URIs as a JSON array.
func (c *MCPOAuthClient) SetRedirectURIs(uris []string) {
	b, err := json.Marshal(uris)
	if err != nil {
		c.RedirectURIs = strings.Join(uris, ";")
		return
	}
	c.RedirectURIs = string(b)
}

// AllowsRedirect reports whether redirectURI is registered for this client.
func (c *MCPOAuthClient) AllowsRedirect(redirectURI string) bool {
	for _, u := range c.RedirectURIList() {
		if u == redirectURI {
			return true
		}
	}
	return false
}

// MCPAuthCode is a short-lived OAuth authorization code (PKCE).
type MCPAuthCode struct {
	Code                string    `json:"code" gorm:"primaryKey;size:128"`
	ClientID            string    `json:"clientId" gorm:"index;not null"`
	RedirectURI         string    `json:"redirectUri"`
	UserID              string    `json:"userId"`
	Scope               string    `json:"scope"`
	CodeChallenge       string    `json:"codeChallenge"`
	CodeChallengeMethod string    `json:"codeChallengeMethod"`
	ExpiresAt           time.Time `json:"expiresAt"`
}

// IsExpired reports whether the authorization code has expired.
func (c *MCPAuthCode) IsExpired() bool {
	return time.Now().UTC().After(c.ExpiresAt)
}

// MCPToken is an issued OAuth access/refresh token pair for MCP.
type MCPToken struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	AccessToken  string     `json:"-" gorm:"uniqueIndex;size:128;not null"`
	RefreshToken string     `json:"-" gorm:"uniqueIndex;size:128;not null"`
	UserID       string     `json:"userId"`
	ClientID     string     `json:"clientId" gorm:"index"`
	Scope        string     `json:"scope"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastUsedAt   *time.Time `json:"lastUsedAt,omitempty"`
}

// IsExpired reports whether the access token has expired.
func (t *MCPToken) IsExpired() bool {
	return time.Now().UTC().After(t.ExpiresAt)
}

// IsRevoked reports whether the token has been revoked.
func (t *MCPToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

// MCPSettings holds MCP feature flags and public URL configuration.
type MCPSettings struct {
	Enabled       bool   `json:"enabled"`
	AllowWrite    bool   `json:"allowWrite"`
	PublicBaseURL string `json:"publicBaseUrl"`
}
