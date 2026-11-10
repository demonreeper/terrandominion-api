package push

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"unicode/utf8"
)

func testClient(t *testing.T, send http.HandlerFunc, tokens *atomic.Int32) *Client {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		tokens.Add(1)
		json.NewEncoder(w).Encode(map[string]string{"access_token": "abc"})
	})
	mux.HandleFunc("/send", send)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	raw, _ := json.Marshal(map[string]string{
		"client_email": "x@y.z",
		"private_key":  string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"token_uri":    server.URL + "/token",
	})
	file := filepath.Join(t.TempDir(), "sa.json")
	if err := os.WriteFile(file, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := Load("proj", file)
	if err != nil {
		t.Fatal(err)
	}
	client.sendURL = server.URL + "/send"
	return client
}

func TestSendShapeAndTokenReuse(t *testing.T) {
	var tokens atomic.Int32
	var got wire
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer abc" {
			t.Errorf("auth header: %q", r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&got)
	}, &tokens)

	m := Message{Token: "phone", Title: "T", Body: "B", Channel: "tn_alerts", High: true, Group: "tn-5", Data: map[string]string{"tnId": "5"}}
	for i := 0; i < 2; i++ {
		if result, err := client.Send(context.Background(), m); result != Sent || err != nil {
			t.Fatalf("send: %v %v", result, err)
		}
	}
	if tokens.Load() != 1 {
		t.Errorf("access token fetched %d times", tokens.Load())
	}
	a := got.Message.Android
	if got.Message.Token != "phone" || a.Priority != "HIGH" || a.CollapseKey != "tn-5" || a.Notification.ChannelID != "tn_alerts" || a.Notification.Tag != "tn-5" {
		t.Errorf("wire: %+v", got)
	}
	i := got.Message.APNs
	if i.Headers["apns-priority"] != "10" || i.Headers["apns-collapse-id"] != "tn-5" ||
		i.Payload.Aps.Sound != "default" || i.Payload.Aps.ThreadID != "tn-5" {
		t.Errorf("apns: %+v", i)
	}
}

func TestAPNsNormalPriorityAndLimits(t *testing.T) {
	w := wireMessage(Message{Token: "p", Title: "T", Body: "B", Channel: "c"})
	if w.Message.APNs.Headers["apns-priority"] != "5" {
		t.Errorf("priority: %v", w.Message.APNs.Headers)
	}
	if _, ok := w.Message.APNs.Headers["apns-collapse-id"]; ok {
		t.Errorf("collapse id without a group: %v", w.Message.APNs.Headers)
	}
	raw, _ := json.Marshal(w)
	if strings.Contains(string(raw), "thread-id") {
		t.Errorf("thread-id without a group: %s", raw)
	}

	long := strings.Repeat("é", 40) // 80 bytes
	w = wireMessage(Message{Token: "p", Group: long})
	id := w.Message.APNs.Headers["apns-collapse-id"]
	if len(id) > apnsCollapseLimit || !utf8.ValidString(id) || len(id) != 64 {
		t.Errorf("collapse id %d bytes, valid %v", len(id), utf8.ValidString(id))
	}
}

func TestSendFailures(t *testing.T) {
	cases := []struct {
		status int
		body   string
		want   Result
	}{
		{404, `{"error":{"status":"NOT_FOUND","details":[{"errorCode":"UNREGISTERED"}]}}`, BadToken},
		{400, `{"error":{"status":"INVALID_ARGUMENT"}}`, BadToken},
		{429, `{"error":{"status":"RESOURCE_EXHAUSTED"}}`, Retry},
		{503, `{}`, Retry},
		{401, `{"error":{"status":"UNAUTHENTICATED"}}`, Retry},
		{403, `{"error":{"status":"PERMISSION_DENIED"}}`, Failed},
	}
	for _, c := range cases {
		var tokens atomic.Int32
		client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(c.status)
			w.Write([]byte(c.body))
		}, &tokens)
		result, err := client.Send(context.Background(), Message{Token: "secret-phone-token"})
		if result != c.want {
			t.Errorf("%d: got %v, want %v", c.status, result, c.want)
		}
		if err == nil || strings.Contains(err.Error(), "secret-phone-token") {
			t.Errorf("%d: error %v", c.status, err)
		}
	}
}

func TestLoadRejectsBadFiles(t *testing.T) {
	if _, err := Load("", "x"); err == nil {
		t.Error("empty project accepted")
	}
	file := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(file, []byte(`{"client_email":"a","private_key":"nope"}`), 0o600)
	if _, err := Load("p", file); err == nil || strings.Contains(err.Error(), "nope") {
		t.Errorf("bad key: %v", err)
	}
}
