package httpapi

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/service"
	"github.com/demonreeper/terrandominion/api/web"
)

// profilePage is GET /p/{code}: the web page behind a profile share link, for anyone, signed in or
// not (docs/profile-plan.md, P2). It shows the same public fields as GET /v1/players/{code}, with
// Open Graph tags so chat apps draw a preview. An unknown, deleted or banned player gets the same
// "no longer active" page with 404. Cached for five minutes (by Cloudflare too).
func (s *Server) profilePage(w http.ResponseWriter, r *http.Request) {
	profile, err := s.svc.Profile(r.Context(), 0, r.PathValue("code"))
	page := web.ProfilePage{PlayURL: s.cfg.PlayURL, AppStoreURL: s.cfg.AppStoreURL}
	status := http.StatusOK
	switch {
	case errors.Is(err, apperr.NotFound):
		status = http.StatusNotFound
		page.Gone = true
		page.Title = "Terran Dominion"
		page.Description = "This operative is no longer active."
	case err != nil:
		fail(w, r, err)
		return
	default:
		page = profilePageOf(profile, page)
	}

	var body bytes.Buffer
	if err := web.RenderProfile(&body, page); err != nil {
		fail(w, r, apperr.Internal.With("profile page: %v", err))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'none'")
	w.WriteHeader(status)
	if _, err := w.Write(body.Bytes()); err != nil {
		slog.Warn("response write failed", "err", err)
	}
}

// profilePageOf lays a public profile out for the page: the same words the game's profile uses.
func profilePageOf(p *service.ProfileView, page web.ProfilePage) web.ProfilePage {
	name := strings.ToUpper(p.Name)
	page.Title = name + " // Terran Dominion"
	page.URL = p.ShareURL
	page.Name = name
	page.Initials = initials(p.Name)
	page.Level = p.Level
	page.Standing = "FREELANCER // UNAFFILIATED"
	standing := "Freelancer"
	if p.Faction != nil {
		page.Standing = strings.ToUpper(p.Faction.Name) + " // " + strings.ToUpper(p.Faction.RankTitle)
		page.FactionColor = safeColor(p.Faction.Color)
		standing = p.Faction.Name + ", " + p.Faction.RankTitle
		for _, perk := range p.Faction.Perks {
			page.Perks = append(page.Perks, perk.Line)
		}
	}
	for _, badge := range p.Badges {
		line := strings.ToUpper(badge.Badge)
		if badge.FactionName != "" {
			line += " // " + strings.ToUpper(badge.FactionName)
		}
		page.Badges = append(page.Badges, line)
	}
	r := p.Record
	page.Record = []web.RecordLine{
		{Label: "ACTIVE SINCE", Value: strings.ToUpper(r.ActiveSince.UTC().Format("Jan 2006"))},
		{Label: "NODES HARVESTED", Value: fmt.Sprint(r.NodesHarvested)},
		{Label: "TETHER NODES BUILT", Value: fmt.Sprint(r.TethersBuilt)},
		{Label: "TETHER NODES ACTIVE NOW", Value: fmt.Sprint(r.TethersActive)},
		{Label: "TETHER NODES DESTROYED", Value: fmt.Sprint(r.TethersDestroyed)},
		{Label: "DRONES INTERCEPTED", Value: fmt.Sprint(r.DronesIntercepted)},
		{Label: "MISSIONS COMPLETED", Value: fmt.Sprint(r.MissionsCompleted)},
	}
	page.Description = fmt.Sprintf("Level %d. %s. %d nodes harvested, %d Tether Nodes built.",
		p.Level, standing, r.NodesHarvested, r.TethersBuilt)
	return page
}

// initials are the two letters on the hexagon: "ECHO-7" is E7, "Nightwalker" is NI.
func initials(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == ' ' || r == '-' || r == '_' })
	switch {
	case len(parts) >= 2:
		return strings.ToUpper(parts[0][:1] + parts[1][:1])
	case len(name) >= 2:
		return strings.ToUpper(name[:2])
	case name == "":
		return "??"
	}
	return strings.ToUpper(name)
}

// safeColor passes a "#rgb" or "#rrggbb" colour and nothing else, since it lands in a style.
func safeColor(color string) string {
	if len(color) != 4 && len(color) != 7 || color[0] != '#' {
		return ""
	}
	for _, c := range color[1:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return ""
		}
	}
	return color
}
