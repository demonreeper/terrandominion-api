package store

import (
	"encoding/json"
	"strings"
	"testing"
)

// Every view that names a player does it with a PlayerRef {code, name}; the counting id and the
// account's real name never appear (docs/profile-plan.md, P0).
func TestViewsNamePlayersByRef(t *testing.T) {
	ref := PlayerRef{Code: "K7QX3M9P", Name: "ECHO-7"}
	cases := []struct {
		name    string
		view    any
		want    string
		without []string
	}{
		{"tether", Tether{ID: 5, OwnerID: 2, Owner: ref},
			`"owner":{"code":"K7QX3M9P","name":"ECHO-7"}`, []string{"ownerId", "ownerName"}},
		{"faction", Faction{Code: "lumen", Leader: &ref},
			`"leader":{"code":"K7QX3M9P","name":"ECHO-7"}`, []string{"leaderName"}},
		{"faction without a commander", Faction{Code: "forge"}, `"leader":null`, []string{"leaderName"}},
		{"notice", Notice{ID: 1, Actor: &ref},
			`"actor":{"code":"K7QX3M9P","name":"ECHO-7"}`, nil},
		{"system notice", Notice{ID: 2}, `"actor":null`, nil},
		{"branch member", BranchMember{PlayerID: 2, Code: ref.Code, Name: ref.Name},
			`"code":"K7QX3M9P","name":"ECHO-7"`, []string{"playerId"}},
	}
	for _, c := range cases {
		raw, err := json.Marshal(c.view)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		got := string(raw)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: %s does not contain %s", c.name, got, c.want)
		}
		for _, key := range c.without {
			if strings.Contains(got, `"`+key+`"`) {
				t.Errorf("%s: %s still has %q", c.name, got, key)
			}
		}
	}
}

// A deleted or banned player is named but not linked: no code to open a profile with.
func TestRefOfDeletedPlayerHasNoCode(t *testing.T) {
	for _, status := range []string{"deleted", "banned"} {
		ref := Player{Code: "K7QX3M9P", Name: "Deleted player", Status: status}.Ref()
		if ref.Code != "" || ref.Name != "Deleted player" {
			t.Errorf("%s: ref = %+v", status, ref)
		}
	}
	if ref := (Player{Code: "K7QX3M9P", Name: "ECHO-7", Status: "active"}).Ref(); ref.Code != "K7QX3M9P" {
		t.Errorf("active: ref = %+v", ref)
	}
}
