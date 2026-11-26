package mcpoauth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// RequireBearerToken protects /mcp. Missing/invalid/expired/revoked tokens get a 401
// with WWW-Authenticate pointing at protected-resource metadata — that header
// is what MCP clients (Cursor, Claude, …) use to show the "Authorize" button.
func RequireBearerToken(st *State, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Preflight must succeed without a token so browsers/clients can discover auth.
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || token == "" {
			unauthorized(w, st, r)
			return
		}
		tok, err := st.Store.FindByAccessToken(token)
		if err != nil || tok.IsExpired() || tok.IsRevoked() {
			unauthorized(w, st, r)
			return
		}
		st.Store.TouchLastUsed(tok.ID)
		ctx := WithToken(r.Context(), tok)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func unauthorized(w http.ResponseWriter, st *State, r *http.Request) {
	base := requestBaseURL(st, r)
	meta := base + "/.well-known/oauth-protected-resource/mcp"
	// Match go-sdk / RFC 9728 challenge shape Cursor expects for needsAuth.
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(
		`Bearer FAKESECRET_g3h4i5j6k7l8m9n0o1p2="%s", scope="%s %s", error="invalid_token", error_description="Authentication required"`,
		meta, ScopeNFSRead, ScopeNFSWrite,
	))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":             "invalid_token",
		"error_description": "Authentication required",
	})
}

func requestBaseURL(st *State, r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	host := r.Host
	if fwd := r.Header.Get("X-Forwarded-Host"); fwd != "" {
		host = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	// Prefer the host the client actually used so discovery URLs match the MCP URL
	// in Cursor (wrong PublicBaseURL otherwise yields Error instead of Authorize).
	if host != "" {
		return scheme + "://" + host
	}
	return st.BaseURLForRequest(scheme, host)
}
