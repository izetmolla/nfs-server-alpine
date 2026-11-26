package mcpoauth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
)

func token(st *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "invalid form body")
			return
		}
		grantType := r.FormValue("grant_type")
		switch grantType {
		case "authorization_code":
			authorizationCodeGrant(w, st, r)
		case "refresh_token":
			refreshTokenGrant(w, st, r)
		default:
			oauthError(w, http.StatusBadRequest, "unsupported_grant_type",
				"grant_type '"+grantType+"' is not supported")
		}
	}
}

func oauthError(w http.ResponseWriter, status int, errCode, description string) {
	writeJSON(w, status, map[string]string{
		"error":             errCode,
		"error_description": description,
	})
}

func authorizationCodeGrant(w http.ResponseWriter, st *State, r *http.Request) {
	code := r.FormValue("code")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	redirectURI := r.FormValue("redirect_uri")
	codeVerifier := r.FormValue("code_verifier")

	authCode, err := st.Store.TakeCode(code)
	if err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "unknown authorization code")
		return
	}
	if authCode.IsExpired() {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "authorization code expired")
		return
	}
	if clientID != "" && authCode.ClientID != clientID {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "client_id mismatch")
		return
	}
	if redirectURI != "" && authCode.RedirectURI != redirectURI {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
		return
	}

	client, err := st.Store.FindClient(authCode.ClientID)
	if err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_client", "unknown client")
		return
	}
	if clientSecret != "" && client.ClientSecret != clientSecret {
		oauthError(w, http.StatusUnauthorized, "invalid_client", "client secret mismatch")
		return
	}
	if !VerifyPKCE(codeVerifier, authCode.CodeChallenge, authCode.CodeChallengeMethod) {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "PKCE verification failed")
		return
	}

	issueToken(w, st, authCode.UserID, authCode.ClientID, authCode.Scope)
}

func refreshTokenGrant(w http.ResponseWriter, st *State, r *http.Request) {
	refresh := r.FormValue("refresh_token")
	old, err := st.Store.FindByRefreshToken(refresh)
	if err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "unknown refresh token")
		return
	}
	if old.IsRevoked() {
		oauthError(w, http.StatusBadRequest, "invalid_grant", "refresh token revoked")
		return
	}
	_ = st.Store.DeleteToken(old)
	issueToken(w, st, old.UserID, old.ClientID, old.Scope)
}

func issueToken(w http.ResponseWriter, st *State, userID, clientID, scope string) {
	const expiresIn = 3600
	access := "mcp-token-" + uuid.NewString()
	refresh := "mcp-refresh-" + uuid.NewString()
	tok := models.MCPToken{
		AccessToken:  access,
		RefreshToken: refresh,
		UserID:       userID,
		ClientID:     clientID,
		Scope:        scope,
		ExpiresAt:    time.Now().UTC().Add(expiresIn * time.Second),
		CreatedAt:    time.Now().UTC(),
	}
	if err := st.Store.InsertToken(&tok); err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    expiresIn,
		"refresh_token": refresh,
		"scope":         scope,
	})
}
