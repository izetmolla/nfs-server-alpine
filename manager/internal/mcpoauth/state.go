package mcpoauth

import (
	"os"
	"strings"

	"gorm.io/gorm"
)

// State is shared across OAuth handlers and the bearer middleware.
type State struct {
	DB              *gorm.DB
	Store           *Store
	PasswordChecker func(user, pass string) bool
	// BaseURLProvider returns the configured public base URL (no trailing slash).
	// Empty means derive from MCP_PUBLIC_BASE_URL env or the incoming request.
	BaseURLProvider func() string
}

// NewState builds OAuth state with a GORM-backed store.
func NewState(db *gorm.DB, passwordChecker func(user, pass string) bool, baseURLProvider func() string) *State {
	return &State{
		DB:              db,
		Store:           &Store{DB: db},
		PasswordChecker: passwordChecker,
		BaseURLProvider: baseURLProvider,
	}
}

// ResolveBaseURL returns the public base URL without a trailing slash.
// Order: BaseURLProvider → MCP_PUBLIC_BASE_URL env → empty (caller may derive from request).
func (s *State) ResolveBaseURL() string {
	if s.BaseURLProvider != nil {
		if u := strings.TrimRight(strings.TrimSpace(s.BaseURLProvider()), "/"); u != "" {
			return u
		}
	}
	if u := strings.TrimRight(strings.TrimSpace(os.Getenv("MCP_PUBLIC_BASE_URL")), "/"); u != "" {
		return u
	}
	return ""
}

// BaseURLForRequest returns ResolveBaseURL, or derives scheme://host from r when empty.
func (s *State) BaseURLForRequest(scheme, host string) string {
	if u := s.ResolveBaseURL(); u != "" {
		return u
	}
	scheme = strings.TrimSpace(scheme)
	host = strings.TrimSpace(host)
	if scheme == "" {
		scheme = "http"
	}
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

// MCPResourceURL returns the protected MCP endpoint URL.
func (s *State) MCPResourceURL(base string) string {
	return strings.TrimRight(base, "/") + "/mcp"
}
