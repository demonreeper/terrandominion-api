package httpapi

import (
	"net/http"
	"strconv"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/service"
)

// Faction bases, B0: admin tools only (docs/base-building-plan.md). Players' routes come with B1.

func (s *Server) adminBases(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.AdminBases(r.Context(), r.URL.Query().Get("region"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, view)
}

func (s *Server) adminCheckBaseSpot(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(q.Get("lon"), 64)
	if err1 != nil || err2 != nil || q.Get("faction") == "" {
		fail(w, r, apperr.BadRequest.With("faction, lat and lon are required"))
		return
	}
	out, err := s.svc.AdminCheckBaseSpot(r.Context(), q.Get("faction"), lat, lon)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) adminPlaceBase(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	var body service.AdminPlaceBaseRequest
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	base, err := s.svc.AdminPlaceBase(r.Context(), adminID, body)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, base)
}

func (s *Server) adminSetBaseTier(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("base id"))
		return
	}
	var body struct {
		Tier  int  `json:"tier"`
		Force bool `json:"force"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	base, err := s.svc.AdminSetBaseTier(r.Context(), adminID, id, body.Tier, body.Force)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, base)
}

func (s *Server) adminClearBase(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("base id"))
		return
	}
	if err := s.svc.AdminClearBase(r.Context(), adminID, id); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"cleared": id})
}

func (s *Server) adminReleaseFlag(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("flag id"))
		return
	}
	if err := s.svc.AdminReleaseFlag(r.Context(), adminID, id); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"released": id})
}

// ---- players (B1: flags, the Commander's answer, the map and the base card) ----

func (s *Server) baseNames(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	names, err := s.svc.BaseNames(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"names": names})
}

func (s *Server) proposeBase(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body service.ProposeRequest
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	out, err := s.svc.ProposeBase(r.Context(), playerID, body)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) baseProposals(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	out, err := s.svc.Proposals(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) approveFlag(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("flag id"))
		return
	}
	base, err := s.svc.ApproveFlag(r.Context(), playerID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"base": base})
}

func (s *Server) declineFlag(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("flag id"))
		return
	}
	if err := s.svc.DeclineFlag(r.Context(), playerID, id); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"declined": id})
}

func (s *Server) setProposalsOff(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Off bool `json:"off"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.SetProposalsOff(r.Context(), playerID, body.Off); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"proposalsOff": body.Off})
}

func (s *Server) basesNearby(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(q.Get("lon"), 64)
	if err1 != nil || err2 != nil {
		fail(w, r, apperr.BadRequest.With("lat and lon are required"))
		return
	}
	radius, _ := strconv.ParseFloat(q.Get("radius"), 64)
	out, err := s.svc.BasesNearby(r.Context(), lat, lon, radius)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) baseCard(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("base id"))
		return
	}
	out, err := s.svc.BaseCard(r.Context(), playerID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

// ---- projects (B2) ----

func (s *Server) startProject(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("base id"))
		return
	}
	var body service.StartProjectRequest
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	out, err := s.svc.StartProject(r.Context(), playerID, id, body)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) giveToProject(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err1 := strconv.ParseInt(r.PathValue("id"), 10, 64)
	pid, err2 := strconv.ParseInt(r.PathValue("pid"), 10, 64)
	if err1 != nil || err2 != nil {
		fail(w, r, apperr.BadRequest.With("base or project id"))
		return
	}
	var body service.GiveRequest
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	out, err := s.svc.GiveToProject(r.Context(), playerID, id, pid, body)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) cancelProject(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err1 := strconv.ParseInt(r.PathValue("id"), 10, 64)
	pid, err2 := strconv.ParseInt(r.PathValue("pid"), 10, 64)
	if err1 != nil || err2 != nil {
		fail(w, r, apperr.BadRequest.With("base or project id"))
		return
	}
	if err := s.svc.CancelProject(r.Context(), playerID, id, pid); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"cancelled": pid})
}

// ---- economy (B4) ----

func (s *Server) baseAt(w http.ResponseWriter, r *http.Request, act func(playerID, baseID int64, body service.AtBaseRequest) (any, error)) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("base id"))
		return
	}
	var body service.AtBaseRequest
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	out, err := act(playerID, id, body)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

func (s *Server) baseDeposit(w http.ResponseWriter, r *http.Request) {
	s.baseAt(w, r, func(p, b int64, body service.AtBaseRequest) (any, error) {
		return s.svc.Deposit(r.Context(), p, b, body)
	})
}

func (s *Server) baseUpload(w http.ResponseWriter, r *http.Request) {
	s.baseAt(w, r, func(p, b int64, body service.AtBaseRequest) (any, error) {
		return s.svc.UploadAtBase(r.Context(), p, b, body)
	})
}

func (s *Server) baseVault(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("base id"))
		return
	}
	out, err := s.svc.Vault(r.Context(), playerID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, out)
}

// ---- combat (B5) ----

func (s *Server) baseCombat(act func(r *http.Request, playerID, baseID int64, body service.BaseAttackRequest) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		playerID, _ := playerFrom(r.Context())
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			fail(w, r, apperr.BadRequest.With("base id"))
			return
		}
		var body service.BaseAttackRequest
		if err := decode(w, r, &body); err != nil {
			fail(w, r, err)
			return
		}
		out, err := act(r, playerID, id, body)
		if err != nil {
			fail(w, r, err)
			return
		}
		ok(w, out)
	}
}

func defendOf(b service.BaseAttackRequest) service.DefendRequest {
	return service.DefendRequest{Lat: b.Lat, Lon: b.Lon, AccuracyM: b.AccuracyM, BaseTarget: b.BaseTarget}
}

func (s *Server) routeBaseCombat(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/bases/{id}/attack", s.authed(s.located(s.grounded(s.baseCombat(func(r *http.Request, p, b int64, body service.BaseAttackRequest) (any, error) {
		return s.svc.AttackBase(r.Context(), p, b, body)
	})))))
	mux.HandleFunc("POST /v1/bases/{id}/drone", s.authed(s.located(s.grounded(s.baseCombat(func(r *http.Request, p, b int64, body service.BaseAttackRequest) (any, error) {
		return s.svc.LaunchBaseDrone(r.Context(), p, b, body)
	})))))
	mux.HandleFunc("POST /v1/bases/{id}/hack", s.authed(s.located(s.grounded(s.baseCombat(func(r *http.Request, p, b int64, body service.BaseAttackRequest) (any, error) {
		return s.svc.HackBase(r.Context(), p, b, body)
	})))))
	mux.HandleFunc("POST /v1/bases/{id}/intercept", s.authed(s.located(s.grounded(s.baseCombat(func(r *http.Request, p, b int64, body service.BaseAttackRequest) (any, error) {
		return s.svc.InterceptBaseDrone(r.Context(), p, b, defendOf(body))
	})))))
	mux.HandleFunc("POST /v1/bases/{id}/defuse", s.authed(s.located(s.grounded(s.baseCombat(func(r *http.Request, p, b int64, body service.BaseAttackRequest) (any, error) {
		return map[string]any{"defused": true}, s.svc.DefuseBaseSpike(r.Context(), p, b, defendOf(body))
	})))))
	mux.HandleFunc("POST /v1/bases/{id}/repair", s.authed(s.located(s.grounded(s.baseCombat(func(r *http.Request, p, b int64, body service.BaseAttackRequest) (any, error) {
		return s.svc.RepairBase(r.Context(), p, b, defendOf(body))
	})))))
}

func (s *Server) baseFlights(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	out, err := s.svc.BaseFlights(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"flights": out})
}
