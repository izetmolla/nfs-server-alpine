package mcpoauth

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
)

func login(st *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
			return
		}

		form := LoginView{
			ClientID:            r.FormValue("client_id"),
			RedirectURI:         r.FormValue("redirect_uri"),
			Scope:               r.FormValue("scope"),
			State:               r.FormValue("state"),
			CodeChallenge:       r.FormValue("code_challenge"),
			CodeChallengeMethod: r.FormValue("code_challenge_method"),
		}
		username := r.FormValue("username")
		password := r.FormValue("password")

		client, err := st.Store.FindClient(form.ClientID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_client"})
			return
		}
		if !client.AllowsRedirect(form.RedirectURI) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":             "invalid_request",
				"error_description": "redirect_uri mismatch",
			})
			return
		}

		ok := false
		if st.PasswordChecker != nil {
			ok = st.PasswordChecker(username, password)
		}
		if !ok {
			form.ClientName = client.ClientName
			form.Error = "Invalid username or password"
			renderLogin(w, http.StatusUnauthorized, form)
			return
		}

		userID := strings.ToLower(strings.TrimSpace(username))
		if userID == "" {
			userID = "admin"
		}
		code := "mcp-code-" + uuid.NewString()
		authCode := models.MCPAuthCode{
			Code:                code,
			ClientID:            form.ClientID,
			RedirectURI:         form.RedirectURI,
			UserID:              userID,
			Scope:               form.Scope,
			CodeChallenge:       form.CodeChallenge,
			CodeChallengeMethod: form.CodeChallengeMethod,
			ExpiresAt:           time.Now().UTC().Add(5 * time.Minute),
		}
		if err := st.Store.InsertCode(&authCode); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error":             "server_error",
				"error_description": err.Error(),
			})
			return
		}

		redirectURL, err := url.Parse(form.RedirectURI)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":             "invalid_request",
				"error_description": "invalid redirect_uri",
			})
			return
		}
		q := redirectURL.Query()
		q.Set("code", code)
		if form.State != "" {
			q.Set("state", form.State)
		}
		redirectURL.RawQuery = q.Encode()
		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	}
}

// AdminPasswordChecker returns a PasswordChecker that accepts username "admin"
// (case-insensitive) and the panel password via checkPassword.
func AdminPasswordChecker(checkPassword func(pass string) bool) func(user, pass string) bool {
	return func(user, pass string) bool {
		if !strings.EqualFold(strings.TrimSpace(user), "admin") {
			return false
		}
		return checkPassword(pass)
	}
}
