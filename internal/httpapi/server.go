// Package httpapi exposes the service over HTTP. Handlers only decode input, call one service
// method, and encode the result; no game rules live here.
package httpapi

import (
	"net/http"
	"strconv"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/config"
	"github.com/demonreeper/terrandominion/api/internal/service"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

type Server struct {
	svc           *service.Service
	cfg           config.Config
	devLogin      bool
	ipLimiter     *limiter
	playerLimiter *limiter
}

func New(svc *service.Service, cfg config.Config, devLogin bool) *Server {
	return &Server{
		svc:           svc,
		cfg:           cfg,
		devLogin:      devLogin && !cfg.IsLive(),
		ipLimiter:     newLimiter(20, 60),
		playerLimiter: newLimiter(5, 20),
	}
}

// Handler is the full route table. The API is versioned in the path so an old app build keeps
// working while a new one rolls out.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { ok(w, "ok") })

	mux.HandleFunc("POST /v1/auth/google", s.signInGoogle)
	mux.HandleFunc("POST /v1/auth/refresh", s.refresh)
	mux.HandleFunc("POST /v1/auth/logout", s.authed(s.logout))
	if s.devLogin {
		mux.HandleFunc("POST /v1/auth/dev", s.signInDev)
	}

	mux.HandleFunc("GET /v1/settings", s.publicSettings)
	mux.HandleFunc("GET /v1/me", s.authed(s.me))
	mux.HandleFunc("POST /v1/me/delete", s.authed(s.requestDeletion))

	mux.HandleFunc("GET /v1/nodes/nearby", s.authed(s.nearbyNodes))
	mux.HandleFunc("POST /v1/nodes/{id}/harvest", s.authed(s.harvest))

	mux.HandleFunc("GET /v1/tethers/nearby", s.authed(s.nearbyTethers))
	mux.HandleFunc("POST /v1/tethers", s.authed(s.buildTether))
	mux.HandleFunc("POST /v1/tethers/{id}/dismantle", s.authed(s.dismantleTether))
	mux.HandleFunc("POST /v1/tethers/{id}/collect", s.authed(s.collectTether))
	mux.HandleFunc("POST /v1/tethers/{id}/upload", s.authed(s.uploadTether))

	mux.HandleFunc("GET /v1/factions", s.authed(s.factions))
	mux.HandleFunc("POST /v1/factions/{code}/lead", s.authed(s.leadFaction))
	mux.HandleFunc("POST /v1/factions/{code}/join", s.authed(s.joinFaction))

	mux.HandleFunc("GET /v1/admin/settings", s.admin(s.adminSettings))
	mux.HandleFunc("PUT /v1/admin/settings/{key}", s.admin(s.adminUpdateSetting))
	mux.HandleFunc("POST /v1/admin/nodes", s.admin(s.adminCreateNode))
	mux.HandleFunc("POST /v1/admin/nodes/scatter", s.admin(s.adminScatterNodes))
	mux.HandleFunc("POST /v1/admin/nodes/clear-scatter", s.admin(s.adminClearScatter))

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { fail(w, r, apperr.NotFound) })

	return s.base(mux)
}

// ---- auth ----------------------------------------------------------------------------

type deviceBody struct {
	DeviceID   string `json:"deviceId"`
	Platform   string `json:"platform"`
	AppVersion string `json:"appVersion"`
}

func (d deviceBody) info() service.DeviceInfo {
	return service.DeviceInfo{DeviceID: d.DeviceID, Platform: d.Platform, AppVersion: d.AppVersion}
}

func (s *Server) signInGoogle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDToken string `json:"idToken"`
		deviceBody
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.SignInGoogle(r.Context(), body.IDToken, body.info())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) signInDev(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
		deviceBody
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.SignInDev(r.Context(), body.Name, body.info())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
		AppVersion   string `json:"appVersion"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.Refresh(r.Context(), body.RefreshToken, body.AppVersion)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	if err := s.svc.Logout(r.Context(), playerID, sessionFrom(r.Context())); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "signed out")
}

// ---- player ----------------------------------------------------------------------------

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	me, err := s.svc.Me(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, me)
}

func (s *Server) requestDeletion(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	after, err := s.svc.RequestDeletion(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"deleteAfter": after})
}

func (s *Server) publicSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.svc.PublicSettings(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, settings)
}

// ---- world -----------------------------------------------------------------------------

func (s *Server) nearbyNodes(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(q.Get("lon"), 64)
	if err1 != nil || err2 != nil {
		fail(w, r, apperr.BadRequest.With("lat and lon are required"))
		return
	}
	radius, _ := strconv.ParseFloat(q.Get("radius"), 64)
	nodes, err := s.svc.NearbyNodes(r.Context(), playerID, lat, lon, radius)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, nodes)
}

func (s *Server) harvest(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	nodeID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("node id"))
		return
	}
	var body struct {
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		AccuracyM float64 `json:"accuracyM"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.Harvest(r.Context(), playerID, service.HarvestRequest{
		NodeID: nodeID, Lat: body.Lat, Lon: body.Lon, AccuracyM: body.AccuracyM,
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

// ---- tether nodes ----------------------------------------------------------------------

func (s *Server) nearbyTethers(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(q.Get("lon"), 64)
	if err1 != nil || err2 != nil {
		fail(w, r, apperr.BadRequest.With("lat and lon are required"))
		return
	}
	radius, _ := strconv.ParseFloat(q.Get("radius"), 64)
	tethers, err := s.svc.NearbyTethers(r.Context(), playerID, lat, lon, radius)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, tethers)
}

func (s *Server) buildTether(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		AccuracyM float64 `json:"accuracyM"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.BuildTether(r.Context(), playerID, service.BuildTetherRequest{
		Lat: body.Lat, Lon: body.Lon, AccuracyM: body.AccuracyM,
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) dismantleTether(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("tether id"))
		return
	}
	res, err := s.svc.DismantleTether(r.Context(), playerID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

// tetherAction reads the Tether Node id and GPS fix shared by collect and upload.
func tetherAction(w http.ResponseWriter, r *http.Request) (int64, service.TetherFix, error) {
	var fix service.TetherFix
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return 0, fix, apperr.BadRequest.With("tether id")
	}
	var body struct {
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		AccuracyM float64 `json:"accuracyM"`
	}
	if err := decode(w, r, &body); err != nil {
		return 0, fix, err
	}
	return id, service.TetherFix{Lat: body.Lat, Lon: body.Lon, AccuracyM: body.AccuracyM}, nil
}

func (s *Server) collectTether(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, fix, err := tetherAction(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.CollectTether(r.Context(), playerID, id, fix)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) uploadTether(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, fix, err := tetherAction(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.UploadTether(r.Context(), playerID, id, fix)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

// ---- factions --------------------------------------------------------------------------

func (s *Server) factions(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.Factions(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, list)
}

func (s *Server) leadFaction(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	m, err := s.svc.LeadFaction(r.Context(), playerID, r.PathValue("code"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, m)
}

func (s *Server) joinFaction(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	m, err := s.svc.JoinFaction(r.Context(), playerID, r.PathValue("code"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, m)
}

// ---- admin -----------------------------------------------------------------------------

func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.svc.AdminSettings(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, settings)
}

func (s *Server) adminUpdateSetting(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Value float64 `json:"value"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.AdminUpdateSetting(r.Context(), playerID, r.PathValue("key"), body.Value); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "updated")
}

func (s *Server) adminScatterNodes(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
		Count   int     `json:"count"`
		RadiusM float64 `json:"radiusM"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.AdminScatterNodes(r.Context(), playerID, body.Lat, body.Lon, body.Count, body.RadiusM)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) adminClearScatter(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	n, err := s.svc.AdminClearScatteredNodes(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"retired": n})
}

func (s *Server) adminCreateNode(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body store.NewNode
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	id, err := s.svc.AdminCreateNode(r.Context(), playerID, body)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"id": id})
}
