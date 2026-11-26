package mcpoauth

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/login.html
var loginFS embed.FS

var loginTmpl = template.Must(template.ParseFS(loginFS, "templates/login.html"))

// LoginView is the data for the OAuth login HTML form.
type LoginView struct {
	ClientID            string
	ClientName          string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Resource            string
	Error               string
}

func renderLogin(w http.ResponseWriter, status int, view LoginView) {
	var buf bytes.Buffer
	if err := loginTmpl.Execute(&buf, view); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":             "server_error",
			"error_description": err.Error(),
		})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

func authorize(st *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		clientID := q.Get("client_id")
		redirectURI := q.Get("redirect_uri")
		scope := q.Get("scope")
		stateParam := q.Get("state")
		codeChallenge := q.Get("code_challenge")
		method := q.Get("code_challenge_method")
		resource := q.Get("resource")
		if method == "" {
			method = "S256"
		}
		if scope == "" {
			scope = ScopeNFSRead + " " + ScopeNFSWrite
		}

		client, err := st.Store.FindClient(clientID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":             "invalid_client",
				"error_description": "unknown client_id",
			})
			return
		}
		if !client.AllowsRedirect(redirectURI) {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":             "invalid_request",
				"error_description": "redirect_uri does not match the registered client",
			})
			return
		}

		renderLogin(w, http.StatusOK, LoginView{
			ClientID:            clientID,
			ClientName:          client.ClientName,
			RedirectURI:         redirectURI,
			Scope:               scope,
			State:               stateParam,
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: method,
			Resource:            resource,
		})
	}
}
