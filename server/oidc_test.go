package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	fakeIDPClientID     = "test-client"
	fakeIDPClientSecret = "test-client-secret"
)

// fakeIDP is a minimal OpenID provider: discovery, JWKS and a token endpoint
// that always succeeds. It exists so the whole login flow can be exercised
// without a real provider, Docker or network access.
type fakeIDP struct {
	server *httptest.Server
	key    *rsa.PrivateKey
	kid    string

	mu sync.Mutex
	// nonceToIssue stands in for the provider echoing the nonce back: the test
	// copies it out of the authorize URL before driving the callback.
	nonceToIssue string
	subject      string
	// tokenForm records what the token endpoint received, so tests can prove
	// PKCE and the client credentials were sent.
	tokenForm url.Values
}

func newFakeIDP(t *testing.T) *fakeIDP {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating provider key: %v", err)
	}

	idp := &fakeIDP{key: key, kid: "test-key", subject: "user-1"}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                                idp.issuer(),
			"authorization_endpoint":                idp.issuer() + "/authorize",
			"token_endpoint":                        idp.issuer() + "/token",
			"jwks_uri":                              idp.issuer() + "/keys",
			"userinfo_endpoint":                     idp.issuer() + "/userinfo",
			"response_types_supported":              []string{"code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})

	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA",
				"kid": idp.kid,
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(idp.key.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(idp.key.PublicKey.E)).Bytes()),
			}},
		})
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()

		idp.mu.Lock()
		idp.tokenForm = r.PostForm
		nonce, subject := idp.nonceToIssue, idp.subject
		idp.mu.Unlock()

		if nonce == "" {
			http.Error(w, "the test did not arm a nonce", http.StatusInternalServerError)
			return
		}

		idToken, err := idp.signIDToken(subject, nonce)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// The access token is deliberately opaque: nothing here should ever need
		// to inspect it, because API calls carry a token we signed ourselves.
		writeJSON(w, map[string]any{
			"access_token": "opaque-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
			"id_token":     idToken,
		})
	})

	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)

	return idp
}

func (f *fakeIDP) issuer() string { return f.server.URL }

// armNonce tells the token endpoint which nonce to embed, which in a real flow
// comes from the authorize request the browser carried to the provider.
func (f *fakeIDP) armNonce(nonce string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nonceToIssue = nonce
}

func (f *fakeIDP) lastTokenForm() url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tokenForm
}

func (f *fakeIDP) signIDToken(subject, nonce string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":                f.issuer(),
		"aud":                fakeIDPClientID,
		"sub":                subject,
		"iat":                now.Unix(),
		"exp":                now.Add(time.Hour).Unix(),
		"nonce":              nonce,
		"email":              subject + "@example.com",
		"name":               "Test User",
		"preferred_username": subject,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = f.kid

	return token.SignedString(f.key)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

// startLogin runs the login handler and returns the provider URL the browser
// would be sent to, plus the state cookie the browser would store.
func startLogin(t *testing.T) (*url.URL, *http.Cookie) {
	t.Helper()

	recorder := httptest.NewRecorder()
	oidcLoginHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/login", nil))

	if recorder.Code != http.StatusFound {
		t.Fatalf("login status = %d, want 302 (body %s)", recorder.Code, recorder.Body.String())
	}

	authorizeURL, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parsing authorize URL: %v", err)
	}

	return authorizeURL, findCookie(t, recorder, oidcStateCookieName)
}

func findCookie(t *testing.T, recorder *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("response did not set cookie %q", name)
	return nil
}

func stateCookieCleared(recorder *httptest.ResponseRecorder) bool {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == oidcStateCookieName && cookie.MaxAge < 0 {
			return true
		}
	}
	return false
}

// callbackRequest builds the request the provider's redirect would produce.
func callbackRequest(query string, cookie *http.Cookie) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/api/auth/callback?"+query, nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	return request
}

func TestOIDCLoginRedirectsToProvider(t *testing.T) {
	idp := newFakeIDP(t)
	withAuthConfig(t, "static-token", testSecret)
	withOIDCConfig(t, idp.issuer(), fakeIDPClientID, fakeIDPClientSecret)

	authorizeURL, cookie := startLogin(t)

	if authorizeURL.Path != "/authorize" {
		t.Errorf("login redirected to %q, want the provider's authorize endpoint", authorizeURL.Path)
	}

	query := authorizeURL.Query()
	if query.Get("response_type") != "code" {
		t.Errorf("response_type = %q, want code", query.Get("response_type"))
	}
	if query.Get("client_id") != fakeIDPClientID {
		t.Errorf("client_id = %q, want %q", query.Get("client_id"), fakeIDPClientID)
	}
	if query.Get("state") == "" {
		t.Error("the authorize request carried no state")
	}
	if query.Get("nonce") == "" {
		t.Error("the authorize request carried no nonce")
	}
	if query.Get("code_challenge") == "" || query.Get("code_challenge_method") != "S256" {
		t.Errorf("PKCE challenge missing: %q / %q", query.Get("code_challenge"), query.Get("code_challenge_method"))
	}
	if !strings.HasSuffix(query.Get("redirect_uri"), "/api/auth/callback") {
		t.Errorf("redirect_uri = %q, want it to point at /api/auth/callback", query.Get("redirect_uri"))
	}

	// The cookie is signed rather than plain JSON, and is not readable by script.
	if !strings.Contains(cookie.Value, ".") {
		t.Errorf("state cookie %q does not look signed", cookie.Value)
	}
	if !cookie.HttpOnly {
		t.Error("state cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("state cookie SameSite = %v, want Lax (the provider redirects back top-level)", cookie.SameSite)
	}
}

// The whole point of the flow: the token the callback mints is what the API
// accepts, and the provider's own access token is not a credential at all.
func TestOIDCCallbackIssuesUsableSessionToken(t *testing.T) {
	idp := newFakeIDP(t)
	withAuthConfig(t, "", testSecret)
	withOIDCConfig(t, idp.issuer(), fakeIDPClientID, fakeIDPClientSecret)

	authorizeURL, cookie := startLogin(t)
	state := authorizeURL.Query().Get("state")
	idp.armNonce(authorizeURL.Query().Get("nonce"))

	recorder := httptest.NewRecorder()
	oidcCallbackHandler(recorder, callbackRequest("code=test-code&state="+url.QueryEscape(state), cookie))

	if recorder.Code != http.StatusFound {
		t.Fatalf("callback status = %d, want 302 (body %s)", recorder.Code, recorder.Body.String())
	}

	location := recorder.Header().Get("Location")
	if !strings.HasPrefix(location, "/#token=") {
		t.Fatalf("callback redirected to %q, want a /#token= fragment", location)
	}
	sessionToken := strings.TrimPrefix(location, "/#token=")

	if !stateCookieCleared(recorder) {
		t.Error("the state cookie was not cleared after the callback")
	}

	// The exchange must have sent the PKCE verifier and the client credentials.
	form := idp.lastTokenForm()
	if form.Get("code_verifier") == "" {
		t.Error("the token exchange did not send code_verifier")
	}
	if form.Get("code") != "test-code" {
		t.Errorf("code = %q, want test-code", form.Get("code"))
	}

	claims, err := verifySessionToken(sessionToken)
	if err != nil {
		t.Fatalf("the minted session token does not verify: %v", err)
	}
	if claims.Via != "oidc" {
		t.Errorf("Via = %q, want oidc", claims.Via)
	}
	if claims.Subject != "user-1" {
		t.Errorf("Subject = %q, want user-1", claims.Subject)
	}
	if claims.Email != "user-1@example.com" || claims.Name != "Test User" {
		t.Errorf("display claims = %q / %q", claims.Email, claims.Name)
	}

	if got := requireTokenStatus(t, sessionToken); got != http.StatusNoContent {
		t.Errorf("the session token was rejected by the API (status %d)", got)
	}

	// The provider's opaque access token must not authenticate anything.
	if got := requireTokenStatus(t, "opaque-access-token"); got != http.StatusUnauthorized {
		t.Errorf("the provider access token was accepted (status %d)", got)
	}
}

func TestOIDCCallbackRejectsBadState(t *testing.T) {
	idp := newFakeIDP(t)
	withAuthConfig(t, "", testSecret)
	withOIDCConfig(t, idp.issuer(), fakeIDPClientID, fakeIDPClientSecret)

	// A signed cookie with an expiry in the past.
	expiredPayload, err := json.Marshal(oidcState{
		State:    "state",
		Nonce:    "nonce",
		Verifier: "verifier",
		Expires:  time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("encoding state: %v", err)
	}
	expiredCookie := &http.Cookie{Name: oidcStateCookieName, Value: signOIDCState(expiredPayload)}

	tamperedCookie := &http.Cookie{Name: oidcStateCookieName, Value: "bm90LWEtc3RhdGU.aGFzaA"}

	tests := []struct {
		name     string
		cookie   *http.Cookie
		query    string
		wantCode string
	}{
		{name: "no cookie", cookie: nil, query: "code=x&state=y", wantCode: "state_missing"},
		{name: "unsigned cookie", cookie: tamperedCookie, query: "code=x&state=y", wantCode: "state_invalid"},
		{name: "expired state", cookie: expiredCookie, query: "code=x&state=state", wantCode: "state_expired"},
		{name: "no code", cookie: expiredCookie, query: "state=state", wantCode: "state_expired"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			oidcCallbackHandler(recorder, callbackRequest(test.query, test.cookie))

			if recorder.Code != http.StatusFound {
				t.Fatalf("status = %d, want 302", recorder.Code)
			}
			if got := recorder.Header().Get("Location"); got != "/#auth_error="+test.wantCode {
				t.Errorf("redirect = %q, want /#auth_error=%s", got, test.wantCode)
			}
		})
	}
}

func TestOIDCCallbackRejectsStateMismatch(t *testing.T) {
	idp := newFakeIDP(t)
	withAuthConfig(t, "", testSecret)
	withOIDCConfig(t, idp.issuer(), fakeIDPClientID, fakeIDPClientSecret)

	_, cookie := startLogin(t)

	recorder := httptest.NewRecorder()
	oidcCallbackHandler(recorder, callbackRequest("code=test-code&state=not-the-state", cookie))

	if got := recorder.Header().Get("Location"); got != "/#auth_error=state_mismatch" {
		t.Errorf("redirect = %q, want /#auth_error=state_mismatch", got)
	}
}

// A replayed nonce means the id_token was not issued for this login.
func TestOIDCCallbackRejectsNonceMismatch(t *testing.T) {
	idp := newFakeIDP(t)
	withAuthConfig(t, "", testSecret)
	withOIDCConfig(t, idp.issuer(), fakeIDPClientID, fakeIDPClientSecret)

	authorizeURL, cookie := startLogin(t)
	idp.armNonce("a-different-nonce")

	recorder := httptest.NewRecorder()
	oidcCallbackHandler(recorder, callbackRequest(
		"code=test-code&state="+url.QueryEscape(authorizeURL.Query().Get("state")), cookie))

	if got := recorder.Header().Get("Location"); got != "/#auth_error=nonce_mismatch" {
		t.Errorf("redirect = %q, want /#auth_error=nonce_mismatch", got)
	}
}

func TestOIDCCallbackReportsProviderError(t *testing.T) {
	idp := newFakeIDP(t)
	withAuthConfig(t, "", testSecret)
	withOIDCConfig(t, idp.issuer(), fakeIDPClientID, fakeIDPClientSecret)

	authorizeURL, cookie := startLogin(t)

	recorder := httptest.NewRecorder()
	oidcCallbackHandler(recorder, callbackRequest(
		"error=access_denied&state="+url.QueryEscape(authorizeURL.Query().Get("state")), cookie))

	if got := recorder.Header().Get("Location"); got != "/#auth_error=provider_error" {
		t.Errorf("redirect = %q, want /#auth_error=provider_error", got)
	}
}

func TestOIDCLoginUnavailableWithoutConfiguration(t *testing.T) {
	withAuthConfig(t, "static-token", testSecret)
	withOIDCConfig(t, "", "", "")

	recorder := httptest.NewRecorder()
	oidcLoginHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/login", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	oidcCallbackHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/callback?code=x", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("callback status = %d, want 503", recorder.Code)
	}
}

// The state cookie is the only thing tying a callback to the login this browser
// started, so a payload that was not signed with the session secret must fail.
func TestOIDCStateCookieSignature(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	payload := []byte(`{"state":"s","nonce":"n","verifier":"v","exp":1}`)
	signed := signOIDCState(payload)

	decoded, err := verifyOIDCState(signed)
	if err != nil {
		t.Fatalf("verifyOIDCState: %v", err)
	}
	if string(decoded) != string(payload) {
		t.Errorf("round trip = %q, want %q", decoded, payload)
	}

	state, err := parseOIDCState(signed)
	if err != nil {
		t.Fatalf("parseOIDCState: %v", err)
	}
	if state.State != "s" || state.Nonce != "n" || state.Verifier != "v" {
		t.Errorf("parsed state = %+v", state)
	}

	// Re-signing the same payload with a different key must not validate, which
	// is also what makes a secret rotation invalidate in-flight logins.
	sessionSecret = []byte("ffffffffffffffffffffffffffffffff")
	if _, err := parseOIDCState(signed); err == nil {
		t.Fatal("a state cookie signed with another key was accepted")
	}
}

func TestParseOIDCStateRejectsIncompletePayloads(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	tests := []struct {
		name string
		json string
	}{
		{name: "not JSON", json: "nope"},
		{name: "missing state", json: `{"nonce":"n","verifier":"v"}`},
		{name: "missing nonce", json: `{"state":"s","verifier":"v"}`},
		{name: "missing verifier", json: `{"state":"s","nonce":"n"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseOIDCState(signOIDCState([]byte(test.json))); err == nil {
				t.Errorf("parseOIDCState accepted %s", test.json)
			}
		})
	}
}

func TestOIDCCallbackURLDerivation(t *testing.T) {
	withOIDCConfig(t, "https://idp.example.com", fakeIDPClientID, fakeIDPClientSecret)

	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			name: "plain http",
			want: "http://example.com/api/auth/callback",
		},
		{
			name:    "behind a proxy",
			headers: map[string]string{"X-Forwarded-Proto": "https", "X-Forwarded-Host": "files.example.com"},
			want:    "https://files.example.com/api/auth/callback",
		},
		{
			name:    "first forwarded value wins",
			headers: map[string]string{"X-Forwarded-Proto": "https, http"},
			want:    "https://example.com/api/auth/callback",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
			for key, value := range test.headers {
				request.Header.Set(key, value)
			}

			if got := oidcCallbackURL(request); got != test.want {
				t.Errorf("oidcCallbackURL = %q, want %q", got, test.want)
			}
		})
	}

	// An explicit redirect URL always wins: it has to match what the provider
	// has registered.
	oidcRedirectURL = "https://files.example.com/api/auth/callback"
	if got := oidcCallbackURL(httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)); got != oidcRedirectURL {
		t.Errorf("oidcCallbackURL = %q, want the configured %q", got, oidcRedirectURL)
	}
}

func TestInitOIDCConfigFromEnvironment(t *testing.T) {
	withAuthConfig(t, "", testSecret)
	// Seeded empty so the cleanup restores the real globals afterwards.
	withOIDCConfig(t, "", "", "")

	t.Run("nothing set", func(t *testing.T) {
		initOIDCConfig()
		if oidcEnabled() {
			t.Error("OIDC enabled with no configuration")
		}
	})

	t.Run("only the issuer", func(t *testing.T) {
		t.Setenv("ART_OIDC_ISSUER", "https://idp.example.com")
		t.Setenv("ART_OIDC_CLIENT_ID", "")

		initOIDCConfig()
		if oidcEnabled() {
			t.Error("OIDC enabled with a half-configured client")
		}
	})

	t.Run("issuer and client id", func(t *testing.T) {
		t.Setenv("ART_OIDC_ISSUER", "https://idp.example.com/")
		t.Setenv("ART_OIDC_CLIENT_ID", fakeIDPClientID)
		t.Setenv("ART_OIDC_CLIENT_SECRET", fakeIDPClientSecret)
		t.Setenv("ART_OIDC_SCOPES", "")
		t.Setenv("ART_OIDC_REDIRECT_URL", "")

		// No group bindings, so the groups scope is not worth asking for.
		withRolesConfig(t, "", "")
		initOIDCConfig()

		if !oidcEnabled() {
			t.Fatal("OIDC not enabled with an issuer and client id")
		}
		if oidcIssuer != "https://idp.example.com" {
			t.Errorf("issuer = %q, want the trailing slash trimmed", oidcIssuer)
		}
		if strings.Join(oidcScopes, " ") != defaultOIDCScopes {
			t.Errorf("scopes = %v, want the default %q", oidcScopes, defaultOIDCScopes)
		}
	})

	// Group membership is only worth requesting when something uses it: a provider
	// that does not recognise the scope rejects the whole authorization request.
	t.Run("groups scope follows the bindings", func(t *testing.T) {
		t.Setenv("ART_OIDC_ISSUER", "https://idp.example.com")
		t.Setenv("ART_OIDC_CLIENT_ID", fakeIDPClientID)
		t.Setenv("ART_OIDC_SCOPES", "")

		withRolesConfig(t, "", `{"admins":["admin"]}`)
		initOIDCConfig()

		want := defaultOIDCScopes + " groups"
		if strings.Join(oidcScopes, " ") != want {
			t.Errorf("scopes = %v, want %q", oidcScopes, want)
		}
	})

	// An operator who names the scopes explicitly keeps control of them.
	t.Run("groups scope is not duplicated", func(t *testing.T) {
		t.Setenv("ART_OIDC_ISSUER", "https://idp.example.com")
		t.Setenv("ART_OIDC_CLIENT_ID", fakeIDPClientID)
		t.Setenv("ART_OIDC_SCOPES", "openid groups")

		withRolesConfig(t, "", `{"admins":["admin"]}`)
		initOIDCConfig()

		if strings.Join(oidcScopes, " ") != "openid groups" {
			t.Errorf("scopes = %v, want [openid groups]", oidcScopes)
		}
	})
}

// Discovery is lazy: a provider that is unreachable must not stop the server
// from serving public routes, and login reports 503 rather than panicking.
func TestOIDCProviderUnavailable(t *testing.T) {
	withAuthConfig(t, "", testSecret)
	// A port nothing is listening on.
	withOIDCConfig(t, "http://127.0.0.1:1", fakeIDPClientID, fakeIDPClientSecret)

	recorder := httptest.NewRecorder()
	oidcLoginHandler(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/login", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", recorder.Code)
	}

	// The failed attempt is remembered, so a second call is not another
	// discovery request against a provider that is already known to be down.
	if _, err := getOIDCProvider(); err == nil {
		t.Error("getOIDCProvider succeeded against an unreachable provider")
	}
}
