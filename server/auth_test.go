package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testSecret is long enough to pass the ART_SESSION_SECRET length check.
const testSecret = "0123456789abcdef0123456789abcdef"

// withAuthConfig installs a credential configuration for one test and restores
// the previous one afterwards. Either argument may be empty, which is how the
// "only a static token" / "only a session secret" / "nothing configured" cases
// are expressed.
func withAuthConfig(t *testing.T, staticToken, secret string) {
	t.Helper()

	previousToken, previousSecret := apiToken, sessionSecret
	apiToken = staticToken
	if secret == "" {
		sessionSecret = nil
	} else {
		sessionSecret = []byte(secret)
	}

	t.Cleanup(func() {
		apiToken = previousToken
		sessionSecret = previousSecret
	})
}

// bearerRequest builds a request with the given credential in the Authorization
// header. An empty token leaves the header off entirely.
func bearerRequest(token string) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	return request
}

// requireTokenStatus runs a handler behind requireToken and reports the status
// it produced: 204 when the credential was accepted, 401 otherwise.
func requireTokenStatus(t *testing.T, token string) int {
	t.Helper()

	handler := requireToken(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	handler(recorder, bearerRequest(token))

	return recorder.Code
}

// requireTokenError runs a handler behind requireToken and returns the error
// message from the 401 body, which is how the tests tell the three rejection
// reasons apart.
func requireTokenError(t *testing.T, token string) string {
	t.Helper()

	handler := requireToken(func(http.ResponseWriter, *http.Request) {})

	recorder := httptest.NewRecorder()
	handler(recorder, bearerRequest(token))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}

	var response Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decoding 401 body: %v", err)
	}

	return response.Error
}

// decodeConfig runs getConfigHandler against a request carrying token (empty
// for no credential at all) and returns both the decoded response and the raw
// body, so tests can assert what was omitted.
func decodeConfig(t *testing.T, token string) (ConfigResponse, string) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	getConfigHandler(recorder, request)

	var response ConfigResponse
	if recorder.Code == http.StatusOK {
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decoding config body: %v", err)
		}
	}

	return response, recorder.Body.String()
}

// mintTestToken mints a session token with the standard test subject.
func mintTestToken(t *testing.T, ttl time.Duration) string {
	t.Helper()

	token, err := mintSessionToken("mint", sessionIdentity{Subject: "ci", Email: "ci@example.com"}, ttl, time.Now())
	if err != nil {
		t.Fatalf("minting session token: %v", err)
	}

	return token
}

// withOIDCConfig points the OIDC configuration somewhere for one test, restoring
// the previous values afterwards.
func withOIDCConfig(t *testing.T, issuer, clientID, clientSecret string) {
	t.Helper()

	previousIssuer, previousClientID := oidcIssuer, oidcClientID
	previousSecret, previousRedirect := oidcClientSecret, oidcRedirectURL
	previousScopes := oidcScopes

	oidcIssuer = issuer
	oidcClientID = clientID
	oidcClientSecret = clientSecret
	oidcRedirectURL = ""
	oidcScopes = strings.Fields(defaultOIDCScopes)
	resetOIDCProvider()

	t.Cleanup(func() {
		oidcIssuer = previousIssuer
		oidcClientID = previousClientID
		oidcClientSecret = previousSecret
		oidcRedirectURL = previousRedirect
		oidcScopes = previousScopes
		resetOIDCProvider()
	})
}

// resetOIDCProvider drops the cached discovery document, so a test never talks
// to the previous test's provider.
func resetOIDCProvider() {
	oidcProviderMu.Lock()
	oidcProvider = nil
	oidcProviderErr = nil
	oidcLastDiscovered = time.Time{}
	oidcProviderMu.Unlock()
}
