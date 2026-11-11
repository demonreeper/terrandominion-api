package httpapi

import (
	"net/http"

	"github.com/demonreeper/terrandominion/api/internal/service"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Regions from GPS (social-signal-plan.md Part B, R1).

func (s *Server) regionHere(w http.ResponseWriter, r *http.Request) {
	fix, _ := readFix(r)
	here, err := s.svc.RegionHere(r.Context(), service.RegionFix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, here)
}

func (s *Server) adminRegions(w http.ResponseWriter, r *http.Request) {
	tests, live, err := s.svc.AdminTestRegions(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"test": tests, "live": live})
}

func (s *Server) adminSaveTestRegion(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	var body store.TestRegion
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	body.Code = r.PathValue("code")
	if err := s.svc.AdminSaveTestRegion(r.Context(), adminID, body); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) adminDeleteTestRegion(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	if err := s.svc.AdminDeleteTestRegion(r.Context(), adminID, r.PathValue("code")); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]bool{"ok": true})
}

func (s *Server) adminSetLiveRegion(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	var body struct {
		Live bool `json:"live"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	live, err := s.svc.AdminSetLiveRegion(r.Context(), adminID, r.PathValue("code"), body.Live)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"live": live})
}

func (s *Server) factionRegions(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	view, err := s.svc.FactionRegions(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, view)
}

func (s *Server) viewRegion(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Region string `json:"region"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	view, err := s.svc.ViewRegion(r.Context(), playerID, body.Region)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, view)
}

func (s *Server) adminMoveRegion(w http.ResponseWriter, r *http.Request) {
	adminID, _ := playerFrom(r.Context())
	var body struct {
		Region string `json:"region"`
		Reason string `json:"reason"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	info, err := s.svc.AdminMoveRegion(r.Context(), adminID, r.PathValue("code"), body.Region, body.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, info)
}
