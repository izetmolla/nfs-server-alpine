package auth

import (
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/session"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/tokens"
)

const (
	CookieName   = "nfs_panel_session"
	sessionKeyOK = "authenticated"
	CtxTokenKey  = "apiToken"
	CtxAuthKind  = "authKind"
)

type Config struct {
	Password string
}

type Guard struct {
	mu       sync.RWMutex
	password string
	handler  fiber.Handler
	store    *session.Store
	Tokens   *tokens.Store
}

func New(cfg Config, tokenStore *tokens.Store) *Guard {
	handler, store := session.NewWithStore(session.Config{
		CookieHTTPOnly:  true,
		CookieSameSite:  "Lax",
		CookieSecure:    false,
		IdleTimeout:     24 * time.Hour,
		AbsoluteTimeout: 7 * 24 * time.Hour,
		Extractor:       extractors.FromCookie(CookieName),
	})
	return &Guard{password: cfg.Password, handler: handler, store: store, Tokens: tokenStore}
}

func (g *Guard) MiddlewareSession() fiber.Handler {
	return g.handler
}

func (g *Guard) SetPassword(password string) {
	g.mu.Lock()
	g.password = password
	g.mu.Unlock()
}

func (g *Guard) CheckPassword(password string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return subtle.ConstantTimeCompare([]byte(password), []byte(g.password)) == 1
}

func (g *Guard) Login(c fiber.Ctx) error {
	sess := session.FromContext(c)
	if sess == nil {
		return fiber.NewError(fiber.StatusInternalServerError, "session unavailable")
	}
	sess.Set(sessionKeyOK, true)
	return nil
}

func (g *Guard) Logout(c fiber.Ctx) error {
	sess := session.FromContext(c)
	if sess == nil {
		return nil
	}
	return sess.Destroy()
}

func (g *Guard) RequireAuth(scopes ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if g.isPanelSession(c) {
			c.Locals(CtxAuthKind, "session")
			return c.Next()
		}
		if g.CheckPassword(bearerToken(c)) {
			c.Locals(CtxAuthKind, "password")
			return c.Next()
		}
		if tok, err := g.authenticateAPIToken(c); err == nil {
			for _, need := range scopes {
				if !g.Tokens.HasScope(tok, need) {
					return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "token missing scope: " + need})
				}
			}
			c.Locals(CtxAuthKind, "token")
			c.Locals(CtxTokenKey, tok)
			return c.Next()
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
}

func (g *Guard) RequirePanel() fiber.Handler {
	return func(c fiber.Ctx) error {
		if g.isPanelSession(c) || g.CheckPassword(bearerToken(c)) {
			return c.Next()
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "panel authentication required"})
	}
}

func (g *Guard) authenticateAPIToken(c fiber.Ctx) (*models.APIToken, error) {
	raw := bearerToken(c)
	if raw == "" || !strings.HasPrefix(raw, "nfs_") {
		return nil, fiber.ErrUnauthorized
	}
	return g.Tokens.Authenticate(raw, c.IP())
}

func (g *Guard) isPanelSession(c fiber.Ctx) bool {
	authHeader := c.Get("Authorization")
	if authHeader != "" {
		lower := strings.ToLower(authHeader)
		if strings.HasPrefix(lower, "basic ") {
			raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(authHeader[6:]))
			if err == nil {
				parts := strings.SplitN(string(raw), ":", 2)
				if len(parts) == 2 && g.CheckPassword(parts[1]) {
					return true
				}
				if len(parts) == 1 && g.CheckPassword(parts[0]) {
					return true
				}
			}
		}
	}
	sess := session.FromContext(c)
	if sess == nil {
		return false
	}
	ok, _ := sess.Get(sessionKeyOK).(bool)
	return ok
}

func bearerToken(c fiber.Ctx) string {
	authHeader := c.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return ""
}
