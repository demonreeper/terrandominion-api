package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/demonreeper/terrandominion/api/internal/config"
)

const sampleCert = "14:6D:E9:83:C5:73:06:50:D8:EE:B9:95:2F:34:FC:64:16:A0:83:42:E6:1D:BE:A8:8A:04:96:B2:3F:CF:44:E5"

func TestCertFingerprint(t *testing.T) {
	bare := strings.ReplaceAll(sampleCert, ":", "")
	for typed, want := range map[string]string{
		sampleCert:                         sampleCert,
		strings.ToLower(sampleCert):        sampleCert,
		"  " + bare + " ":                  sampleCert,
		bare[:62]:                          "",
		"":                                 "",
		strings.Replace(bare, "1", "G", 1): "",
	} {
		if got := CertFingerprint(typed); got != want {
			t.Errorf("CertFingerprint(%q) = %q, want %q", typed, got, want)
		}
	}
}

func TestAssetLinks(t *testing.T) {
	if got := assetLinks("com.hisstudios.terrandominion", nil); len(got) != 0 {
		t.Errorf("no fingerprints: %v", got)
	}
	if got := assetLinks("com.hisstudios.terrandominion", []string{"not a fingerprint"}); len(got) != 0 {
		t.Errorf("bad fingerprint: %v", got)
	}
	got := assetLinks("com.hisstudios.terrandominion", []string{strings.ToLower(sampleCert), "junk"})
	raw, _ := json.Marshal(got)
	want := `[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"com.hisstudios.terrandominion","sha256_cert_fingerprints":["` + sampleCert + `"]}}]`
	if string(raw) != want {
		t.Errorf("asset links\n got %s\nwant %s", raw, want)
	}
}

func TestAppleAssociation(t *testing.T) {
	raw, _ := json.Marshal(appleAssociation([]string{" J2P3QBTV3J.com.hisstudios.terrandominion ", ""}))
	want := `{"applinks":{"details":[{"appIDs":["J2P3QBTV3J.com.hisstudios.terrandominion"],"components":[{"/":"/p/*","comment":"A player's profile share link"}]}]}}`
	if string(raw) != want {
		t.Errorf("association\n got %s\nwant %s", raw, want)
	}
	raw, _ = json.Marshal(appleAssociation(nil))
	if string(raw) != `{"applinks":{"details":[]}}` {
		t.Errorf("empty association %s", raw)
	}
}

// The phones' verifiers want the file itself: 200, JSON, no redirect, no envelope.
func TestWellKnownFilesAreServedBare(t *testing.T) {
	s := &Server{cfg: config.Config{
		AndroidPackage: "com.hisstudios.terrandominion", AndroidCertSHA256: []string{sampleCert},
		AppleAppIDs: []string{"J2P3QBTV3J.com.hisstudios.terrandominion"},
	}}
	for path, handler := range map[string]http.HandlerFunc{
		"/.well-known/assetlinks.json":            s.assetLinksFile,
		"/.well-known/apple-app-site-association": s.appleAssociationFile,
	} {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("%s: status %d, type %q", path, rec.Code, rec.Header().Get("Content-Type"))
		}
		body := rec.Body.String()
		if strings.Contains(body, `"data"`) || !json.Valid([]byte(body)) {
			t.Errorf("%s: body %s", path, body)
		}
	}
}
