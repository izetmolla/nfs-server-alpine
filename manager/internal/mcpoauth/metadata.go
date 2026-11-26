package mcpoauth

import (
	"encoding/json"
	"net/http"
)

type authorizationServerMetadata struct {
	Issuer                            string   `json:"issuer,omitempty"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	RevocationEndpoint                string   `json:"revocation_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
}

type protectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
	ResourceName           string   `json:"resource_name,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func mcpScopes() []string {
	return []string{ScopeNFSRead, ScopeNFSWrite}
}

func authorizationServer(st *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := requestBaseURL(st, r)
		writeJSON(w, http.StatusOK, authorizationServerMetadata{
			Issuer:                            base,
			AuthorizationEndpoint:             base + "/oauth/authorize",
			TokenEndpoint:                     base + "/oauth/token",
			RegistrationEndpoint:              base + "/oauth/register",
			RevocationEndpoint:                base + "/oauth/revoke",
			ScopesSupported:                   mcpScopes(),
			ResponseTypesSupported:            []string{"code"},
			CodeChallengeMethodsSupported:     []string{"S256"},
			GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
			TokenEndpointAuthMethodsSupported: []string{"client_secret_post", "none"},
		})
	}
}

func protectedResource(st *State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		base := requestBaseURL(st, r)
		writeJSON(w, http.StatusOK, protectedResourceMetadata{
			Resource:               st.MCPResourceURL(base),
			AuthorizationServers:   []string{base},
			ScopesSupported:        mcpScopes(),
			BearerMethodsSupported: []string{"header"},
			ResourceName:           "NFS Manager MCP",
		})
	}
}
