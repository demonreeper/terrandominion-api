package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims carried by the game's access token. Short-lived; the session row behind it is checked
// again on every refresh, so a ban or sign-out takes effect within one access-token lifetime.
type Claims struct {
	SessionID int64 `json:"sid"`
	Admin     bool  `json:"adm,omitempty"`
	jwt.RegisteredClaims
}

func (c Claims) PlayerID() (int64, error) {
	return strconv.ParseInt(c.Subject, 10, 64)
}

type Tokens struct {
	secret []byte
	issuer string
}

func NewTokens(secret []byte, env string) *Tokens {
	return &Tokens{secret: secret, issuer: "terrandominion-" + env}
}

func (t *Tokens) IssueAccess(playerID, sessionID int64, admin bool, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	expires := now.Add(ttl)
	claims := Claims{
		SessionID: sessionID,
		Admin:     admin,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.issuer,
			Subject:   strconv.FormatInt(playerID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	return signed, expires, err
}

func (t *Tokens) ParseAccess(token string) (Claims, error) {
	var claims Claims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(t.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return Claims{}, err
	}
	if claims.Subject == "" || claims.SessionID == 0 {
		return Claims{}, errors.New("incomplete token")
	}
	return claims, nil
}

// NewRefreshToken returns an opaque random token for the client and the hash the database keeps.
func NewRefreshToken() (token string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, HashRefreshToken(token), nil
}

func HashRefreshToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// HashDevice turns the client's device id into the value stored in the database. The pepper
// means a leaked table cannot be matched back to real device ids.
func HashDevice(deviceID string, pepper []byte) []byte {
	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte(deviceID))
	return mac.Sum(nil)
}
