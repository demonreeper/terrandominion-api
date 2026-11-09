// Package push sends phone notifications through the Firebase Cloud Messaging HTTP v1 API, signed
// with a service account. No Google SDK; nothing here touches game rules or the database.
package push

import (
	"bytes"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	scope          = "https://www.googleapis.com/auth/firebase.messaging"
	defaultToken   = "https://oauth2.googleapis.com/token"
	tokenLifetime  = time.Hour
	tokenReuse     = 55 * time.Minute
	requestTimeout = 10 * time.Second
)

// Result is what became of one message to one phone.
type Result int

const (
	Sent     Result = iota
	BadToken        // the phone is gone; forget the token
	Retry           // quota or a server problem; try again later
	Failed          // rejected for some other reason; retrying won't help
)

// Message is one notification for one phone.
type Message struct {
	Token   string
	Title   string
	Body    string
	Channel string
	High    bool
	// Group makes later messages replace earlier ones with the same value. Optional.
	Group string
	Data  map[string]string
}

// Client talks to FCM for one project.
type Client struct {
	projectID string
	email     string
	key       *rsa.PrivateKey
	tokenURL  string
	sendURL   string
	http      *http.Client

	mu      sync.Mutex
	access  string
	expires time.Time
}

// Load reads a service account JSON file. Its contents never appear in an error.
func Load(projectID, credentialsFile string) (*Client, error) {
	if projectID == "" || credentialsFile == "" {
		return nil, errors.New("project id or credentials file not set")
	}
	raw, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, errors.New("credentials file can't be read")
	}
	var account struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal(raw, &account); err != nil || account.ClientEmail == "" {
		return nil, errors.New("credentials file is not a service account")
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(account.PrivateKey))
	if err != nil {
		return nil, errors.New("credentials file has no usable private key")
	}
	tokenURL := account.TokenURI
	if tokenURL == "" {
		tokenURL = defaultToken
	}
	return &Client{
		projectID: projectID,
		email:     account.ClientEmail,
		key:       key,
		tokenURL:  tokenURL,
		sendURL:   "https://fcm.googleapis.com/v1/projects/" + url.PathEscape(projectID) + "/messages:send",
		http:      &http.Client{Timeout: requestTimeout},
	}, nil
}

// Send delivers one message. The error says why for the log, and never holds the token.
func (c *Client) Send(ctx context.Context, m Message) (Result, error) {
	access, err := c.accessToken(ctx)
	if err != nil {
		return Retry, err
	}
	body, err := json.Marshal(wireMessage(m))
	if err != nil {
		return Failed, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.sendURL, bytes.NewReader(body))
	if err != nil {
		return Failed, err
	}
	req.Header.Set("Authorization", "Bearer "+access)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Retry, errors.New("fcm unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return Sent, nil
	}

	var failure struct {
		Error struct {
			Status  string `json:"status"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&failure)
	code := failure.Error.Status
	for _, detail := range failure.Error.Details {
		if detail.ErrorCode != "" {
			code = detail.ErrorCode
		}
	}
	return classify(resp.StatusCode, code, c), fmt.Errorf("fcm %d %s", resp.StatusCode, code)
}

func classify(status int, code string, c *Client) Result {
	switch {
	case code == "UNREGISTERED" || code == "INVALID_ARGUMENT":
		return BadToken
	case status == http.StatusUnauthorized:
		c.forgetAccess()
		return Retry
	case status == http.StatusTooManyRequests || status >= 500:
		return Retry
	default:
		return Failed
	}
}

type wire struct {
	Message wirePayload `json:"message"`
}

type wirePayload struct {
	Token        string            `json:"token"`
	Notification wireNotification  `json:"notification"`
	Android      wireAndroid       `json:"android"`
	Data         map[string]string `json:"data,omitempty"`
}

type wireNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type wireAndroid struct {
	CollapseKey  string                  `json:"collapse_key,omitempty"`
	Priority     string                  `json:"priority"`
	Notification wireAndroidNotification `json:"notification"`
}

type wireAndroidNotification struct {
	ChannelID string `json:"channel_id"`
	Tag       string `json:"tag,omitempty"`
}

func wireMessage(m Message) wire {
	priority := "NORMAL"
	if m.High {
		priority = "HIGH"
	}
	return wire{Message: wirePayload{
		Token:        m.Token,
		Notification: wireNotification{Title: m.Title, Body: m.Body},
		Android: wireAndroid{
			CollapseKey:  m.Group,
			Priority:     priority,
			Notification: wireAndroidNotification{ChannelID: m.Channel, Tag: m.Group},
		},
		Data: m.Data,
	}}
}

func (c *Client) forgetAccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.access = ""
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if c.access != "" && now.Before(c.expires) {
		return c.access, nil
	}

	assertion, err := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   c.email,
		"scope": scope,
		"aud":   c.tokenURL,
		"iat":   now.Unix(),
		"exp":   now.Add(tokenLifetime).Unix(),
	}).SignedString(c.key)
	if err != nil {
		return "", errors.New("can't sign the token request")
	}

	form := url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"},
		"assertion":  {assertion},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", errors.New("google token service unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google token service answered %d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out); err != nil || out.AccessToken == "" {
		return "", errors.New("google token service sent no token")
	}
	c.access = out.AccessToken
	c.expires = now.Add(tokenReuse)
	return c.access, nil
}
