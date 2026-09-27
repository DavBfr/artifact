package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// handlerStatus runs a handler with an optional credential and reports the
// status code.
func handlerStatus(handler http.HandlerFunc, token string) int {
	recorder := httptest.NewRecorder()
	handler(recorder, bearerRequest(token))

	return recorder.Code
}

// Both credentials can be configured at once, and each is accepted on its own.
func TestRequireTokenAcceptsEitherCredential(t *testing.T) {
	withAuthConfig(t, "static-token", testSecret)
	sessionToken := mintTestToken(t, time.Hour)

	tests := []struct {
		name  string
		token string
		want  int
	}{
		{name: "static token", token: "static-token", want: http.StatusNoContent},
		{name: "session token", token: sessionToken, want: http.StatusNoContent},
		{name: "wrong static token", token: "static-toke", want: http.StatusUnauthorized},
		{name: "expired session token", token: "expired", want: http.StatusUnauthorized},
		{name: "no credential", token: "", want: http.StatusUnauthorized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			token := test.token
			if token == "expired" {
				expired, err := mintSessionToken("mint", sessionIdentity{Subject: "ci"}, nil, time.Minute, time.Now().Add(-time.Hour))
				if err != nil {
					t.Fatalf("minting expired token: %v", err)
				}
				token = expired
			}

			if got := requireTokenStatus(t, token); got != test.want {
				t.Errorf("status = %d, want %d", got, test.want)
			}
		})
	}
}

// ART_SESSION_SECRET on its own is a complete configuration: the server can both
// verify and mint tokens, so uploads and deletes require a credential instead of
// being refused as unconfigurable.
func TestRequireTokenEnforcedWithSessionSecretOnly(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	if !authConfigured() {
		t.Fatal("authConfigured() = false with only ART_SESSION_SECRET set")
	}

	if got := requireTokenStatus(t, mintTestToken(t, time.Hour)); got != http.StatusNoContent {
		t.Errorf("session token status = %d, want 204", got)
	}

	// The refusal must be the ordinary "credential required", not the
	// "nothing is configured" message: the server is very much configured.
	want := "Authentication required. Provide a token in the Authorization header."
	if got := requireTokenError(t, ""); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

// With ART_SESSION_SECRET set, a static token that happens to match nothing must
// not be mistaken for a valid credential.
func TestRequireTokenRejectsStaticTokenWhenOnlySessionSecretIsSet(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	if got := requireTokenStatus(t, "some-random-token"); got != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", got)
	}
}

// The "no API token configured" message must survive, but only for the case it
// actually describes: no credential source at all.
func TestRequireTokenWithoutAnyCredentialSource(t *testing.T) {
	withAuthConfig(t, "", "")

	if authConfigured() {
		t.Fatal("authConfigured() = true with nothing configured")
	}

	want := "Access denied. No API token configured on server."
	if got := requireTokenError(t, ""); got != want {
		t.Errorf("error for a missing header = %q, want %q", got, want)
	}
	if got := requireTokenError(t, "anything"); got != want {
		t.Errorf("error for a supplied header = %q, want %q", got, want)
	}
}

// The Authorization header is read as-is apart from an optional "Bearer "
// prefix, which is the shape the README's curl examples use.
func TestRequireTokenHeaderShapes(t *testing.T) {
	withAuthConfig(t, "static-token", "")

	handler := requireToken(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{name: "bearer prefix", header: "Bearer static-token", want: http.StatusNoContent},
		{name: "bare token", header: "static-token", want: http.StatusNoContent},
		{name: "another scheme", header: "Token static-token", want: http.StatusUnauthorized},
		{name: "basic auth", header: "Basic static-token", want: http.StatusUnauthorized},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/upload", nil)
			request.Header.Set("Authorization", test.header)

			recorder := httptest.NewRecorder()
			handler(recorder, request)

			if recorder.Code != test.want {
				t.Errorf("status = %d, want %d", recorder.Code, test.want)
			}
		})
	}
}

func TestMayRequireTokenFollowsNoListing(t *testing.T) {
	withAuthConfig(t, "static-token", "")

	previous := noListing
	t.Cleanup(func() { noListing = previous })

	handler := mayRequireToken(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	noListing = false
	if got := handlerStatus(handler, ""); got != http.StatusNoContent {
		t.Errorf("public listing status = %d, want 204", got)
	}

	noListing = true
	if got := handlerStatus(handler, ""); got != http.StatusUnauthorized {
		t.Errorf("private listing without a credential status = %d, want 401", got)
	}
	if got := handlerStatus(handler, "static-token"); got != http.StatusNoContent {
		t.Errorf("private listing with a credential status = %d, want 204", got)
	}
}

func TestGetConfigHandlerPublicFields(t *testing.T) {
	withAuthConfig(t, "static-token", "")
	withOIDCConfig(t, "", "", "")

	previousLimit := maxListLimit
	previousListing := noListing
	t.Cleanup(func() {
		maxListLimit = previousLimit
		noListing = previousListing
	})
	maxListLimit = 500
	noListing = false

	response, body := decodeConfig(t, "")

	if response.MaxContentLength != 0 {
		t.Errorf("max_content_length = %d without a credential, want it omitted", response.MaxContentLength)
	}
	if response.MaxListLimit != 500 {
		t.Errorf("max_list_limit = %d, want the server cap 500", response.MaxListLimit)
	}
	// OIDC is off, so the field must be absent rather than false: the UI treats
	// its presence as the signal to redirect to a provider.
	if strings.Contains(body, "oidc_enabled") {
		t.Errorf("oidc_enabled appeared in the response while OIDC is disabled: %s", body)
	}
}

func TestGetConfigHandlerWithACredential(t *testing.T) {
	withAuthConfig(t, "static-token", testSecret)

	previousLength := maxContentLength
	t.Cleanup(func() { maxContentLength = previousLength })
	maxContentLength = 42 * 1024 * 1024

	for _, test := range []struct {
		name  string
		token string
	}{
		{name: "static token", token: "static-token"},
		{name: "session token", token: mintTestToken(t, time.Hour)},
	} {
		t.Run(test.name, func(t *testing.T) {
			response, _ := decodeConfig(t, test.token)

			if response.MaxContentLength != 42*1024*1024 {
				t.Errorf("max_content_length = %d, want %d", response.MaxContentLength, 42*1024*1024)
			}
		})
	}
}

func TestGetConfigHandlerRejectsAnInvalidCredential(t *testing.T) {
	withAuthConfig(t, "static-token", testSecret)

	request := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	request.Header.Set("Authorization", "Bearer nope")
	recorder := httptest.NewRecorder()
	getConfigHandler(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", recorder.Code)
	}
}

func TestGetConfigHandlerAdvertisesOIDC(t *testing.T) {
	withAuthConfig(t, "static-token", testSecret)
	withOIDCConfig(t, "https://idp.example.com", "artifact", "secret")

	// No credential: the UI needs to know about OIDC before it can log in.
	response, body := decodeConfig(t, "")
	if !response.OidcEnabled {
		t.Error("oidc_enabled = false while OIDC is configured")
	}
	if !strings.Contains(body, `"oidc_enabled":true`) {
		t.Errorf("response did not advertise OIDC: %s", body)
	}
}
