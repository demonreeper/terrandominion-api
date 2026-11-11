package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/demonreeper/terrandominion/api/internal/config"
)

// /v1/me carries the share code, the player-facing name and the share link, never the real name.
func TestMeViewHasNoRealName(t *testing.T) {
	callsign := "ECHO-7"
	raw, err := json.Marshal(MeView{ID: 2, Code: "K7QX3M9P", Name: callsign, Callsign: &callsign,
		ShareURL: "https://td-dev.demonreeper.xyz/p/K7QX3M9P"})
	if err != nil {
		t.Fatal(err)
	}
	got := string(raw)
	for _, want := range []string{`"code":"K7QX3M9P"`, `"name":"ECHO-7"`, `"callsign":"ECHO-7"`,
		`"shareUrl":"https://td-dev.demonreeper.xyz/p/K7QX3M9P"`} {
		if !strings.Contains(got, want) {
			t.Errorf("%s does not contain %s", got, want)
		}
	}
	if strings.Contains(got, "displayName") {
		t.Errorf("%s still has displayName", got)
	}

	// Until a callsign is picked it is null, so the app knows to ask.
	raw, _ = json.Marshal(MeView{Code: "K7QX3M9P", Name: "OPERATIVE-K7QX3M9P"})
	if !strings.Contains(string(raw), `"callsign":null`) {
		t.Errorf("%s: callsign should be null", raw)
	}
}

func TestShareURL(t *testing.T) {
	s := &Service{cfg: config.Config{PublicURL: "https://td-dev.demonreeper.xyz"}}
	if got := s.ShareURL("K7QX3M9P"); got != "https://td-dev.demonreeper.xyz/p/K7QX3M9P" {
		t.Errorf("ShareURL = %q", got)
	}
}
