package httpapi

import (
	"net/http"
	"strconv"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/service"
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

// adminFactions lists the factions for an admin to look inside.
func (s *Server) adminFactions(w http.ResponseWriter, r *http.Request) {
	factions, err := s.svc.AdminFactions(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"factions": factions})
}

// adminFaction is an admin's look inside one faction without joining it.
func (s *Server) adminFaction(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	inside, err := s.svc.AdminFaction(r.Context(), playerID, r.PathValue("code"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, inside)
}

// adminReleasePledge returns a member to freelancer (a mistaken pledge).
func (s *Server) adminReleasePledge(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Reason string `json:"reason"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	info, err := s.svc.AdminReleasePledge(r.Context(), playerID, r.PathValue("code"), body.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, info)
}

// adminMark finds a device mark by its appeal code.
func (s *Server) adminMark(w http.ResponseWriter, r *http.Request) {
	mark, err := s.svc.AdminMark(r.Context(), r.PathValue("code"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, mark)
}

// adminClearMark removes a device mark after an appeal.
func (s *Server) adminClearMark(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Reason string `json:"reason"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.AdminClearMark(r.Context(), playerID, r.PathValue("code"), body.Reason); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "cleared")
}

// jobs is the JOBS board: open jobs for a freelancer who may take them, posted jobs for a Warden.
func (s *Server) jobs(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	view, err := s.svc.Jobs(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, view)
}

func (s *Server) postJob(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Kind      string           `json:"kind"`
		TetherID  int64            `json:"tetherId"`
		NodeID    int64            `json:"nodeId"`
		Lat       float64          `json:"lat"`
		Lon       float64          `json:"lon"`
		AccuracyM float64          `json:"accuracyM"`
		Reward    map[string]int64 `json:"reward"`
		Slots     int              `json:"slots"`
		Hours     int              `json:"hours"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	job, err := s.svc.PostJob(r.Context(), playerID, service.PostJob{
		Kind: body.Kind, TetherID: body.TetherID, NodeID: body.NodeID, Lat: body.Lat, Lon: body.Lon,
		AccuracyM: body.AccuracyM, Reward: body.Reward, Slots: body.Slots, Hours: body.Hours,
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, job)
}

func (s *Server) takeJob(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.NotFound)
		return
	}
	job, err := s.svc.TakeJob(r.Context(), playerID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, job)
}
