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

// A public profile carries only what anyone may see. This is the whole list of its fields: adding
// one means deciding it is public.
func TestProfileViewHasOnlyPublicFields(t *testing.T) {
	raw, err := json.Marshal(ProfileView{Code: "K7QX3M9P", Name: "ECHO-7", Level: 12,
		Faction: &ProfileFaction{Code: "lumen", Name: "Lumen Collective", Color: "#4fd8ff", Rank: 50, RankTitle: "Warden"}})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"code": true, "name": true, "level": true, "faction": true, "badges": true,
		"record": true, "isMe": true, "shareUrl": true}
	for key := range fields {
		if !allowed[key] {
			t.Errorf("profile has a field that is not on the public list: %q", key)
		}
	}
	var record map[string]json.RawMessage
	if err := json.Unmarshal(fields["record"], &record); err != nil {
		t.Fatal(err)
	}
	publicRecord := map[string]bool{"activeSince": true, "nodesHarvested": true, "tethersBuilt": true,
		"tethersActive": true, "tethersDestroyed": true, "dronesIntercepted": true, "missionsCompleted": true}
	for key := range record {
		if !publicRecord[key] {
			t.Errorf("service record has a field that is not on the public list: %q", key)
		}
	}
	for _, private := range []string{"displayName", "email", "xp", "carried", "stored", "gear", "items", "linked",
		"lastSeen", "lat", "lon", "id"} {
		if strings.Contains(string(raw), `"`+private+`"`) {
			t.Errorf("profile leaks %q: %s", private, raw)
		}
	}
}
