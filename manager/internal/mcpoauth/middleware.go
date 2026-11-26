package mcpoauth

import (
	"net/http"
	"strings"
)

// RequireBearerToken protects /mcp. Missing/invalid/expired/revoked tokens get a 401
// with WWW-Authenticate pointing at protected-resource metadata — that header
// is what MCP clients use to show the "Authorize" button.
func RequireBearerToken(st *State, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	w.Header().Set("WWW-Authenticate",
		`Bearer resource_metadata="`+base+`/.well-known/oauth-protected-resource"`)
	w.WriteHeader(http.StatusUnauthorized)
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
	return st.BaseURLForRequest(scheme, host)
}
