// Package auth verifies Google and Apple sign-in and issues the game's own tokens.
package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const googleCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

var googleIssuers = []string{"accounts.google.com", "https://accounts.google.com"}

// GoogleIdentity is what the API keeps from a verified Google ID token.
type GoogleIdentity struct {
	Subject string
	Email   string
	Name    string
	Nonce   string
}

// GoogleVerifier checks Google ID tokens against Google's published signing keys, which it
// caches for as long as Google's Cache-Control allows. It uses only the JWT library the API
// already needs, rather than Google's large client SDK.
type GoogleVerifier struct {
	clientIDs []string
	keys      *keySet
}

func NewGoogleVerifier(clientIDs []string) *GoogleVerifier {
	return &GoogleVerifier{clientIDs: clientIDs, keys: newKeySet("google certs", googleCertsURL)}
}

type googleClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Nonce         string `json:"nonce"`
	jwt.RegisteredClaims
}

func (v *GoogleVerifier) Verify(ctx context.Context, idToken string) (GoogleIdentity, error) {
	var claims googleClaims
	_, err := jwt.ParseWithClaims(idToken, &claims, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		return v.keys.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return GoogleIdentity{}, err
	}

	issuerOK := false
	for _, iss := range googleIssuers {
		if claims.Issuer == iss {
			issuerOK = true
		}
	}
	if !issuerOK {
		return GoogleIdentity{}, fmt.Errorf("unexpected issuer %q", claims.Issuer)
	}

	audienceOK := false
	for _, aud := range claims.Audience {
		for _, id := range v.clientIDs {
			if aud == id {
				audienceOK = true
			}
		}
	}
	if !audienceOK {
		return GoogleIdentity{}, errors.New("token was issued for another app")
	}
	if claims.Subject == "" {
		return GoogleIdentity{}, errors.New("token has no subject")
	}

	email := ""
	if claims.EmailVerified {
		email = claims.Email
	}
	return GoogleIdentity{Subject: claims.Subject, Email: email, Name: claims.Name, Nonce: claims.Nonce}, nil
}
