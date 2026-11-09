// Package auth verifies Google sign-in and issues the game's own tokens.
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"sync"
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
	http      *http.Client

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

func NewGoogleVerifier(clientIDs []string) *GoogleVerifier {
	return &GoogleVerifier{clientIDs: clientIDs, http: &http.Client{Timeout: 10 * time.Second}}
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
		return v.key(ctx, kid)
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

func (v *GoogleVerifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if key, ok := v.keys[kid]; ok && time.Now().Before(v.expires) {
		return key, nil
	}
	if err := v.refresh(ctx); err != nil {
		return nil, err
	}
	if key, ok := v.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("unknown signing key %q", kid)
}

var maxAge = regexp.MustCompile(`max-age=(\d+)`)

func (v *GoogleVerifier) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleCertsURL, nil)
	if err != nil {
		return err
	}
	res, err := v.http.Do(req)
	if err != nil {
		return fmt.Errorf("google certs: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("google certs: status %d", res.StatusCode)
	}

	var body struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return fmt.Errorf("google certs: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(body.Keys))
	for _, k := range body.Keys {
		n, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		e, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return errors.New("google certs: no usable keys")
	}

	ttl := time.Hour
	if m := maxAge.FindStringSubmatch(res.Header.Get("Cache-Control")); m != nil {
		if secs, err := strconv.Atoi(m[1]); err == nil && secs > 0 {
			ttl = time.Duration(secs) * time.Second
		}
	}
	v.keys = keys
	v.expires = time.Now().Add(ttl)
	return nil
}
