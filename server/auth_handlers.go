package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	// oidcStateCookieName carries the login state through the provider round
	// trip. There is no server-side session store, so the CSRF state, the OIDC
	// nonce and the PKCE verifier all travel in this signed, HttpOnly cookie.
	oidcStateCookieName = "art_oidc_state"
	oidcStateCookieTTL  = 10 * time.Minute
)

var errInvalidOIDCState = errors.New("invalid OIDC state cookie")

// oidcState is the payload of the state cookie.
type oidcState struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	Expires  int64  `json:"exp"`
}

// signOIDCState returns base64(payload).base64(hmac). The HMAC key is
// ART_SESSION_SECRET, which OIDC requires to be set anyway (completing a login
// mints a session token), so the cookie cannot be forged and stops being
// accepted the moment the key is rotated.
func signOIDCState(payload []byte) string {
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write(payload)

	return base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// verifyOIDCState checks the signature and returns the payload.
func verifyOIDCState(raw string) ([]byte, error) {
	encodedPayload, encodedMAC, ok := strings.Cut(raw, ".")
	if !ok {
		return nil, errInvalidOIDCState
	}

	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return nil, errInvalidOIDCState
	}
	signature, err := base64.RawURLEncoding.DecodeString(encodedMAC)
	if err != nil {
		return nil, errInvalidOIDCState
	}

	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return nil, errInvalidOIDCState
	}

	return payload, nil
}

// parseOIDCState verifies and decodes a state cookie value.
func parseOIDCState(raw string) (*oidcState, error) {
	payload, err := verifyOIDCState(raw)
	if err != nil {
		return nil, err
	}

	var state oidcState
	if err := json.Unmarshal(payload, &state); err != nil {
		return nil, errInvalidOIDCState
	}
	if state.State == "" || state.Nonce == "" || state.Verifier == "" {
		return nil, errInvalidOIDCState
	}

	return &state, nil
}

// oidcLoginHandler starts a login: it stores the state, nonce and PKCE verifier
// in a signed cookie and redirects the browser to the provider. The UI login
// button navigates here instead of showing a token form whenever OIDC is on.
func oidcLoginHandler(w http.ResponseWriter, r *http.Request) {
	if !oidcEnabled() {
		http.Error(w, "OIDC is not configured on this server", http.StatusServiceUnavailable)
		return
	}

	provider, err := getOIDCProvider()
	if err != nil {
		log.Printf("OIDC login failed: %v", err)
		http.Error(w, "OIDC provider is unavailable", http.StatusServiceUnavailable)
		return
	}

	state, err := randomURLToken(32)
	if err != nil {
		log.Printf("OIDC login failed to generate state: %v", err)
		http.Error(w, "Failed to start login", http.StatusInternalServerError)
		return
	}
	nonce, err := randomURLToken(32)
	if err != nil {
		log.Printf("OIDC login failed to generate nonce: %v", err)
		http.Error(w, "Failed to start login", http.StatusInternalServerError)
		return
	}

	verifier := oauth2.GenerateVerifier()

	payload, err := json.Marshal(oidcState{
		State:    state,
		Nonce:    nonce,
		Verifier: verifier,
		Expires:  time.Now().Add(oidcStateCookieTTL).Unix(),
	})
	if err != nil {
		log.Printf("OIDC login failed to encode state: %v", err)
		http.Error(w, "Failed to start login", http.StatusInternalServerError)
		return
	}

	setOIDCStateCookie(w, r, signOIDCState(payload))

	conf := oidcOAuthConfig(provider, oidcCallbackURL(r))
	http.Redirect(w, r, conf.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

// oidcCallbackHandler completes a login: it verifies the state cookie, exchanges
// the code, checks the id_token and then mints a session token which the UI
// picks up from the URL fragment. The provider's access token is deliberately
// dropped - this server only trusts tokens it signed itself.
func oidcCallbackHandler(w http.ResponseWriter, r *http.Request) {
	// The state is single-use whichever way this request ends.
	cookie, cookieErr := r.Cookie(oidcStateCookieName)
	clearOIDCStateCookie(w, r)

	if !oidcEnabled() {
		http.Error(w, "OIDC is not configured on this server", http.StatusServiceUnavailable)
		return
	}
	if cookieErr != nil {
		oidcFailRedirect(w, r, "state_missing")
		return
	}

	provider, err := getOIDCProvider()
	if err != nil {
		log.Printf("OIDC callback failed: %v", err)
		oidcFailRedirect(w, r, "provider_unavailable")
		return
	}

	state, err := parseOIDCState(cookie.Value)
	if err != nil {
		oidcFailRedirect(w, r, "state_invalid")
		return
	}
	if time.Now().After(time.Unix(state.Expires, 0)) {
		oidcFailRedirect(w, r, "state_expired")
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(state.State)) != 1 {
		oidcFailRedirect(w, r, "state_mismatch")
		return
	}
	if providerErr := r.URL.Query().Get("error"); providerErr != "" {
		log.Printf("OIDC provider rejected the login: %s %s", providerErr, r.URL.Query().Get("error_description"))
		oidcFailRedirect(w, r, "provider_error")
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		oidcFailRedirect(w, r, "missing_code")
		return
	}

	conf := oidcOAuthConfig(provider, oidcCallbackURL(r))
	token, err := conf.Exchange(r.Context(), code, oauth2.VerifierOption(state.Verifier))
	if err != nil {
		log.Printf("OIDC code exchange failed: %v", err)
		oidcFailRedirect(w, r, "exchange_failed")
		return
	}

	// The id_token is what identifies the user; the access token is not needed
	// for anything, since API calls carry a session token we sign ourselves.
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		oidcFailRedirect(w, r, "missing_id_token")
		return
	}

	idToken, err := provider.Verifier(&oidc.Config{ClientID: oidcClientID}).Verify(r.Context(), rawIDToken)
	if err != nil {
		log.Printf("OIDC id_token verification failed: %v", err)
		oidcFailRedirect(w, r, "invalid_id_token")
		return
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(state.Nonce)) != 1 {
		oidcFailRedirect(w, r, "nonce_mismatch")
		return
	}

	sessionToken, err := mintSessionToken("oidc", oidcIdentityFromIDToken(idToken), sessionTTL, time.Now())
	if err != nil {
		log.Printf("OIDC session token minting failed: %v", err)
		http.Error(w, "Failed to complete login", http.StatusInternalServerError)
		return
	}

	// The fragment is not sent to the server and does not appear in Referer, so
	// the token stays out of logs. It is base64url, which is fragment-safe.
	http.Redirect(w, r, "/#token="+sessionToken, http.StatusFound)
}

// oidcIdentityFromIDToken pulls the display metadata out of an id_token. Only
// the subject is authoritative; the rest exists so the UI can show who is
// signed in, and must never feed an authorization decision.
func oidcIdentityFromIDToken(idToken *oidc.IDToken) sessionIdentity {
	var claims struct {
		Email             string `json:"email"`
		Name              string `json:"name"`
		PreferredUsername string `json:"preferred_username"`
	}
	// A claims decode failure only costs the display name, not the login.
	if err := idToken.Claims(&claims); err != nil {
		log.Printf("OIDC id_token claims could not be decoded: %v", err)
	}

	return sessionIdentity{
		Subject:           idToken.Subject,
		Email:             claims.Email,
		Name:              claims.Name,
		PreferredUsername: claims.PreferredUsername,
	}
}

// oidcFailRedirect sends the browser back to the app with a reason in the URL
// fragment, which the UI turns into a notification.
func oidcFailRedirect(w http.ResponseWriter, r *http.Request, reason string) {
	http.Redirect(w, r, "/#auth_error="+url.QueryEscape(reason), http.StatusFound)
}

// setOIDCStateCookie stores the login state. SameSite=Lax is required rather
// than Strict: the cookie has to come back on the provider's top-level
// redirect. Secure is only set over HTTPS, since the cookie would otherwise be
// dropped in a plain-HTTP deployment - which is why the name carries no
// __Host- prefix.
func setOIDCStateCookie(w http.ResponseWriter, r *http.Request, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(oidcStateCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// clearOIDCStateCookie expires the state cookie.
func clearOIDCStateCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}
