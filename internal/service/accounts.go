package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/auth"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

type DeviceInfo struct {
	DeviceID   string
	Platform   string
	AppVersion string
}

type AuthResult struct {
	AccessToken     string    `json:"accessToken"`
	AccessExpiresAt time.Time `json:"accessExpiresAt"`
	RefreshToken    string    `json:"refreshToken"`
	NewAccount      bool      `json:"newAccount"`
	Player          *MeView   `json:"player"`
}

func (d DeviceInfo) validate() *apperr.Error {
	if len(d.DeviceID) < 8 || len(d.DeviceID) > 200 {
		return apperr.DeviceRequired
	}
	switch d.Platform {
	case "android", "ios", "editor", "pc":
	default:
		return apperr.BadRequest.With("unknown platform %q", d.Platform)
	}
	return nil
}

// SignInGoogle verifies a Google ID token and signs the player in, creating the account on first
// sign-in. A new account gets starter gear, and the welcome resources only if this phone has not
// already had them.
func (s *Service) SignInGoogle(ctx context.Context, idToken, nonce string, device DeviceInfo) (AuthResult, error) {
	if err := device.validate(); err != nil {
		return AuthResult{}, err
	}
	identity, err := s.google.Verify(ctx, idToken)
	if err != nil {
		return AuthResult{}, apperr.GoogleTokenInvalid.With("%v", err)
	}
	// The app asks Google to put its own random nonce in the token, so a token lifted from
	// elsewhere can't be replayed here. Older apps send none.
	if nonce != "" && identity.Nonce != nonce {
		return AuthResult{}, apperr.GoogleTokenInvalid.With("nonce does not match")
	}
	return s.signIn(ctx, "google", identity.Subject, identity.Email, identity.Name, device)
}

// SignInApple verifies a Sign in with Apple identity token and signs the player in, the same way
// as SignInGoogle. rawNonce is required: the app sent its SHA-256 to Apple. name is only sent on
// the first sign-in (Apple shares it once) and only names a new account. authorizationCode is
// accepted but not used yet; before App Store submission it gets exchanged for a refresh token so
// account deletion can revoke the Apple sign-in.
func (s *Service) SignInApple(ctx context.Context, identityToken, authorizationCode, rawNonce, name string, device DeviceInfo) (AuthResult, error) {
	if err := device.validate(); err != nil {
		return AuthResult{}, err
	}
	_ = authorizationCode
	identity, err := s.apple.Verify(ctx, identityToken, rawNonce)
	if err != nil {
		return AuthResult{}, apperr.AppleTokenInvalid.With("%v", err)
	}
	return s.signIn(ctx, "apple", identity.Subject, identity.Email, name, device)
}

// SignInDev signs in without Google, for local testing only. The HTTP layer exposes it only when
// TD_DEV_LOGIN is on and TD_ENV is dev.
func (s *Service) SignInDev(ctx context.Context, name string, device DeviceInfo) (AuthResult, error) {
	if err := device.validate(); err != nil {
		return AuthResult{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return AuthResult{}, apperr.BadRequest.With("dev sign-in needs a name")
	}
	return s.signIn(ctx, "dev", strings.ToLower(name), "", name, device)
}

func (s *Service) signIn(ctx context.Context, provider, subject, email, name string, device DeviceInfo) (AuthResult, error) {
	settings := s.Settings(ctx)
	deviceHash := auth.HashDevice(device.DeviceID, s.cfg.DevicePepper)
	var result AuthResult

	signinHash := s.signinHash(provider, subject)
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		// The device record first: a banned phone or sign-in gets no account, new or old.
		verdict, err := s.checkMarks(ctx, tx, deviceHash, signinHash)
		if err != nil {
			return err
		}
		playerID, err := store.FindIdentity(ctx, tx, provider, subject)
		isNew := errors.Is(err, store.ErrNotFound)
		if err != nil && !isNew {
			return err
		}

		if isNew {
			// The owner's first sign-in on a database with no player 1 takes id 1 (the primary admin).
			owner := provider != "dev" && game.IsPrimaryAdminEmail(s.cfg.PrimaryAdminEmail, email)
			made := false
			if owner {
				if made, err = store.CreatePrimaryAdmin(ctx, tx, game.PrimaryAdminID, defaultDisplayName(name), "MV"); err != nil {
					return err
				}
			}
			if made {
				playerID = game.PrimaryAdminID
			} else if playerID, err = store.CreatePlayer(ctx, tx, defaultDisplayName(name), "MV"); err != nil {
				return err
			}
			if err := store.LinkIdentity(ctx, tx, provider, subject, playerID, email); err != nil {
				return err
			}
			if err := store.GrantStarterGear(ctx, tx, playerID); err != nil {
				return err
			}
		}

		player, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if err != nil {
			return err
		}
		switch player.Status {
		case "banned":
			return apperr.AccountBanned
		case "deleted":
			return apperr.AccountDeleted
		}
		if err := s.checkSuspension(ctx, tx, player); err != nil {
			return err
		}

		granted, err := store.RegisterDevice(ctx, tx, deviceHash, playerID, isNew)
		if err != nil {
			return err
		}
		if isNew && !granted {
			// Uploaded, so it doesn't fill the carry limit: building and crafting pay from there.
			for resource, amount := range map[string]int{
				"crystal": settings.WelcomeCrystal, "alloy": settings.WelcomeAlloy, "data": settings.WelcomeData,
			} {
				if err := store.AddUploaded(ctx, tx, playerID, resource, int64(amount)); err != nil {
					return err
				}
			}
			if settings.WelcomeCells > 0 {
				if err := store.AddItem(ctx, tx, playerID, "energy_cell", settings.WelcomeCells); err != nil {
					return err
				}
			}
			if err := store.MarkWelcomeGranted(ctx, tx, deviceHash); err != nil {
				return err
			}
		}

		if err := store.TouchPlayer(ctx, tx, playerID, device.AppVersion); err != nil {
			return err
		}
		if err := s.applyMarks(ctx, tx, playerID, verdict, deviceHash, signinHash); err != nil {
			return err
		}

		refresh, refreshHash, err := auth.NewRefreshToken()
		if err != nil {
			return err
		}
		expires := s.now().Add(time.Duration(settings.SessionDays) * 24 * time.Hour)
		sessionID, err := store.CreateSession(ctx, tx, playerID, refreshHash, deviceHash, device.Platform, device.AppVersion, expires)
		if err != nil {
			return err
		}

		access, accessExpires, err := s.tokens.IssueAccess(playerID, sessionID, player.IsAdmin,
			time.Duration(settings.AccessTokenMinutes)*time.Minute)
		if err != nil {
			return err
		}

		me, err := s.meView(ctx, tx, playerID, settings)
		if err != nil {
			return err
		}
		result = AuthResult{
			AccessToken: access, AccessExpiresAt: accessExpires, RefreshToken: refresh,
			NewAccount: isNew, Player: me,
		}
		if isNew {
			slog.Info("player created", "player", playerID, "provider", provider, "welcome", !granted)
		}
		return nil
	})
	return result, err
}

// Refresh spends a refresh token and returns a new pair. A spent, revoked or expired token fails,
// and the player's status is checked again, so a ban lands within one access-token lifetime.
func (s *Service) Refresh(ctx context.Context, refreshToken, appVersion string) (AuthResult, error) {
	settings := s.Settings(ctx)
	var result AuthResult

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		session, err := store.SessionByRefreshForUpdate(ctx, tx, auth.HashRefreshToken(refreshToken))
		if errors.Is(err, store.ErrNotFound) {
			return apperr.SessionExpired
		}
		if err != nil {
			return err
		}
		if session.RevokedAt.Valid || s.now().After(session.ExpiresAt) {
			return apperr.SessionExpired
		}

		player, err := store.GetPlayer(ctx, tx, session.PlayerID)
		if err != nil {
			return err
		}
		switch player.Status {
		case "banned":
			_ = store.RevokeAllSessions(ctx, tx, player.ID)
			return apperr.AccountBanned
		case "deleted":
			return apperr.AccountDeleted
		}
		if err := s.checkSuspension(ctx, tx, player); err != nil {
			return err
		}

		refresh, refreshHash, err := auth.NewRefreshToken()
		if err != nil {
			return err
		}
		expires := s.now().Add(time.Duration(settings.SessionDays) * 24 * time.Hour)
		if err := store.RotateSession(ctx, tx, session.ID, refreshHash, appVersion, expires); err != nil {
			return err
		}
		if err := store.TouchPlayer(ctx, tx, player.ID, appVersion); err != nil {
			return err
		}

		access, accessExpires, err := s.tokens.IssueAccess(player.ID, session.ID, player.IsAdmin,
			time.Duration(settings.AccessTokenMinutes)*time.Minute)
		if err != nil {
			return err
		}
		result = AuthResult{AccessToken: access, AccessExpiresAt: accessExpires, RefreshToken: refresh}
		return nil
	})
	return result, err
}

func (s *Service) Logout(ctx context.Context, playerID, sessionID int64) error {
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := store.DeleteSessionPushTokens(ctx, tx, sessionID, playerID); err != nil {
			return err
		}
		return store.RevokeSession(ctx, tx, sessionID, playerID)
	})
}

// RequestDeletion starts the grace period and signs every device out. Signing in again within
// DeleteGraceDays cancels it.
func (s *Service) RequestDeletion(ctx context.Context, playerID int64) (time.Time, error) {
	settings := s.Settings(ctx)
	after := s.now().Add(time.Duration(settings.DeleteGraceDays) * 24 * time.Hour)
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := store.ScheduleDeletion(ctx, tx, playerID, after); err != nil {
			return err
		}
		// A Commander deleting the account gives up the seat at once (leader succession).
		if err := s.leaveSeat(ctx, tx, playerID, game.RankMember, "deleting account"); err != nil {
			return err
		}
		if err := store.DeletePlayerPushTokens(ctx, tx, playerID); err != nil {
			return err
		}
		return store.RevokeAllSessions(ctx, tx, playerID)
	})
	return after, err
}

// MeView is the signed-in player's own view of their account.
type MeView struct {
	ID int64 `json:"id"`
	// Code is the share code; Name the player-facing name; Callsign is null until one is picked;
	// CallsignChangeAt is when it may next be changed (absent when it can be changed now).
	// ShareURL is the link to this profile. The account's real name is never sent.
	Code             string     `json:"code"`
	Name             string     `json:"name"`
	Callsign         *string    `json:"callsign"`
	CallsignChangeAt *time.Time `json:"callsignChangeAt,omitempty"`
	ShareURL         string     `json:"shareUrl"`
	Type             string     `json:"type"`
	IsAdmin          bool       `json:"isAdmin,omitempty"`
	// IsMod: a moderator (the ADMIN area shows them the report queue). Admins can do all a mod can.
	IsMod             bool                  `json:"isMod,omitempty"`
	Progress          game.Progress         `json:"progress"`
	Carried           map[string]int64      `json:"carried"`
	Stored            map[string]int64      `json:"stored"`
	TethersOwned      int                   `json:"tethersOwned"`
	TetherMax         int                   `json:"tetherMax"`
	TetherCost        map[string]int64      `json:"tetherCost"`
	TetherUpgradeCost map[string]int64      `json:"tetherUpgradeCost"`
	CarryCap          int64                 `json:"carryCapacity"`
	ScanRangeM        float64               `json:"scanRangeM"`
	Gear              []store.GearItem      `json:"gear"`
	Items             map[string]int64      `json:"items"`
	Recipes           []game.Recipe         `json:"recipes"`
	CraftableGear     []store.CraftableGear `json:"craftableGear"`
	Faction           *store.Membership     `json:"faction"`
	// PledgeBound: a pledge made on this phone or sign-in before stands. At the faction level the
	// player may pledge only to this faction ("Your pledge stands."). AppealCode and AppealURL go with
	// the APPEAL link for a second-hand phone.
	PledgeBound string `json:"pledgeBound,omitempty"`
	// FavourOpen: a freelancer, who earns a power's daily favour by finishing its mission (F4).
	FavourOpen bool `json:"favourOpen,omitempty"`
	// Contact: the seventh voice's first call has been heard or turned down, so its name is known.
	// The key names nothing on purpose: nothing names it before that call. JobsUnlocked follows it
	// for a freelancer (the JOBS board, F6). Note is its line on FACTIONS at the faction level.
	Contact      bool       `json:"contact,omitempty"`
	JobsUnlocked bool       `json:"jobsUnlocked,omitempty"`
	Note         *VoiceNote `json:"note,omitempty"`
	AppealCode   string     `json:"appealCode,omitempty"`
	AppealURL    string     `json:"appealUrl,omitempty"`
	// Badges are permanent marks, such as Founder of a faction's branch in a country.
	Badges []store.Badge `json:"badges"`
	// Linked are the sign-ins that open this account ("apple", "google"; "dev" on the dev server).
	Linked      []string   `json:"linked"`
	Freelancer  bool       `json:"freelancer"`
	PushAttacks bool       `json:"pushAttacks"`
	PushResults bool       `json:"pushResults"`
	DeleteAfter *time.Time `json:"deleteAfter,omitempty"`
	// ActivePerks are the faction perks working on this player now (empty for a freelancer). Carry
	// capacity, scan range and the Tether Node costs above already include them.
	ActivePerks []game.PerkLine `json:"activePerks"`
	// Announcement is the one-line banner for the main menu while one is running (admin settings).
	Announcement *store.Announcement `json:"announcement,omitempty"`
	// SignalLockedUntil: a signal lock is running (anti-spoofing); location actions are refused until then.
	// SignalLockLine is the narrator line it carries: first or repeat.
	SignalLockedUntil *time.Time `json:"signalLockedUntil,omitempty"`
	SignalLockLine    string     `json:"signalLockLine,omitempty"`
	// OpenReports is how many players have open reports (staff only), for the ADMIN button.
	OpenReports int `json:"openReports,omitempty"`
	// EmailSearchable: other players can find this one by their whole email (FR1). FriendRequestsWaiting:
	// requests waiting for this player's answer (the FRIENDS row's count).
	EmailSearchable bool `json:"emailSearchable"`
	// SpoofExempt (admins only): the admin's own SPOOF EXEMPT switch on the ADMIN page, off by default.
	SpoofExempt           bool `json:"spoofExempt,omitempty"`
	FriendRequestsWaiting int  `json:"friendRequestsWaiting"`
	// Codex is the signal echoes found, set by set (no-ar-support-plan.md milestone 7). Your own only.
	Codex []CodexSet `json:"codex"`
	// WalkLink is the player's walk-together link, or null; WalkedTogetherM the profile stat (own only).
	WalkLink        *WalkLinkView `json:"walkLink"`
	WalkedTogetherM float64       `json:"walkedTogetherM"`
}

func (s *Service) Me(ctx context.Context, playerID int64) (*MeView, error) {
	return s.meView(ctx, s.db, playerID, s.Settings(ctx))
}

func (s *Service) meView(ctx context.Context, q db.Querier, playerID int64, settings game.Settings) (*MeView, error) {
	player, err := store.GetPlayer(ctx, q, playerID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apperr.Unauthorized
	}
	if err != nil {
		return nil, err
	}
	carried, err := store.Carried(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	uploaded, err := store.Uploaded(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	tethersOwned, err := store.CountActiveTethers(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	gear, err := store.Gear(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	for i := range gear {
		gear[i].RepairCost = game.RepairCost(settings, gear[i].Durability, gear[i].MaxDurability)
	}
	items, err := store.Items(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	craftable, err := store.CraftableGearFor(ctx, q, playerID, game.CraftableGearIDs)
	if err != nil {
		return nil, err
	}
	stats, perks, err := s.equipped(ctx, q, playerID, settings)
	// The costs this player pays: their faction may discount them.
	own := game.SettingsWithPerks(settings, perks)
	if err != nil {
		return nil, err
	}
	membership, err := store.MembershipOf(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	s.describeMembership(ctx, membership)
	badges, err := store.Badges(ctx, q, playerID)
	if err != nil {
		return nil, err
	}

	pushAttacks, pushResults, err := store.PushPrefs(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	linked, err := store.PlayerIdentities(ctx, q, playerID)
	if err != nil {
		return nil, err
	}

	me := &MeView{
		PushAttacks: pushAttacks, PushResults: pushResults, Linked: linked, ActivePerks: []game.PerkLine{},
		ID: player.ID, Code: player.Code, Name: player.Name, ShareURL: s.ShareURL(player.Code),
		Type: player.Type, IsAdmin: player.IsAdmin, IsMod: player.IsMod,
		Progress: game.ProgressFor(player.XP, settings), Carried: carried, Stored: uploaded, TethersOwned: tethersOwned,
		TetherMax: settings.TnMaxPerPlayer, TetherCost: game.TetherBuildCost(own), TetherUpgradeCost: game.TetherUpgradeCost(own), CarryCap: stats.CarryCapacity,
		ScanRangeM: stats.ScanRangeM, Gear: gear, Items: items, Recipes: game.Recipes(settings), CraftableGear: craftable, Faction: membership, Badges: badges, Freelancer: membership == nil,
	}
	if membership == nil {
		bound, err := store.BoundFaction(ctx, q, playerID)
		if err != nil {
			return nil, err
		}
		if bound != "" {
			me.PledgeBound = bound
			if me.AppealCode, err = s.boundAppeal(ctx, q, playerID, bound); err != nil {
				return nil, err
			}
			me.AppealURL = game.AppealMailto(me.AppealCode)
		}
	}
	if player.DeleteAfter.Valid {
		t := player.DeleteAfter.Time
		me.DeleteAfter = &t
	}
	// Working on the player now: their faction's perks, or for a freelancer today's favour.
	if _, lines, err := s.perksOf(ctx, q, playerID, settings); err != nil {
		return nil, err
	} else if lines != nil {
		me.ActivePerks = lines
	}
	me.FavourOpen = membership == nil
	if membership == nil {
		if me.Contact, err = store.LangisContact(ctx, q, playerID); err != nil {
			return nil, err
		}
		me.JobsUnlocked = me.Contact
		if me.Contact && player.Level >= settings.LeadFactionLevel {
			me.Note = &VoiceNote{Text: game.LangisLineStayFree, Sign: game.LangisSign}
		}
	}
	if me.Announcement, err = store.CurrentAnnouncement(ctx, q); err != nil {
		return nil, err
	}
	if err := s.signalLock(ctx, q, me, settings); err != nil {
		return nil, err
	}
	if friendSettings, err := store.FriendSettingsOf(ctx, q, playerID); err != nil {
		return nil, err
	} else {
		me.EmailSearchable = friendSettings.FindableByEmail
	}
	if me.FriendRequestsWaiting, err = store.RequestsWaiting(ctx, q, playerID); err != nil {
		return nil, err
	}
	if player.IsAdmin || player.IsMod {
		if me.OpenReports, err = store.OpenReportCount(ctx, q); err != nil {
			return nil, err
		}
	}
	if player.IsAdmin {
		state, err := store.SpoofStateOf(ctx, q, playerID, s.now())
		if err != nil {
			return nil, err
		}
		me.SpoofExempt = state.Exempt
	}
	if player.Callsign.Valid {
		callsign := player.Callsign.String
		me.Callsign = &callsign
	}
	if player.CallsignChangedAt.Valid {
		if at := game.CallsignChangeAt(&player.CallsignChangedAt.Time, settings.CallsignChangeDays); at.After(s.now()) {
			me.CallsignChangeAt = &at
		}
	}
	if me.Codex, err = codex(ctx, q, playerID); err != nil {
		return nil, err
	}
	if linkID, err := store.LiveWalkLink(ctx, q, playerID); err != nil {
		return nil, err
	} else if linkID != 0 {
		if me.WalkLink, err = s.walkLinkView(ctx, q, linkID, playerID, settings); err != nil {
			return nil, err
		}
	}
	if me.WalkedTogetherM, err = store.WalkedTogether(ctx, q, playerID); err != nil {
		return nil, err
	}
	return me, nil
}

// awardXP adds XP and keeps the stored level in step. Every action calls it.
func awardXP(ctx context.Context, q db.Querier, playerID int64, action string, xp int, refID int64,
	settings game.Settings) (game.Progress, bool, error) {
	return awardXPAgainst(ctx, q, playerID, action, xp, refID, 0, settings)
}

// awardXPAgainst is awardXP for a fight: the ledger keeps the other player, for the daily cap.
func awardXPAgainst(ctx context.Context, q db.Querier, playerID int64, action string, xp int, refID, targetID int64,
	settings game.Settings) (game.Progress, bool, error) {
	before, err := store.GetPlayer(ctx, q, playerID)
	if err != nil {
		return game.Progress{}, false, err
	}
	total, err := store.AddXP(ctx, q, playerID, action, xp, refID, targetID)
	if err != nil {
		return game.Progress{}, false, err
	}
	progress := game.ProgressFor(total, settings)
	now := time.Now()
	// Halfway through level 10 and level 19 the matching power calls a freelancer.
	if err := startCalls(ctx, q, playerID, before.XP, total, settings, now); err != nil {
		return progress, false, err
	}
	// Levels only ever go up: an admin may grant a level by hand, and XP must not take it away.
	if progress.Level > before.Level {
		if err := store.SetLevel(ctx, q, playerID, progress.Level); err != nil {
			return progress, false, err
		}
		// A freelancer reaching the faction level is invited by the power that speaks to them.
		if err := maybeInvite(ctx, q, playerID, before.Level, progress.Level, settings, now); err != nil {
			return progress, false, err
		}
		// Past it, a freelancer keeps earning Unbound titles from the seventh voice.
		if err := grantTitles(ctx, q, playerID, progress.Level, settings, now); err != nil {
			return progress, false, err
		}
		return progress, true, nil
	}
	progress.Level = before.Level
	return progress, false, nil
}

func defaultDisplayName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 24 {
		return fmt.Sprintf("Freelancer-%04d", time.Now().UnixNano()%10000)
	}
	return name
}

// VoiceNote is a line shown on a screen in a voice's own text style, with its signature.
type VoiceNote struct {
	Text string `json:"text"`
	Sign string `json:"sign"`
}
