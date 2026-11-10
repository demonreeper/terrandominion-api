package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testBundle = "com.hisstudios.terrandominion"

// fakeApple serves one RSA key as a JWKS document and signs tokens with it.
func fakeApple(t *testing.T) (*AppleVerifier, func(jwt.MapClaims) string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=600")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	}))
	t.Cleanup(srv.Close)

	v := NewAppleVerifier([]string{testBundle})
	v.keys = newKeySet("apple keys", srv.URL)
	sign := func(c jwt.MapClaims) string {
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
		tok.Header["kid"] = "k1"
		s, err := tok.SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	return v, sign
}

func goodClaims(nonce string) jwt.MapClaims {
	now := time.Now()
	return jwt.MapClaims{
		"iss": appleIssuer, "aud": testBundle, "sub": "001234.abcdef.0001",
		"iat": now.Unix(), "exp": now.Add(10 * time.Minute).Unix(),
		"nonce": HashNonce(nonce), "email": "x@privaterelay.appleid.com", "email_verified": "true",
	}
}

func TestAppleVerifyAccepts(t *testing.T) {
	v, sign := fakeApple(t)
	id, err := v.Verify(context.Background(), sign(goodClaims("raw-nonce")), "raw-nonce")
	if err != nil {
		t.Fatal(err)
	}
	if id.Subject != "001234.abcdef.0001" || id.Email != "x@privaterelay.appleid.com" {
		t.Fatalf("got %+v", id)
	}
}

func TestAppleVerifyEmailVerifiedBool(t *testing.T) {
	v, sign := fakeApple(t)
	c := goodClaims("n")
	c["email_verified"] = false
	id, err := v.Verify(context.Background(), sign(c), "n")
	if err != nil {
		t.Fatal(err)
	}
	if id.Email != "" {
		t.Fatalf("unverified email kept: %q", id.Email)
	}
}

func TestAppleVerifyRejects(t *testing.T) {
	cases := map[string]struct {
		change func(jwt.MapClaims)
		nonce  string
		want   string
	}{
		"wrong nonce":    {func(c jwt.MapClaims) {}, "other", "nonce"},
		"missing nonce":  {func(c jwt.MapClaims) {}, "", "nonce is required"},
		"raw nonce sent": {func(c jwt.MapClaims) { c["nonce"] = "raw-nonce" }, "raw-nonce", "nonce"},
		"wrong audience": {func(c jwt.MapClaims) { c["aud"] = "com.other.app" }, "raw-nonce", "another app"},
		"wrong issuer":   {func(c jwt.MapClaims) { c["iss"] = "https://accounts.google.com" }, "raw-nonce", "issuer"},
		"expired":        {func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, "raw-nonce", "expired"},
		"no subject":     {func(c jwt.MapClaims) { delete(c, "sub") }, "raw-nonce", "subject"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			v, sign := fakeApple(t)
			c := goodClaims("raw-nonce")
			tc.change(c)
			_, err := v.Verify(context.Background(), sign(c), tc.nonce)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestAppleVerifyNotConfigured(t *testing.T) {
	v := NewAppleVerifier(nil)
	if _, err := v.Verify(context.Background(), "x", "n"); err == nil || !strings.Contains(err.Error(), "TD_APPLE_BUNDLE_IDS") {
		t.Fatalf("got %v", err)
	}
}
