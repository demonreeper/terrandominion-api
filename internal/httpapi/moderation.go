package httpapi

import (
	"net/http"
	"strconv"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/service"
)

// Reports and moderation (docs/profile-plan.md, M1).

// reportPlayer files a griefing report on the player behind a share code.
func (s *Server) reportPlayer(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Detail string `json:"detail"`
		Note   string `json:"note"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.ReportPlayer(r.Context(), playerID, r.PathValue("code"), body.Detail, body.Note); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "reported")
}

func (s *Server) modReports(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	queue, err := s.svc.ModReports(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, queue)
}

func (s *Server) modDismissReport(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.NotFound)
		return
	}
	var body struct {
		Reason string `json:"reason"`
		// All also closes every other open report on the same player.
		All bool `json:"all"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.DismissReport(r.Context(), playerID, id, body.Reason, body.All); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "dismissed")
}

func (s *Server) modSanction(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Kind      string  `json:"kind"`
		Days      int     `json:"days"`
		Reason    string  `json:"reason"`
		ReportIDs []int64 `json:"reportIds"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	sanction, err := s.svc.Sanction(r.Context(), playerID, r.PathValue("code"), service.SanctionRequest{
		Kind: body.Kind, Days: body.Days, Reason: body.Reason, ReportIDs: body.ReportIDs,
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, sanction)
}

func (s *Server) adminSanctions(w http.ResponseWriter, r *http.Request) {
	sanctions, err := s.svc.AdminSanctions(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, sanctions)
}

func (s *Server) adminLiftSanction(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.NotFound)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	sanction, err := s.svc.LiftSanction(r.Context(), playerID, id, body.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, sanction)
}

func (s *Server) adminReopenReport(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.NotFound)
		return
	}
	if err := s.svc.ReopenReport(r.Context(), playerID, id); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "reopened")
}
