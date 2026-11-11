package web

import (
	"bytes"
	"strings"
	"testing"
)

func render(t *testing.T, page ProfilePage) string {
	t.Helper()
	var out bytes.Buffer
	if err := RenderProfile(&out, page); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestProfilePageShowsThePublicProfile(t *testing.T) {
	html := render(t, ProfilePage{
		Title: "ECHO-7 // Terran Dominion", Description: "Level 12. Lumen Collective, Warden. 214 nodes harvested.",
		URL: "https://td-dev.demonreeper.xyz/p/K7QX3M9P", Name: "ECHO-7", Initials: "E7", Level: 12,
		Standing: "LUMEN COLLECTIVE // WARDEN", FactionColor: "#4fd8ff",
		Perks:  []string{"+25% CRYSTAL FROM HARVESTS AND TETHER NODES"},
		Badges: []string{"FOUNDER // LUMEN COLLECTIVE"},
		Record: []RecordLine{{"NODES HARVESTED", "214"}, {"TETHER NODES BUILT", "9"}},
	})
	for _, want := range []string{
		`<meta property="og:title" content="ECHO-7 // Terran Dominion">`,
		`<meta property="og:url" content="https://td-dev.demonreeper.xyz/p/K7QX3M9P">`,
		`<meta property="og:description" content="Level 12. Lumen Collective, Warden. 214 nodes harvested.">`,
		"OPERATIVE: ECHO-7", "LVL 12", "LUMEN COLLECTIVE // WARDEN", "NODES HARVESTED", ">214<",
		"FOUNDER // LUMEN COLLECTIVE", "25% CRYSTAL FROM HARVESTS AND TETHER NODES", "OPEN IN TERRAN DOMINION",
		"not in the stores yet",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// No store links were given, so there are no store buttons.
	for _, without := range []string{"GOOGLE PLAY", "APP STORE"} {
		if strings.Contains(html, without) {
			t.Errorf("page should not contain %q", without)
		}
	}
}

func TestProfilePageStoreButtons(t *testing.T) {
	html := render(t, ProfilePage{Name: "ECHO-7", PlayURL: "https://play.google.com/store/apps/details?id=com.hisstudios.terrandominion",
		AppStoreURL: "https://apps.apple.com/app/id0000000000"})
	if !strings.Contains(html, "GET IT ON GOOGLE PLAY") || !strings.Contains(html, "DOWNLOAD ON THE APP STORE") {
		t.Error("both store buttons should show")
	}
	if strings.Contains(html, "not in the stores yet") {
		t.Error("the pre-alpha note should go once a store link is set")
	}
}

// A callsign can't hold markup (the server refuses it), but the page must not trust that.
func TestProfilePageEscapesEverything(t *testing.T) {
	evil := `<script>alert(1)</script>"><img src=x onerror=alert(2)>`
	html := render(t, ProfilePage{
		Title: evil, Description: evil, URL: "javascript:alert(3)", Name: evil, Initials: evil, Standing: evil,
		FactionColor: `red" onmouseover="alert(4)`, Perks: []string{evil}, Badges: []string{evil},
		Record: []RecordLine{{evil, evil}},
	})
	for _, bad := range []string{"<script>alert(1)", "<img src=x", `onmouseover="alert(4)"`, `href="javascript:alert(3)"`} {
		if strings.Contains(html, bad) {
			t.Errorf("page contains unescaped %q", bad)
		}
	}
	if !strings.Contains(html, "&lt;script&gt;") {
		t.Error("the markup should be shown as text")
	}
}

func TestProfilePageGone(t *testing.T) {
	html := render(t, ProfilePage{Gone: true, Title: "Terran Dominion", Name: "SHOULD-NOT-SHOW"})
	if !strings.Contains(html, "This operative is no longer active.") {
		t.Error("the gone page should say so")
	}
	for _, without := range []string{"SHOULD-NOT-SHOW", "SERVICE RECORD", "OPEN IN TERRAN DOMINION"} {
		if strings.Contains(html, without) {
			t.Errorf("the gone page should not contain %q", without)
		}
	}
}
