package httpapi

import (
	"net/http"

	"github.com/demonreeper/terrandominion/api/internal/game"
)

// Admin players and stats (docs/profile-plan.md, A2).

// modPlayers is the staff player search (?q=).
func (s *Server) modPlayers(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	players, err := s.svc.SearchPlayers(r.Context(), playerID, r.URL.Query().Get("q"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"players": players})
}

func (s *Server) modPlayer(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	info, err := s.svc.PlayerInfo(r.Context(), playerID, r.PathValue("code"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, info)
}

func (s *Server) adminAdjustPlayer(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		XP      int64            `json:"xp"`
		Carried map[string]int64 `json:"carried"`
		Stored  map[string]int64 `json:"stored"`
		Reason  string           `json:"reason"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	info, err := s.svc.AdminAdjust(r.Context(), playerID, r.PathValue("code"),
		game.Adjustment{XP: body.XP, Carried: body.Carried, Stored: body.Stored, Reason: body.Reason})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, info)
}

func (s *Server) adminResetCallsign(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Reason string `json:"reason"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	info, err := s.svc.AdminResetCallsign(r.Context(), playerID, r.PathValue("code"), body.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, info)
}

func (s *Server) adminSetMod(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		IsMod bool `json:"isMod"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	info, err := s.svc.AdminSetMod(r.Context(), playerID, r.PathValue("code"), body.IsMod)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, info)
}

func (s *Server) adminStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.svc.AdminStats(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, stats)
}
