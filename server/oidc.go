package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	defaultOIDCScopes = "openid profile email"

	// How long discovery may take, and how often a failed attempt is retried.
	// The retry interval stops a provider that is down from costing one request
	// per login attempt, while still recovering without a restart.
	oidcDiscoveryTimeout      = 10 * time.Second
	oidcProviderRetryInterval = 30 * time.Second
)

var (
	oidcIssuer       string
	oidcClientID     string
	oidcClientSecret string
	oidcScopes       []string
	oidcRedirectURL  string

	oidcProviderMu     sync.Mutex
	oidcProvider       *oidc.Provider
	oidcProviderErr    error
	oidcLastDiscovered time.Time

	errOIDCDisabled    = errors.New("OIDC is not configured")
	errOIDCUnavailable = errors.New("OIDC provider is unavailable")
)

// oidcEnabled reports whether enough of the client configuration is present to
// offer a provider login. Both halves are required, so a half-configured
// deployment does not expose a login button that cannot work.
func oidcEnabled() bool {
	return oidcIssuer != "" && oidcClientID != ""
}

// initOIDCConfig reads the ART_OIDC_* variables.
func initOIDCConfig() {
	oidcIssuer = strings.TrimRight(os.Getenv("ART_OIDC_ISSUER"), "/")
	oidcClientID = os.Getenv("ART_OIDC_CLIENT_ID")
	oidcClientSecret = os.Getenv("ART_OIDC_CLIENT_SECRET")
	oidcRedirectURL = os.Getenv("ART_OIDC_REDIRECT_URL")

	scopes := os.Getenv("ART_OIDC_SCOPES")
	if scopes == "" {
		scopes = defaultOIDCScopes
	}
	oidcScopes = strings.Fields(scopes)

	if !oidcEnabled() {
		if oidcIssuer != "" || oidcClientID != "" {
			log.Printf("OIDC disabled: ART_OIDC_ISSUER and ART_OIDC_CLIENT_ID must both be set")
		}
		return
	}

	// Completing a provider login means minting a session token, so without a
	// signing key there is no way for a login to succeed. Fail loudly instead of
	// serving a login button that always errors.
	if !sessionAuthEnabled() {
		log.Fatalf("ART_OIDC_ISSUER/ART_OIDC_CLIENT_ID are set but ART_SESSION_SECRET is not: OIDC login needs a signing key to issue session tokens")
	}

	log.Printf("OIDC enabled (issuer %s, client %s, scopes %s)", oidcIssuer, oidcClientID, strings.Join(oidcScopes, " "))
	if oidcClientSecret == "" {
		log.Printf("OIDC: ART_OIDC_CLIENT_SECRET is unset, relying on PKCE alone")
	}
}

// getOIDCProvider returns the provider's discovery document, fetching it on
// first use. Failures are retried at most once per oidcProviderRetryInterval, so
// a server that starts while the provider is down keeps serving public routes
// and can still log users in once it comes back.
func getOIDCProvider() (*oidc.Provider, error) {
	oidcProviderMu.Lock()
	defer oidcProviderMu.Unlock()

	if !oidcEnabled() {
		return nil, errOIDCDisabled
	}
	if oidcProvider != nil {
		return oidcProvider, nil
	}
	if !oidcLastDiscovered.IsZero() && time.Since(oidcLastDiscovered) < oidcProviderRetryInterval {
		return nil, oidcProviderErr
	}

	// Discovery is deliberately detached from the request that triggered it: a
	// browser giving up should not abandon the fetch we need for the next login.
	ctx, cancel := context.WithTimeout(context.Background(), oidcDiscoveryTimeout)
	defer cancel()

	oidcLastDiscovered = time.Now()
	provider, err := oidc.NewProvider(ctx, oidcIssuer)
	if err != nil {
		oidcProviderErr = fmt.Errorf("%w: %v", errOIDCUnavailable, err)
		log.Printf("OIDC discovery failed for %s: %v", oidcIssuer, err)
		return nil, oidcProviderErr
	}

	oidcProvider = provider
	oidcProviderErr = nil
	log.Printf("OIDC discovery succeeded for %s", oidcIssuer)

	return provider, nil
}

// requestIsHTTPS reports whether the browser reached us over TLS. This server
// runs behind a reverse proxy in practice, so X-Forwarded-Proto is trusted; the
// container itself only ever speaks HTTP.
func requestIsHTTPS(r *http.Request) bool {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		// Proxies may append rather than replace, so only the first value counts.
		return strings.EqualFold(strings.TrimSpace(strings.Split(proto, ",")[0]), "https")
	}
	return r.TLS != nil
}

// oidcCallbackURL is the redirect_uri sent to the provider. It has to match one
// registered there, so ART_OIDC_REDIRECT_URL wins when set; otherwise it is
// derived from the request so a proxied deployment works unconfigured.
func oidcCallbackURL(r *http.Request) string {
	if oidcRedirectURL != "" {
		return oidcRedirectURL
	}

	scheme := "http"
	if requestIsHTTPS(r) {
		scheme = "https"
	}

	host := r.Host
	if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
		host = strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}

	return fmt.Sprintf("%s://%s/api/auth/callback", scheme, host)
}

// oidcOAuthConfig builds the OAuth2 client for a single login/callback round
// trip.
func oidcOAuthConfig(provider *oidc.Provider, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     oidcClientID,
		ClientSecret: oidcClientSecret,
		RedirectURL:  redirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       oidcScopes,
	}
}

// randomURLToken returns a random URL-safe token, used for the login state and
// the OIDC nonce. Both must be unguessable: the state is what proves the
// callback belongs to a login this browser started.
func randomURLToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
