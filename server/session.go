package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	// Stamped into every token this server mints, so a deployment that happens
	// to share its secret with another service still rejects that service's
	// tokens (and vice versa).
	sessionIssuer   = "artifact-server"
	sessionAudience = "artifact-api"

	// A short HMAC key can be brute-forced offline, and this secret is also the
	// only way to invalidate outstanding tokens (there is no revocation list),
	// so a weak one is refused instead of warned about.
	minSessionSecretLength = 32

	// How long a token minted by the OIDC login stays valid. Overridable via
	// ART_SESSION_TTL.
	defaultSessionTTL = 12 * time.Hour
)

var (
	// sessionSecret is the HS256 key behind both OIDC sessions and tokens minted
	// by the `token` command. Empty means ART_SESSION_SECRET is unset, which
	// disables session tokens entirely.
	sessionSecret []byte

	// sessionTTL is the lifetime given to tokens minted by the OIDC login.
	sessionTTL = defaultSessionTTL

	errSessionAuthDisabled = errors.New("session tokens are not configured")
	errInvalidSessionToken = errors.New("invalid session token")
)

// sessionIdentity is who a session token belongs to. Only Subject is
// authoritative - it comes from the id_token's sub. The rest is display
// metadata that must never feed an authorization decision.
type sessionIdentity struct {
	Subject           string
	Email             string
	Name              string
	PreferredUsername string
}

// sessionClaims is the payload of a session token. Everything needed to
// authorize a request lives here, so verifying one never touches the network.
type sessionClaims struct {
	jwt.RegisteredClaims

	// Via records how the token was obtained: "oidc" for a provider login,
	// "mint" for the CLI. The UI uses it to tell a session apart from a pasted
	// static token when it reports an auth failure.
	Via               string `json:"via,omitempty"`
	Email             string `json:"email,omitempty"`
	Name              string `json:"name,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
}

// sessionAuthEnabled reports whether a signing key is configured.
func sessionAuthEnabled() bool {
	return len(sessionSecret) > 0
}

// sessionTTLLabel describes the configured session lifetime for logs.
func sessionTTLLabel() string {
	if sessionTTL <= 0 {
		return "no expiry"
	}
	return sessionTTL.String()
}

// initSessionConfig reads ART_SESSION_SECRET and ART_SESSION_TTL.
func initSessionConfig() {
	if secret := os.Getenv("ART_SESSION_SECRET"); secret != "" {
		if len(secret) < minSessionSecretLength {
			log.Fatalf("ART_SESSION_SECRET must be at least %d characters (got %d)", minSessionSecretLength, len(secret))
		}
		sessionSecret = []byte(secret)
	}

	// An unparseable TTL is a config mistake, not a reason to refuse to serve,
	// so warn and keep the default - matching how parseSize/parseInt behave.
	if raw := os.Getenv("ART_SESSION_TTL"); raw != "" {
		ttl, err := parseDuration(raw)
		if err != nil {
			log.Printf("Ignoring invalid ART_SESSION_TTL %q (%v); using %s", raw, err, defaultSessionTTL)
		} else {
			sessionTTL = ttl
		}
	}

	if sessionAuthEnabled() {
		log.Printf("Session token auth enabled (TTL %s)", sessionTTLLabel())
	}
}

// mintSessionToken signs a session token. A ttl of zero or less omits the exp
// claim, producing a token that never expires (what `token -ttl 0` asks for).
// now is a parameter so tests don't depend on the wall clock.
func mintSessionToken(via string, id sessionIdentity, ttl time.Duration, now time.Time) (string, error) {
	if !sessionAuthEnabled() {
		return "", errSessionAuthDisabled
	}

	claims := sessionClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:   sessionIssuer,
			Subject:  id.Subject,
			Audience: jwt.ClaimStrings{sessionAudience},
			IssuedAt: jwt.NewNumericDate(now),
		},
		Via:               via,
		Email:             id.Email,
		Name:              id.Name,
		PreferredUsername: id.PreferredUsername,
	}
	if ttl > 0 {
		claims.ExpiresAt = jwt.NewNumericDate(now.Add(ttl))
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(sessionSecret)
}

// verifySessionToken checks a token's signature, issuer, audience and (when it
// carries one) expiry. WithValidMethods pins the algorithm, so an "alg: none"
// or asymmetrically-signed token can't be smuggled past the keyfunc. A token
// without an exp claim is accepted - only `-ttl 0` mints those.
func verifySessionToken(raw string) (*sessionClaims, error) {
	return verifySessionTokenAt(raw, time.Now())
}

// verifySessionTokenAt is verifySessionToken with the clock injected, so a test
// that mints a token at a fixed instant can verify it at that same instant
// instead of racing the wall clock (an exp claim is only in the future relative
// to the clock it was minted against).
func verifySessionTokenAt(raw string, now time.Time) (*sessionClaims, error) {
	if !sessionAuthEnabled() {
		return nil, errSessionAuthDisabled
	}

	claims := &sessionClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (interface{}, error) {
		return sessionSecret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(sessionIssuer),
		jwt.WithAudience(sessionAudience),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		// Wrapped so callers can test the outcome without matching jwt/v5's
		// error strings.
		return nil, fmt.Errorf("%w: %v", errInvalidSessionToken, err)
	}
	if !token.Valid {
		return nil, errInvalidSessionToken
	}

	return claims, nil
}
