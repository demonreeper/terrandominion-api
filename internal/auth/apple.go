package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	appleKeysURL = "https://appleid.apple.com/auth/keys"
	appleIssuer  = "https://appleid.apple.com"
)

// AppleIdentity is what the API keeps from a verified Sign in with Apple identity token. Apple
// never puts the person's name in the token; the app sends it, on the first sign-in only.
type AppleIdentity struct {
	Subject string
	Email   string // empty unless Apple marks it verified; may be a private relay address
}

// AppleVerifier checks Sign in with Apple identity tokens against Apple's published keys. The
// audience is the app's bundle id.
type AppleVerifier struct {
	bundleIDs []string
	keys      *keySet
}

func NewAppleVerifier(bundleIDs []string) *AppleVerifier {
	return &AppleVerifier{bundleIDs: bundleIDs, keys: newKeySet("apple keys", appleKeysURL)}
}

type appleClaims struct {
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"` // Apple sends true or "true"
	Nonce         string `json:"nonce"`
	jwt.RegisteredClaims
}

// HashNonce is what the app hands Apple in the sign-in request: the hex SHA-256 of its raw
// nonce. Apple copies that value into the token; the app sends the raw nonce to the API.
func HashNonce(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Verify checks the token's signature, issuer, audience and expiry, and that it carries the
// hash of rawNonce, so a token lifted from another sign-in can't be replayed here.
func (v *AppleVerifier) Verify(ctx context.Context, identityToken, rawNonce string) (AppleIdentity, error) {
	if len(v.bundleIDs) == 0 {
		return AppleIdentity{}, errors.New("Sign in with Apple is not configured (TD_APPLE_BUNDLE_IDS)")
	}
	if rawNonce == "" {
		return AppleIdentity{}, errors.New("nonce is required")
	}

	var claims appleClaims
	_, err := jwt.ParseWithClaims(identityToken, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return v.keys.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(appleIssuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return AppleIdentity{}, err
	}

	audienceOK := false
	for _, aud := range claims.Audience {
		for _, id := range v.bundleIDs {
			if aud == id {
				audienceOK = true
			}
		}
	}
	if !audienceOK {
		return AppleIdentity{}, errors.New("token was issued for another app")
	}
	if claims.Subject == "" {
		return AppleIdentity{}, errors.New("token has no subject")
	}
	if claims.Nonce != HashNonce(rawNonce) {
		return AppleIdentity{}, fmt.Errorf("nonce does not match")
	}

	email := ""
	if isTrue(claims.EmailVerified) {
		email = claims.Email
	}
	return AppleIdentity{Subject: claims.Subject, Email: email}, nil
}

func isTrue(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return b == "true"
	}
	return false
}
