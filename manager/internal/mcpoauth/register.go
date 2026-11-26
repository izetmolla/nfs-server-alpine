package mcpoauth

import (
	"crypto/rand"
	"encoding/json"
	"math/big"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/izetmolla/nfs-server-alpine/manager/internal/models"
)

type registerRequest struct {
	ClientName   string   `json:"client_name"`
	RedirectURIs []string `json:"redirect_uris"`
}

func randomSecret(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		v, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		b[i] = alphabet[v.Int64()]
	}
	return string(b)
}

func register(st *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":             "invalid_client_metadata",
				"error_description": "invalid JSON body",
			})
			return
		}
		if req.ClientName == "" {
			req.ClientName = "MCP Client"
		}
		if len(req.RedirectURIs) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":             "invalid_client_metadata",
				"error_description": "at least one redirect_uri is required",
			})
			return
		}

		clientID := "client-" + uuid.NewString()
		clientSecret := randomSecret(32)
		client := models.MCPOAuthClient{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			ClientName:   req.ClientName,
			CreatedAt:    time.Now().UTC(),
		}
		client.SetRedirectURIs(req.RedirectURIs)
		if err := st.Store.CreateClient(&client); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error":             "server_error",
				"error_description": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"client_id":                  clientID,
			"client_secret":              clientSecret,
			"client_name":                req.ClientName,
			"redirect_uris":              req.RedirectURIs,
			"token_endpoint_auth_method": "client_secret_post",
			"grant_types":                []string{"authorization_code", "refresh_token"},
			"response_types":             []string{"code"},
		})
	}
}
