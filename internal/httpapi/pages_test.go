package httpapi

import (
	"testing"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/service"
	"github.com/demonreeper/terrandominion/api/internal/store"
	"github.com/demonreeper/terrandominion/api/web"
)

func TestProfilePageOf(t *testing.T) {
	page := profilePageOf(&service.ProfileView{
		Code: "K7QX3M9P", Name: "Echo-7", Level: 12, ShareURL: "https://td-dev.demonreeper.xyz/p/K7QX3M9P",
		Faction: &service.ProfileFaction{Name: "Lumen Collective", Color: "#4fd8ff", RankTitle: "Warden",
			Perks: []store.PerkLine{{Key: "crystal", Line: "+25% CRYSTAL FROM HARVESTS AND TETHER NODES"}}},
		Badges: []store.Badge{{Badge: "founder", FactionName: "Lumen Collective", Region: "MV"}},
		Record: store.ServiceRecord{ActiveSince: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), NodesHarvested: 214, TethersBuilt: 9},
	}, web.ProfilePage{})

	if page.Title != "ECHO-7 // Tether Rise" || page.Name != "ECHO-7" || page.Initials != "E7" {
		t.Errorf("title %q name %q initials %q", page.Title, page.Name, page.Initials)
	}
	if page.Standing != "LUMEN COLLECTIVE // WARDEN" || page.FactionColor != "#4fd8ff" {
		t.Errorf("standing %q colour %q", page.Standing, page.FactionColor)
	}
	if page.Description != "Level 12. Lumen Collective, Warden. 214 nodes harvested, 9 Tether Nodes built." {
		t.Errorf("description %q", page.Description)
	}
	// The badge names no region: player-facing text stays country-neutral.
	if len(page.Badges) != 1 || page.Badges[0] != "FOUNDER // LUMEN COLLECTIVE" {
		t.Errorf("badges %v", page.Badges)
	}
	if page.Record[0].Label != "ACTIVE SINCE" || page.Record[0].Value != "NOV 2026" {
		t.Errorf("active since %+v", page.Record[0])
	}

	freelancer := profilePageOf(&service.ProfileView{Name: "Nightwalker", Level: 3}, web.ProfilePage{})
	if freelancer.Standing != "FREELANCER // UNAFFILIATED" || freelancer.FactionColor != "" || freelancer.Initials != "NI" {
		t.Errorf("freelancer page %+v", freelancer)
	}
}

func TestSafeColor(t *testing.T) {
	for _, ok := range []string{"#4fd8ff", "#FFB648", "#abc"} {
		if safeColor(ok) != ok {
			t.Errorf("safeColor(%q) should pass", ok)
		}
	}
	for _, bad := range []string{"", "red", "4fd8ff", "#4fd8f", "#4fd8fg", `#fff" onmouseover="x`, "#4fd8ff;background:url(x)"} {
		if safeColor(bad) != "" {
			t.Errorf("safeColor(%q) should be refused", bad)
		}
	}
}
