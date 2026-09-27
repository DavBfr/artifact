package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestMintAndVerifySessionToken(t *testing.T) {
	withAuthConfig(t, "", testSecret)
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

	raw, err := mintSessionToken("oidc", sessionIdentity{
		Subject:           "user-1",
		Email:             "user@example.com",
		Name:              "Test User",
		PreferredUsername: "tester",
	}, time.Hour, now)
	if err != nil {
		t.Fatalf("mintSessionToken: %v", err)
	}

	claims, err := verifySessionToken(raw)
	if err != nil {
		t.Fatalf("verifySessionToken: %v", err)
	}

	if claims.Subject != "user-1" {
		t.Errorf("Subject = %q, want %q", claims.Subject, "user-1")
	}
	if claims.Issuer != sessionIssuer {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, sessionIssuer)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != sessionAudience {
		t.Errorf("Audience = %v, want [%s]", claims.Audience, sessionAudience)
	}
	if claims.Via != "oidc" {
		t.Errorf("Via = %q, want %q", claims.Via, "oidc")
	}
	if claims.Email != "user@example.com" || claims.Name != "Test User" || claims.PreferredUsername != "tester" {
		t.Errorf("display claims = %q/%q/%q", claims.Email, claims.Name, claims.PreferredUsername)
	}
	if claims.ExpiresAt == nil || !claims.ExpiresAt.Time.Equal(now.Add(time.Hour)) {
		t.Errorf("ExpiresAt = %v, want %v", claims.ExpiresAt, now.Add(time.Hour))
	}
}

func TestVerifySessionTokenRejectsExpiredToken(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	raw, err := mintSessionToken("mint", sessionIdentity{Subject: "ci"}, time.Minute, time.Now().Add(-2*time.Minute))
	if err != nil {
		t.Fatalf("mintSessionToken: %v", err)
	}

	if _, err := verifySessionToken(raw); err == nil {
		t.Fatal("an expired session token was accepted")
	}
}

// A "-ttl 0" token carries no exp claim at all, so it must keep verifying no
// matter how much time passes.
func TestMintSessionTokenWithoutTTLNeverExpires(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	raw, err := mintSessionToken("mint", sessionIdentity{Subject: "ci"}, 0, time.Now().Add(-10_000*time.Hour))
	if err != nil {
		t.Fatalf("mintSessionToken: %v", err)
	}

	claims, err := verifySessionToken(raw)
	if err != nil {
		t.Fatalf("verifySessionToken: %v", err)
	}
	if claims.ExpiresAt != nil {
		t.Errorf("ExpiresAt = %v, want no exp claim", claims.ExpiresAt)
	}
}

func TestVerifySessionTokenRejectsForeignSecret(t *testing.T) {
	withAuthConfig(t, "", testSecret)
	raw := mintTestToken(t, time.Hour)

	sessionSecret = []byte("ffffffffffffffffffffffffffffffff")

	if _, err := verifySessionToken(raw); err == nil {
		t.Fatal("a token signed with a different secret was accepted")
	}
}

// WithValidMethods is what keeps an HMAC-signed token from being re-signed with
// another algorithm (or with "alg: none") and slipped past the keyfunc.
func TestVerifySessionTokenPinsTheAlgorithm(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	claims := jwt.MapClaims{
		"iss": sessionIssuer,
		"aud": sessionAudience,
		"sub": "ci",
		"exp": time.Now().Add(time.Hour).Unix(),
	}

	hs512, err := jwt.NewWithClaims(jwt.SigningMethodHS512, claims).SignedString(sessionSecret)
	if err != nil {
		t.Fatalf("signing HS512 token: %v", err)
	}

	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("signing alg=none token: %v", err)
	}

	for _, test := range []struct {
		name  string
		token string
	}{
		{"HS512", hs512},
		{"alg none", none},
		{"garbage", "not-a-jwt"},
		{"empty", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := verifySessionToken(test.token); err == nil {
				t.Fatalf("a %s token was accepted", test.name)
			}
		})
	}
}

func TestVerifySessionTokenRejectsTamperedPayload(t *testing.T) {
	withAuthConfig(t, "", testSecret)
	raw := mintTestToken(t, time.Hour)

	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d segments, want 3", len(parts))
	}

	// Flip a character in the payload; the signature no longer matches.
	payload := []byte(parts[1])
	if payload[0] == 'A' {
		payload[0] = 'B'
	} else {
		payload[0] = 'A'
	}
	tampered := parts[0] + "." + string(payload) + "." + parts[2]

	if _, err := verifySessionToken(tampered); err == nil {
		t.Fatal("a tampered token was accepted")
	}
}

// Tokens must be rejected outright when the issuer or audience does not match,
// so the secret can safely be shared with another service.
func TestVerifySessionTokenRejectsWrongIssuerOrAudience(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	for _, test := range []struct {
		name   string
		issuer string
		aud    string
	}{
		{"wrong issuer", "some-other-service", sessionAudience},
		{"wrong audience", sessionIssuer, "some-other-api"},
	} {
		t.Run(test.name, func(t *testing.T) {
			claims := jwt.MapClaims{
				"iss": test.issuer,
				"aud": test.aud,
				"sub": "ci",
				"exp": time.Now().Add(time.Hour).Unix(),
			}

			raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(sessionSecret)
			if err != nil {
				t.Fatalf("signing token: %v", err)
			}

			if _, err := verifySessionToken(raw); err == nil {
				t.Fatalf("a token with the %s was accepted", test.name)
			}
		})
	}
}

func TestSessionTokensDisabledWithoutSecret(t *testing.T) {
	withAuthConfig(t, "static-token", "")

	if sessionAuthEnabled() {
		t.Fatal("sessionAuthEnabled() = true with no ART_SESSION_SECRET")
	}

	if _, err := mintSessionToken("mint", sessionIdentity{Subject: "ci"}, time.Hour, time.Now()); !errors.Is(err, errSessionAuthDisabled) {
		t.Errorf("mintSessionToken error = %v, want errSessionAuthDisabled", err)
	}

	if _, err := verifySessionToken(mintTestTokenPlaceholder()); !errors.Is(err, errSessionAuthDisabled) {
		t.Errorf("verifySessionToken error = %v, want errSessionAuthDisabled", err)
	}
}

// mintTestTokenPlaceholder stands in for "any string at all" in the disabled
// case, where the token is never parsed.
func mintTestTokenPlaceholder() string {
	return "any.thing.at-all"
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "hours", input: "12h", want: 12 * time.Hour},
		{name: "minutes", input: "90m", want: 90 * time.Minute},
		{name: "days", input: "30d", want: 720 * time.Hour},
		{name: "fractional days", input: "0.5d", want: 12 * time.Hour},
		{name: "zero means no expiry", input: "0", want: 0},
		{name: "surrounding whitespace", input: "  6h ", want: 6 * time.Hour},

		{name: "empty", input: "", wantErr: true},
		{name: "word", input: "soon", wantErr: true},
		{name: "negative", input: "-1h", wantErr: true},
		{name: "negative days", input: "-2d", wantErr: true},
		{name: "bare unit", input: "h", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseDuration(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("parseDuration(%q) = %v, want an error", test.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDuration(%q): %v", test.input, err)
			}
			if got != test.want {
				t.Errorf("parseDuration(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestSessionTTLLabel(t *testing.T) {
	withAuthConfig(t, "", testSecret)

	sessionTTL = 12 * time.Hour
	if got := sessionTTLLabel(); got != "12h0m0s" {
		t.Errorf("sessionTTLLabel() = %q", got)
	}

	sessionTTL = 0
	if got := sessionTTLLabel(); got != "no expiry" {
		t.Errorf("sessionTTLLabel() with no expiry = %q", got)
	}
}
