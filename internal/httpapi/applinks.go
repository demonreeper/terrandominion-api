package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// Opening a share link in the game (docs/profile-plan.md, P3). Android and iOS each fetch a file
// from /.well-known/ on the link's domain to check that the app may open its links. Both refuse a
// redirected file, so these are served directly, as JSON, with a short cache.

// CertFingerprint tidies a signing certificate's SHA-256 into the form Android wants: 32 bytes in
// upper-case hex with colons ("14:6D:E9:…"). It accepts the same without colons. "" when it isn't one.
func CertFingerprint(typed string) string {
	hex := strings.ToUpper(strings.NewReplacer(":", "", " ", "").Replace(strings.TrimSpace(typed)))
	if len(hex) != 64 {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(hex); i += 2 {
		for _, c := range hex[i : i+2] {
			if !(c >= '0' && c <= '9') && !(c >= 'A' && c <= 'F') {
				return ""
			}
		}
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(hex[i : i+2])
	}
	return b.String()
}

// assetLinks is Android's Digital Asset Links statement: this app, signed with one of these
// certificates, may open this site's links. With no fingerprints it is an empty list, which
// verifies nothing.
func assetLinks(pkg string, fingerprints []string) []map[string]any {
	clean := []string{}
	for _, f := range fingerprints {
		if c := CertFingerprint(f); c != "" {
			clean = append(clean, c)
		}
	}
	if pkg == "" || len(clean) == 0 {
		return []map[string]any{}
	}
	return []map[string]any{{
		"relation": []string{"delegate_permission/common.handle_all_urls"},
		"target": map[string]any{
			"namespace":                "android_app",
			"package_name":             pkg,
			"sha256_cert_fingerprints": clean,
		},
	}}
}

// appleAssociation is iOS's apple-app-site-association: these apps ("<team id>.<bundle id>") open
// profile links, /p/*, and nothing else on the site.
func appleAssociation(appIDs []string) map[string]any {
	ids := []string{}
	for _, id := range appIDs {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	details := []map[string]any{}
	if len(ids) > 0 {
		details = append(details, map[string]any{
			"appIDs":     ids,
			"components": []map[string]any{{"/": "/p/*", "comment": "A player's profile share link"}},
		})
	}
	return map[string]any{"applinks": map[string]any{"details": details}}
}

func (s *Server) assetLinksFile(w http.ResponseWriter, r *http.Request) {
	writeWellKnown(w, assetLinks(s.cfg.AndroidPackage, s.cfg.AndroidCertSHA256))
}

func (s *Server) appleAssociationFile(w http.ResponseWriter, r *http.Request) {
	writeWellKnown(w, appleAssociation(s.cfg.AppleAppIDs))
}

// writeWellKnown writes the bare JSON (no envelope: the phones' verifiers read the file as it is).
func writeWellKnown(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Warn("response write failed", "err", err)
	}
}
