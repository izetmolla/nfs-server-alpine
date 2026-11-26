package mcpoauth

import (
	"net/http"
	"strings"
)

// cors wraps a handler with wide-open CORS for OAuth discovery / browser flows.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate, MCP-Session-Id")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Mount registers OAuth + well-known discovery routes on mux.
func Mount(mux *http.ServeMux, st *State) {
	wrap := func(h http.HandlerFunc) http.Handler {
		return cors(h)
	}
	prm := wrap(protectedResource(st))
	as := wrap(authorizationServer(st))

	// Root + path-inserted PRM (RFC 9728 / MCP clients try both).
	mux.Handle("GET /.well-known/oauth-authorization-server", as)
	mux.Handle("GET /.well-known/oauth-protected-resource", prm)
	mux.Handle("GET /.well-known/oauth-protected-resource/mcp", prm)
	mux.Handle("GET /mcp/.well-known/oauth-protected-resource", prm)

	mux.Handle("POST /oauth/register", wrap(register(st)))
	mux.Handle("GET /oauth/authorize", wrap(authorize(st)))
	mux.Handle("POST /oauth/login", wrap(login(st)))
	mux.Handle("POST /oauth/token", wrap(token(st)))
	mux.Handle("POST /oauth/revoke", wrap(revoke(st)))
	mux.Handle("OPTIONS /oauth/", wrap(func(w http.ResponseWriter, r *http.Request) {}))
	mux.Handle("OPTIONS /.well-known/", wrap(func(w http.ResponseWriter, r *http.Request) {}))
}

// IsMCPPath reports whether path is an MCP/OAuth route that should bypass the
// panel SPA catch-all (and optionally the panel firewall when MCP is enabled).
func IsMCPPath(path string) bool {
	if path == "/mcp" || strings.HasPrefix(path, "/mcp/") {
		return true
	}
	if strings.HasPrefix(path, "/oauth/") {
		return true
	}
	if strings.HasPrefix(path, "/.well-known/oauth-authorization-server") ||
		strings.HasPrefix(path, "/.well-known/oauth-protected-resource") {
		return true
	}
	return false
}
