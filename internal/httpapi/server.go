// Package httpapi exposes the service over HTTP. Handlers only decode input, call one service
// method, and encode the result; no game rules live here.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

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
	// A refused request that failed a location check is stored on the player (td.infringements).
	onFail = func(r *http.Request, playerID int64, e *apperr.Error) {
		svc.NoteInfringement(r.Context(), playerID, e, r.URL.Path)
	}
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
	mux.HandleFunc("POST /v1/auth/apple", s.signInApple)
	mux.HandleFunc("POST /v1/auth/refresh", s.refresh)
	mux.HandleFunc("POST /v1/auth/logout", s.authed(s.logout))
	if s.devLogin {
		mux.HandleFunc("POST /v1/auth/dev", s.signInDev)
	}

	mux.HandleFunc("GET /v1/settings", s.publicSettings)
	mux.HandleFunc("GET /v1/me", s.authed(s.me))
	mux.HandleFunc("GET /v1/me/callsign/options", s.authed(s.callsignOptions))
	mux.HandleFunc("PUT /v1/me/callsign", s.authed(s.setCallsign))
	mux.HandleFunc("POST /v1/me/delete", s.authed(s.requestDeletion))
	mux.HandleFunc("PUT /v1/me/push", s.authed(s.setPushPrefs))
	mux.HandleFunc("GET /v1/players/{code}", s.authed(s.playerProfile))
	mux.HandleFunc("GET /v1/friends", s.authed(s.friends))
	mux.HandleFunc("GET /v1/friends/search", s.authed(s.findFriends))
	mux.HandleFunc("POST /v1/friends/{code}", s.authed(s.requestFriend))
	mux.HandleFunc("POST /v1/friends/{code}/accept", s.authed(s.answerFriend(true)))
	mux.HandleFunc("POST /v1/friends/{code}/decline", s.authed(s.answerFriend(false)))
	mux.HandleFunc("DELETE /v1/friends/{code}", s.authed(s.removeFriend))
	mux.HandleFunc("PUT /v1/me/friends", s.authed(s.setFriendSettings))
	mux.HandleFunc("PUT /v1/me/privacy", s.authed(s.setPrivacy))
	mux.HandleFunc("POST /v1/players/{code}/report", s.authed(s.reportPlayer))

	// Moderation: mods and admins.
	mux.HandleFunc("GET /v1/mod/reports", s.mod(s.modReports))
	mux.HandleFunc("POST /v1/mod/reports/{id}/dismiss", s.mod(s.modDismissReport))
	mux.HandleFunc("POST /v1/mod/players/{code}/sanction", s.mod(s.modSanction))
	mux.HandleFunc("POST /v1/me/link/google", s.authed(s.linkGoogle))
	mux.HandleFunc("POST /v1/me/link/apple", s.authed(s.linkApple))
	mux.HandleFunc("DELETE /v1/me/link/{provider}", s.authed(s.unlink))

	mux.HandleFunc("PUT /v1/push/token", s.authed(s.registerPushToken))
	mux.HandleFunc("DELETE /v1/push/token", s.authed(s.removePushToken))

	mux.HandleFunc("POST /v1/craft", s.authed(s.craft))
	mux.HandleFunc("POST /v1/gear/craft", s.authed(s.craftGear))
	mux.HandleFunc("POST /v1/gear/{id}/upgrade", s.authed(s.upgradeGear))
	mux.HandleFunc("POST /v1/gear/{id}/repair", s.authed(s.repairGear))

	mux.HandleFunc("GET /v1/nodes/nearby", s.authed(s.located(s.nearbyNodes)))
	mux.HandleFunc("POST /v1/nodes/{id}/harvest", s.authed(s.located(s.harvest)))

	mux.HandleFunc("GET /v1/tethers/nearby", s.authed(s.located(s.nearbyTethers)))
	mux.HandleFunc("GET /v1/tethers/mine", s.authed(s.myTethers))
	mux.HandleFunc("GET /v1/tethers/{id}", s.authed(s.located(s.getTether)))
	mux.HandleFunc("POST /v1/tethers", s.authed(s.located(s.buildTether)))
	mux.HandleFunc("POST /v1/tethers/{id}/upgrade", s.authed(s.located(s.upgradeTether)))
	mux.HandleFunc("POST /v1/tethers/{id}/dismantle", s.authed(s.dismantleTether))
	mux.HandleFunc("POST /v1/tethers/{id}/collect", s.authed(s.located(s.collectTether)))
	mux.HandleFunc("POST /v1/tethers/{id}/upload", s.authed(s.located(s.uploadTether)))
	mux.HandleFunc("POST /v1/tethers/{id}/attack", s.authed(s.located(s.attackTether)))
	mux.HandleFunc("POST /v1/tethers/{id}/defuse", s.authed(s.located(s.defuseSpike)))
	mux.HandleFunc("POST /v1/tethers/{id}/drone", s.authed(s.located(s.launchDrone)))
	mux.HandleFunc("POST /v1/tethers/{id}/intercept", s.authed(s.located(s.interceptDrone)))
	mux.HandleFunc("POST /v1/tethers/{id}/shield", s.authed(s.located(s.fitShield)))
	mux.HandleFunc("GET /v1/drones", s.authed(s.myDrones))

	mux.HandleFunc("POST /v1/nodes/{id}/contest", s.authed(s.located(s.startContest)))
	mux.HandleFunc("POST /v1/contests/{id}/join", s.authed(s.located(s.joinContest)))

	mux.HandleFunc("GET /v1/notices", s.authed(s.notices))

	mux.HandleFunc("GET /v1/drops/nearby", s.authed(s.located(s.nearbyDrops)))
	mux.HandleFunc("POST /v1/drops/{id}/pick", s.authed(s.located(s.pickDrop)))
	mux.HandleFunc("GET /v1/echoes/nearby", s.authed(s.located(s.nearbyEchoes)))
	// Walk together (docs/walk-together-plan.md): every call carries a fix, so the spoof checks cover it.
	mux.HandleFunc("POST /v1/walk/open", s.authed(s.located(s.walkOpen)))
	mux.HandleFunc("POST /v1/walk/ask", s.authed(s.located(s.walkAsk)))
	mux.HandleFunc("POST /v1/walk/ask/{code}/accept", s.authed(s.located(s.walkAccept)))
	mux.HandleFunc("POST /v1/walk/heartbeat", s.authed(s.located(s.walkHeartbeat)))
	mux.HandleFunc("DELETE /v1/walk", s.authed(s.walkLeave))
	mux.HandleFunc("POST /v1/echoes/{id}/read", s.authed(s.located(s.readEcho)))

	mux.HandleFunc("GET /v1/jobs", s.authed(s.jobs))
	mux.HandleFunc("POST /v1/jobs", s.authed(s.located(s.postJob)))
	mux.HandleFunc("POST /v1/jobs/{id}/take", s.authed(s.located(s.takeJob)))
	mux.HandleFunc("GET /v1/factions", s.authed(s.factions))
	mux.HandleFunc("POST /v1/factions/{code}/lead", s.authed(s.located(s.leadFaction)))
	mux.HandleFunc("POST /v1/factions/{code}/join", s.authed(s.located(s.joinFaction)))
	mux.HandleFunc("GET /v1/regions/here", s.authed(s.regionHere))
	mux.HandleFunc("GET /v1/factions/regions", s.authed(s.factionRegions))
	mux.HandleFunc("POST /v1/factions/view-region", s.authed(s.viewRegion))
	mux.HandleFunc("POST /v1/admin/players/{code}/move-region", s.admin(s.adminMoveRegion))
	mux.HandleFunc("GET /v1/admin/regions", s.admin(s.adminRegions))
	mux.HandleFunc("PUT /v1/admin/regions/test/{code}", s.admin(s.adminSaveTestRegion))
	mux.HandleFunc("DELETE /v1/admin/regions/test/{code}", s.admin(s.adminDeleteTestRegion))
	mux.HandleFunc("PUT /v1/admin/regions/live/{code}", s.admin(s.adminSetLiveRegion))
	mux.HandleFunc("GET /v1/factions/branch", s.authed(s.factionBranch))
	mux.HandleFunc("POST /v1/factions/branch/wardens", s.authed(s.appointWarden))
	mux.HandleFunc("GET /v1/missions", s.authed(s.missions))
	mux.HandleFunc("POST /v1/missions/{id}/claim", s.authed(s.claimMission))
	mux.HandleFunc("GET /v1/calls", s.authed(s.calls))
	mux.HandleFunc("POST /v1/calls/{id}/played", s.authed(s.playCall))
	mux.HandleFunc("POST /v1/calls/{id}/reject", s.authed(s.rejectCall))
	mux.HandleFunc("POST /v1/messages/{id}/read", s.authed(s.readMessage))

	mux.HandleFunc("GET /v1/admin/settings", s.admin(s.adminSettings))
	mux.HandleFunc("PUT /v1/admin/settings/{key}", s.admin(s.adminUpdateSetting))
	mux.HandleFunc("PUT /v1/admin/announcement", s.admin(s.adminSetAnnouncement))
	mux.HandleFunc("GET /v1/mod/players", s.mod(s.modPlayers))
	mux.HandleFunc("GET /v1/mod/players/{code}", s.mod(s.modPlayer))
	mux.HandleFunc("POST /v1/admin/players/{code}/adjust", s.admin(s.adminAdjustPlayer))
	mux.HandleFunc("POST /v1/admin/players/{code}/reset-callsign", s.admin(s.adminResetCallsign))
	mux.HandleFunc("PUT /v1/admin/players/{code}/mod", s.admin(s.adminSetMod))
	mux.HandleFunc("PUT /v1/admin/players/{code}/spoof-exempt", s.admin(s.adminSpoofExempt))
	mux.HandleFunc("POST /v1/admin/players/{code}/release-pledge", s.admin(s.adminReleasePledge))
	mux.HandleFunc("GET /v1/admin/marks/{code}", s.admin(s.adminMark))
	mux.HandleFunc("POST /v1/admin/marks/{code}/clear", s.admin(s.adminClearMark))
	mux.HandleFunc("GET /v1/admin/stats", s.admin(s.adminStats))
	mux.HandleFunc("GET /v1/admin/factions", s.admin(s.adminFactions))
	mux.HandleFunc("GET /v1/admin/factions/{code}", s.admin(s.adminFaction))
	mux.HandleFunc("GET /v1/admin/sanctions", s.admin(s.adminSanctions))
	mux.HandleFunc("POST /v1/admin/sanctions/{id}/lift", s.admin(s.adminLiftSanction))
	mux.HandleFunc("POST /v1/admin/reports/{id}/reopen", s.admin(s.adminReopenReport))
	mux.HandleFunc("POST /v1/admin/nodes", s.admin(s.adminCreateNode))
	mux.HandleFunc("POST /v1/admin/nodes/scatter", s.admin(s.adminScatterNodes))
	mux.HandleFunc("POST /v1/admin/nodes/clear-scatter", s.admin(s.adminClearScatter))
	mux.HandleFunc("POST /v1/admin/push/test", s.admin(s.adminPushTest))
	mux.HandleFunc("GET /v1/admin/seed", s.admin(s.adminSeedStatus))
	mux.HandleFunc("POST /v1/admin/seed", s.admin(s.adminSeedArea))
	mux.HandleFunc("POST /v1/admin/seed/cell", s.admin(s.adminReseedCell))
	// Faction bases, B0 admin tools (base-building-plan.md).
	mux.HandleFunc("GET /v1/admin/bases", s.admin(s.adminBases))
	mux.HandleFunc("GET /v1/admin/bases/check", s.admin(s.adminCheckBaseSpot))
	mux.HandleFunc("POST /v1/admin/bases", s.admin(s.adminPlaceBase))
	mux.HandleFunc("PUT /v1/admin/bases/{id}/tier", s.admin(s.adminSetBaseTier))
	mux.HandleFunc("POST /v1/admin/bases/{id}/clear", s.admin(s.adminClearBase))
	mux.HandleFunc("POST /v1/admin/flags/{id}/release", s.admin(s.adminReleaseFlag))

	// The web page behind a profile share link: public, no sign-in.
	mux.HandleFunc("GET /p/{code}", s.profilePage)
	// The files Android and iOS fetch to check that the app may open /p/ links.
	mux.HandleFunc("GET /.well-known/assetlinks.json", s.assetLinksFile)
	mux.HandleFunc("GET /.well-known/apple-app-site-association", s.appleAssociationFile)

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
		Nonce   string `json:"nonce"`
		deviceBody
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.SignInGoogle(r.Context(), body.IDToken, body.Nonce, body.info())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) signInApple(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IdentityToken     string `json:"identityToken"`
		AuthorizationCode string `json:"authorizationCode"`
		Nonce             string `json:"nonce"`
		Name              string `json:"name"`
		deviceBody
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.SignInApple(r.Context(), body.IdentityToken, body.AuthorizationCode, body.Nonce, body.Name, body.info())
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
	// environment sits beside data, so data stays the settings list. The app shows its dev text
	// (server host, map scale, GPS source, AR diagnostics) only on "dev".
	environment := "prod"
	if s.cfg.Env == "dev" {
		environment = "dev"
	}
	writeJSON(w, http.StatusOK, struct {
		Data        any    `json:"data"`
		Environment string `json:"environment"`
	}{Data: settings, Environment: environment})
}

// ---- push ------------------------------------------------------------------------------

func (s *Server) registerPushToken(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Token    string `json:"token"`
		Platform string `json:"platform"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.RegisterPushToken(r.Context(), playerID, sessionFrom(r.Context()), body.Token, body.Platform); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "registered")
}

func (s *Server) removePushToken(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	if err := s.svc.RemovePushToken(r.Context(), playerID, sessionFrom(r.Context())); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "removed")
}

func (s *Server) setPushPrefs(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Attacks bool `json:"attacks"`
		Results bool `json:"results"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.SetPushPrefs(r.Context(), playerID, body.Attacks, body.Results); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, body)
}

func (s *Server) linkGoogle(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		IDToken string `json:"idToken"`
		Nonce   string `json:"nonce"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.LinkGoogle(r.Context(), playerID, body.IDToken, body.Nonce)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) linkApple(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		IdentityToken     string `json:"identityToken"`
		AuthorizationCode string `json:"authorizationCode"`
		Nonce             string `json:"nonce"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.LinkApple(r.Context(), playerID, body.IdentityToken, body.AuthorizationCode, body.Nonce)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) unlink(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	res, err := s.svc.Unlink(r.Context(), playerID, r.PathValue("provider"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) adminPushTest(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	sent, err := s.svc.AdminPushTest(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]int{"sent": sent})
}

// ---- crafting --------------------------------------------------------------------------

func (s *Server) craft(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Recipe string `json:"recipe"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.Craft(r.Context(), playerID, body.Recipe)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) craftGear(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Type string `json:"type"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.CraftGear(r.Context(), playerID, body.Type)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) launchDrone(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, fix, err := tetherAction(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.LaunchDrone(r.Context(), playerID, id, fix)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) interceptDrone(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, fix, err := tetherAction(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.InterceptDrone(r.Context(), playerID, id, fix)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) fitShield(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("tether id"))
		return
	}
	var body struct {
		Item      string  `json:"item"`
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		AccuracyM float64 `json:"accuracyM"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.FitShield(r.Context(), playerID, id, body.Item,
		service.TetherFix{Lat: body.Lat, Lon: body.Lon, AccuracyM: body.AccuracyM})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) missions(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	res, err := s.svc.Missions(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) claimMission(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("mission id"))
		return
	}
	res, err := s.svc.ClaimMission(r.Context(), playerID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

// playerProfile is the public profile behind a share code.
func (s *Server) playerProfile(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	profile, err := s.svc.Profile(r.Context(), playerID, r.PathValue("code"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, profile)
}

// callsignOptions offers a handful of made-up callsigns to pick from; every call offers new ones.
func (s *Server) callsignOptions(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	options, err := s.svc.CallsignOptions(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, options)
}

// setCallsign picks or changes the name other players see, and answers with the new /me.
func (s *Server) setCallsign(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Callsign string `json:"callsign"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	me, err := s.svc.SetCallsign(r.Context(), playerID, body.Callsign)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, me)
}

func (s *Server) factionBranch(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	res, err := s.svc.Branch(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) appointWarden(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Code string `json:"code"`
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
	if err := s.svc.AppointWarden(r.Context(), playerID, targetID); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "appointed")
}

func (s *Server) myDrones(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	drones, err := s.svc.MyDrones(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, drones)
}

func (s *Server) upgradeGear(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("gear id"))
		return
	}
	res, err := s.svc.UpgradeGear(r.Context(), playerID, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) repairGear(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("gear id"))
		return
	}
	if err := s.svc.RepairGear(r.Context(), playerID, id); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "repaired")
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
	// A scan with a fix counts toward the player's scout and guard jobs (F6).
	if accuracy, err := strconv.ParseFloat(q.Get("accuracy"), 64); err == nil && accuracy > 0 {
		s.svc.JobFix(r.Context(), playerID, lat, lon, accuracy)
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

func (s *Server) myTethers(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	list, err := s.svc.MyTethers(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, list)
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

func (s *Server) getTether(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("tether id"))
		return
	}
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(q.Get("lon"), 64)
	if err1 != nil || err2 != nil {
		fail(w, r, apperr.BadRequest.With("lat and lon are required"))
		return
	}
	accuracy, _ := strconv.ParseFloat(q.Get("accuracyM"), 64)
	res, err := s.svc.GetTether(r.Context(), playerID, id, service.TetherFix{Lat: lat, Lon: lon, AccuracyM: accuracy})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) upgradeTether(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, fix, err := tetherAction(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.UpgradeTether(r.Context(), playerID, id, fix)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
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

// ---- combat ----------------------------------------------------------------------------

func (s *Server) attackTether(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("tether id"))
		return
	}
	var body struct {
		Weapon    string  `json:"weapon"`
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		AccuracyM float64 `json:"accuracyM"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.AttackTether(r.Context(), playerID, id, body.Weapon,
		service.TetherFix{Lat: body.Lat, Lon: body.Lon, AccuracyM: body.AccuracyM})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) defuseSpike(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, fix, err := tetherAction(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.DefuseSpike(r.Context(), playerID, id, fix); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "defused")
}

func (s *Server) startContest(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, fix, err := tetherAction(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.StartContest(r.Context(), playerID, id, fix)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) joinContest(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, fix, err := tetherAction(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.JoinContest(r.Context(), playerID, id, fix); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "joined")
}

func (s *Server) notices(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	list, err := s.svc.Notices(r.Context(), playerID, after)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, list)
}

// ---- map drops -------------------------------------------------------------------------

func (s *Server) nearbyDrops(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(q.Get("lon"), 64)
	if err1 != nil || err2 != nil {
		fail(w, r, apperr.BadRequest.With("lat and lon are required"))
		return
	}
	drops, err := s.svc.NearbyDrops(r.Context(), playerID, lat, lon)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, drops)
}

func (s *Server) pickDrop(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, fix, err := tetherAction(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.PickDrop(r.Context(), playerID, id, fix)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

// ---- signal echoes (no-ar-support-plan.md milestone 7) ---------------------------------

func (s *Server) nearbyEchoes(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(q.Get("lon"), 64)
	if err1 != nil || err2 != nil {
		fail(w, r, apperr.BadRequest.With("lat and lon are required"))
		return
	}
	echoes, err := s.svc.NearbyEchoes(r.Context(), playerID, lat, lon)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, echoes)
}

func (s *Server) readEcho(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("echo id"))
		return
	}
	var body struct {
		Lat       float64 `json:"lat"`
		Lon       float64 `json:"lon"`
		AccuracyM float64 `json:"accuracyM"`
		// ArView: the AR view was open. Echoes are read only there.
		ArView bool `json:"arView"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.ReadEcho(r.Context(), playerID, id,
		service.TetherFix{Lat: body.Lat, Lon: body.Lon, AccuracyM: body.AccuracyM}, body.ArView)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

// ---- walk together (docs/walk-together-plan.md) ----------------------------------------

// walkBody is what every walk call sends: the fix (with the fake flag and the fix's time, also accepted
// from the G1 headers) and, for an ask, the friend's share code.
type walkBody struct {
	Code      string     `json:"code"`
	Lat       float64    `json:"lat"`
	Lon       float64    `json:"lon"`
	AccuracyM float64    `json:"accuracyM"`
	Mock      bool       `json:"mock"`
	FixAt     *time.Time `json:"fixAt"`
	// Heartbeats (W2): Bluetooth on and allowed on this phone, and the tokens it heard.
	BleOn    bool     `json:"bleOn"`
	BleHeard []string `json:"bleHeard"`
}

func readWalk(w http.ResponseWriter, r *http.Request) (walkBody, service.WalkFixIn, error) {
	var body walkBody
	if err := decode(w, r, &body); err != nil {
		return body, service.WalkFixIn{}, err
	}
	fix, _ := readFix(r)
	in := service.WalkFixIn{Lat: body.Lat, Lon: body.Lon, AccuracyM: body.AccuracyM, Mock: body.Mock || fix.Mock, FixAt: body.FixAt}
	if in.FixAt == nil {
		in.FixAt = fix.FixAt
	}
	return body, in, nil
}

func (s *Server) walkOpen(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	_, in, err := readWalk(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.OpenWalk(r.Context(), playerID, sessionFrom(r.Context()), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) walkAsk(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	body, in, err := readWalk(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.AskWalk(r.Context(), playerID, sessionFrom(r.Context()), body.Code, in); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]bool{"asked": true})
}

func (s *Server) walkAccept(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	_, in, err := readWalk(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.AcceptWalk(r.Context(), playerID, sessionFrom(r.Context()), r.PathValue("code"), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) walkHeartbeat(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	body, in, err := readWalk(w, r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.WalkHeartbeat(r.Context(), playerID, in, service.WalkBeatIn{BleOn: body.BleOn, BleHeard: body.BleHeard})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) walkLeave(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	if err := s.svc.LeaveWalk(r.Context(), playerID); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]bool{"left": true})
}

// ---- the powers' calls and messages ----------------------------------------------------

func (s *Server) calls(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	res, err := s.svc.Calls(r.Context(), playerID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) playCall(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("call id"))
		return
	}
	if err := s.svc.PlayCall(r.Context(), playerID, id); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "played")
}

func (s *Server) rejectCall(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("call id"))
		return
	}
	if err := s.svc.RejectCall(r.Context(), playerID, id); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "rejected")
}

func (s *Server) readMessage(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		fail(w, r, apperr.BadRequest.With("message id"))
		return
	}
	if err := s.svc.ReadMessage(r.Context(), playerID, id); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, "read")
}

// ---- factions --------------------------------------------------------------------------

func (s *Server) factions(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	fix, _ := readFix(r)
	list, err := s.svc.Factions(r.Context(), playerID, service.RegionFix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, list)
}

func (s *Server) leadFaction(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	fix, _ := readFix(r)
	m, err := s.svc.LeadFaction(r.Context(), playerID, r.PathValue("code"), service.RegionFix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, m)
}

func (s *Server) joinFaction(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	fix, _ := readFix(r)
	m, err := s.svc.JoinFaction(r.Context(), playerID, r.PathValue("code"), service.RegionFix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM})
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
	key := r.PathValue("key")
	build, _ := strconv.Atoi(r.Header.Get("X-App-Build"))
	if err := s.svc.CheckMinAppBuildChange(key, body.Value, r.Header.Get("X-App-Platform"), build); err != nil {
		fail(w, r, err)
		return
	}
	setting, err := s.svc.AdminUpdateSetting(r.Context(), playerID, key, body.Value)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, setting)
}

// adminSetAnnouncement posts, replaces or (empty text) removes the main menu's one-line banner.
func (s *Server) adminSetAnnouncement(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Text  string `json:"text"`
		Kind  string `json:"kind"`
		Hours int    `json:"hours"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	announcement, err := s.svc.AdminSetAnnouncement(r.Context(), playerID, body.Text, body.Kind, body.Hours)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, struct {
		Announcement any `json:"announcement"`
	}{announcement})
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

// adminSeedStatus is the Field Ops readout for the cell at lat, lon, plus the seed areas.
func (s *Server) adminSeedStatus(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, err1 := strconv.ParseFloat(q.Get("lat"), 64)
	lon, err2 := strconv.ParseFloat(q.Get("lon"), 64)
	if err1 != nil || err2 != nil {
		fail(w, r, apperr.BadRequest.With("lat and lon are required"))
		return
	}
	status, err := s.svc.AdminSeedStatus(r.Context(), lat, lon)
	if err != nil {
		fail(w, r, err)
		return
	}
	areas, err := s.svc.SeedAreaIDs(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"cell": status, "areas": areas})
}

// adminSeedArea starts pre-seeding an area in the background (a big area takes longer than one
// request may); progress shows in GET /admin/seed's totals and in the server log.
func (s *Server) adminSeedArea(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		AreaID string `json:"areaId"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	if body.AreaID == "" {
		fail(w, r, apperr.BadRequest.With("areaId is required"))
		return
	}
	if _, err := s.svc.SeedAreaCheck(r.Context(), body.AreaID); err != nil {
		fail(w, r, err)
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if _, err := s.svc.SeedArea(ctx, playerID, body.AreaID); err != nil {
			slog.Error("E1900 area seeding failed", "area", body.AreaID, "err", err)
		}
	}()
	ok(w, map[string]any{"started": body.AreaID})
}

func (s *Server) adminReseedCell(w http.ResponseWriter, r *http.Request) {
	playerID, _ := playerFrom(r.Context())
	var body struct {
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}
	if err := decode(w, r, &body); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.AdminReseedCell(r.Context(), playerID, body.Lat, body.Lon)
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
