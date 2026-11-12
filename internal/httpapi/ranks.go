package httpapi

import (
	"net/http"
	"strconv"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/game"
)

// Faction ranks v2 and Nomads (docs/faction-ranks-plan.md, R1).

func (s *Server) ranksLadder(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]any{"ranks": game.Ranks})
}

func (s *Server) appointRank(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Code string `json:"code"`
		Rank string `json:"rank"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	targetID, err := s.svc.PlayerIDByCode(r.Context(), body.Code)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.AppointRank(r.Context(), playerID, targetID, body.Rank); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"appointed": body.Rank})
}

func (s *Server) squad(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	out, err := s.svc.Squad(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) squadAdd(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	targetID, err := s.svc.PlayerIDByCode(r.Context(), r.PathValue("code"))
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.SquadAdd(r.Context(), playerID, targetID); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"added": r.PathValue("code")})
}

func (s *Server) squadRemove(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	targetID := playerID
	if code := r.PathValue("code"); code != "me" {
		id, err := s.svc.PlayerIDByCode(r.Context(), code)
		if err != nil {
			fail(w, r, err)
			return
		}
		targetID = id
	}
	if err := s.svc.SquadRemove(r.Context(), playerID, targetID); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"removed": true})
}

func (s *Server) becomeNomad(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	if err := s.svc.BecomeNomad(r.Context(), playerID); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"nomad": true})
}

func (s *Server) settle(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	if err := s.svc.Settle(r.Context(), playerID); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"nomad": false})
}

func (s *Server) nomads(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	out, err := s.svc.Nomads(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) askMove(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Region string `json:"region"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	out, err := s.svc.AskMove(r.Context(), playerID, body.Region)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) cancelMove(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	if err := s.svc.CancelMove(r.Context(), playerID); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"cancelled": true})
}

func (s *Server) answerMove(yes bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, _ := playerFrom(r.Context())
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			fail(w, r, apperr.BadRequest.With("ask id"))
			return
		}
		if err := s.svc.AnswerMove(r.Context(), playerID, id, yes); err != nil {
			fail(w, r, err)
			return
		}
		ok(w, map[string]any{"approved": yes})
	}
}

func (s *Server) adminSetRank(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	var body struct {
		Rank string `json:"rank"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.AdminSetRank(r.Context(), adminID, r.PathValue("code"), body.Rank); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"rank": body.Rank})
}

func (s *Server) adminClearNomadCooldown(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	if err := s.svc.AdminClearNomadCooldown(r.Context(), adminID, r.PathValue("code")); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"cleared": true})
}
