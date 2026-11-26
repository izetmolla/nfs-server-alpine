package mcpoauth

import (
	"context"
	"strings"

	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
)

type ctxKey int

const tokenCtxKey ctxKey = 1

// WithToken stores the authenticated MCP token on ctx.
func WithToken(ctx context.Context, tok *models.MCPToken) context.Context {
	return context.WithValue(ctx, tokenCtxKey, tok)
}

// TokenFromContext returns the MCP token from ctx, or nil.
func TokenFromContext(ctx context.Context) *models.MCPToken {
	tok, _ := ctx.Value(tokenCtxKey).(*models.MCPToken)
	return tok
}

// HasScope reports whether the token in ctx includes the required scope.
func HasScope(ctx context.Context, need string) bool {
	tok := TokenFromContext(ctx)
	if tok == nil {
		return false
	}
	need = strings.TrimSpace(need)
	if need == "" {
		return true
	}
	for _, s := range strings.Fields(tok.Scope) {
		if s == need {
			return true
		}
	}
	return false
}

// ScopeNFSRead / ScopeNFSWrite are MCP OAuth scopes.
const (
	ScopeNFSRead  = "nfs:read"
	ScopeNFSWrite = "nfs:write"
)
