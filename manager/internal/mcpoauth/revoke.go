package mcpoauth

import (
	"net/http"
)

// revoke implements RFC 7009 token revocation.
// Always returns 200 per the RFC (even if the token is unknown).
func revoke(st *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "invalid form body")
			return
		}
		token := r.FormValue("token")
		if token == "" {
			oauthError(w, http.StatusBadRequest, "invalid_request", "token is required")
			return
		}
		clientID := r.FormValue("client_id")
		clientSecret := r.FormValue("client_secret")
		if clientID != "" {
			client, err := st.Store.FindClient(clientID)
			if err != nil {
				oauthError(w, http.StatusUnauthorized, "invalid_client", "unknown client")
				return
			}
			if clientSecret != "" && client.ClientSecret != clientSecret {
				oauthError(w, http.StatusUnauthorized, "invalid_client", "client secret mismatch")
				return
			}
			tok, err := st.Store.FindByAnyToken(token)
			if err == nil && tok.ClientID != clientID {
				// Token belongs to another client — still return 200 per RFC 7009 §2.2
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		_ = st.Store.RevokeByTokenValue(token)
		w.WriteHeader(http.StatusOK)
	}
}
