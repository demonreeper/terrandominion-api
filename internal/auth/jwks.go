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
)

// keySet caches a provider's published RSA signing keys (a JWKS document) for as long as its
// Cache-Control allows, and fetches them again when a token names a key it hasn't seen.
type keySet struct {
	name string // for error messages: "google certs", "apple keys"
	url  string
	http *http.Client

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	expires time.Time
}

func newKeySet(name, url string) *keySet {
	return &keySet{name: name, url: url, http: &http.Client{Timeout: 10 * time.Second}}
}

func (k *keySet) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()

	if key, ok := k.keys[kid]; ok && time.Now().Before(k.expires) {
		return key, nil
	}
	if err := k.refresh(ctx); err != nil {
		return nil, err
	}
	if key, ok := k.keys[kid]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("unknown signing key %q", kid)
}

var maxAge = regexp.MustCompile(`max-age=(\d+)`)

func (k *keySet) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, nil)
	if err != nil {
		return err
	}
	res, err := k.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", k.name, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: status %d", k.name, res.StatusCode)
	}

	var body struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return fmt.Errorf("%s: %w", k.name, err)
	}

	keys := make(map[string]*rsa.PublicKey, len(body.Keys))
	for _, key := range body.Keys {
		n, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			continue
		}
		e, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			continue
		}
		keys[key.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
	}
	if len(keys) == 0 {
		return fmt.Errorf("%s: %w", k.name, errors.New("no usable keys"))
	}

	ttl := time.Hour
	if m := maxAge.FindStringSubmatch(res.Header.Get("Cache-Control")); m != nil {
		if secs, err := strconv.Atoi(m[1]); err == nil && secs > 0 {
			ttl = time.Duration(secs) * time.Second
		}
	}
	k.keys = keys
	k.expires = time.Now().Add(ttl)
	return nil
}
