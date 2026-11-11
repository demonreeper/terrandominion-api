package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Walk together, W1 (docs/walk-together-plan.md, approved 2026-11-11). Two to four accepted friends standing
// together form a signal link; while it holds and they keep walking, each gets the link bonus on scan range,
// node harvests, map drop pickups and harvest XP, added to any faction perk. The server checks the phones on
// every heartbeat: fresh accurate fixes, range after accuracy, fixes close in time, different devices, and the
// normal spoof checks (the routes are wrapped in located). Spoof-exempt testers skip the walk's own fake-flag
// check. Nobody's position is ever shown except to the members of their own link, while linked.

// WalkFixIn is the fix a walk request carries.
type WalkFixIn struct {
	Lat, Lon  float64
	AccuracyM float64
	Mock      bool
	FixAt     *time.Time
}

func (s *Service) walkFix(in WalkFixIn) game.WalkFix {
	at := s.now()
	if in.FixAt != nil {
		at = *in.FixAt
	}
	return game.WalkFix{Lat: in.Lat, Lon: in.Lon, AccuracyM: in.AccuracyM, FixAt: at, Mock: in.Mock}
}

func walkPoint(f game.WalkFix) store.WalkPoint {
	return store.WalkPoint{Lat: f.Lat, Lon: f.Lon, AccuracyM: f.AccuracyM, FixAt: f.FixAt}
}

func fromPoint(p store.WalkPoint) game.WalkFix {
	return game.WalkFix{Lat: p.Lat, Lon: p.Lon, AccuracyM: p.AccuracyM, FixAt: p.FixAt}
}

// WalkMemberView is one member as the others see them: the callsign only (never a friend name), and their
// position, which is shown only to the members of the same link, only while linked.
type WalkMemberView struct {
	Player store.PlayerRef `json:"player"`
	Me     bool            `json:"me"`
	Lat    *float64        `json:"lat,omitempty"`
	Lon    *float64        `json:"lon,omitempty"`
	// OutOfRange: this member is drifting away (their grace time is running).
	OutOfRange bool `json:"outOfRange"`
}

// WalkLinkView is the player's link: who is in it, whether their bonus is on (and why not), the numbers for
// this size, how often to beat, and the bonus time left today.
type WalkLinkView struct {
	Size             int              `json:"size"`
	Members          []WalkMemberView `json:"members"`
	BonusActive      bool             `json:"bonusActive"`
	PausedReason     string           `json:"pausedReason,omitempty"`
	Bonus            game.WalkPcts    `json:"bonus"`
	HeartbeatSec     int              `json:"heartbeatSec"`
	BonusSecondsLeft int              `json:"bonusSecondsLeft"`
	WalkedM          float64          `json:"walkedM"`
	// W2 Bluetooth proof: the token this phone advertises now, the other members' tokens to listen for (current
	// and previous), and whether the bonus needs Bluetooth.
	BleToken    string   `json:"bleToken"`
	BleListen   []string `json:"bleListen"`
	BleRequired bool     `json:"bleRequired"`
	// X1: your scan range while linked (the group's best scan perk and the link's percent) and the shared perk's
	// tag ("x2 SĒMA (LINKED)", empty when your own is the best).
	ScanRangeM float64 `json:"scanRangeM"`
	ScanTag    string  `json:"scanTag,omitempty"`
}

// WalkBeatIn is what a heartbeat carries besides the fix: whether Bluetooth is on (and allowed) on this phone,
// and the tokens it heard in the last WalkLinkBleHeardSec.
type WalkBeatIn struct {
	BleOn    bool
	BleHeard []string
}

// newBleToken is 8 random bytes, as 16 hex characters: never tied to the device or the player.
func newBleToken() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// cleanHeard keeps at most 16 well-formed tokens from a phone's report.
func cleanHeard(heard []string) []string {
	out := make([]string, 0, len(heard))
	for _, t := range heard {
		t = strings.ToLower(strings.TrimSpace(t))
		if len(t) != 16 {
			continue
		}
		if _, err := hex.DecodeString(t); err != nil {
			continue
		}
		out = append(out, t)
		if len(out) == 16 {
			break
		}
	}
	return out
}

func splitHeard(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// fileWalkReport adds a walk-link signal to the player's Signal watch report (no lock): L3 twin fixes, L8 no
// Bluetooth contact and, in W3, L4 lockstep. The mod queue shows it with the other auto signals.
func (s *Service) fileWalkReport(ctx context.Context, tx *sql.Tx, playerID int64, kind, detail string, now time.Time) error {
	signal := store.SpoofSignal{Kind: kind, Detail: detail, At: now}
	reportID, context, err := store.OpenSpoofReport(ctx, tx, game.SignalWatchID, playerID)
	if err != nil {
		return err
	}
	if reportID == 0 {
		_, err = store.InsertReport(ctx, tx, game.SignalWatchID, playerID, game.ReportReasonSpoof, kind, "",
			store.SpoofReportContext{Auto: true, Signals: []store.SpoofSignal{signal}})
		return err
	}
	context.Auto = true
	context.Signals = append(context.Signals, signal)
	return store.UpdateReportContext(ctx, tx, reportID, kind, context)
}

// WalkPanel is the WALK TOGETHER panel: friends nearby with theirs open (callsign only, never a distance),
// asks waiting for the player, and their link if they are in one. Ended says why the link just ended (a
// heartbeat after it broke), and Who whom it was about.
type WalkPanel struct {
	Nearby []store.PlayerRef `json:"nearby"`
	Asks   []store.PlayerRef `json:"asks"`
	Link   *WalkLinkView     `json:"link"`
	Ended  string            `json:"ended,omitempty"`
	Who    *store.PlayerRef  `json:"who,omitempty"`
	// X1, on heartbeats: what any member's scan found, on every member's map until the link ends.
	SharedNodes []store.Node `json:"sharedNodes,omitempty"`
	SharedDrops []store.Drop `json:"sharedDrops,omitempty"`
}

// walkBonus is the link bonus working on the player right now (zero unless linked, moving, under the cap and
// heard from within the grace time).
func (s *Service) walkBonus(ctx context.Context, q db.Querier, playerID int64, settings game.Settings) game.WalkPcts {
	linked, size, active, seenAt, err := store.WalkStateOf(ctx, q, playerID)
	if err != nil {
		slog.Error("E1900 walk state read failed", "player", playerID, "err", err)
		return game.WalkPcts{}
	}
	return game.WalkLinkBonus(settings, game.WalkMemberState{
		Linked: linked, GroupSize: size, BonusActive: active, LastSeenAt: seenAt,
	}, s.now())
}

func (s *Service) walkEnabled(settings game.Settings) error {
	if settings.WalkLinkEnabled != 1 {
		return apperr.WalkDisabled
	}
	return nil
}

// walkLockState says whether the player is signal-locked (L2) and whether they are a spoof-exempt tester.
func (s *Service) walkLockState(ctx context.Context, q db.Querier, playerID int64) (locked, exempt bool, err error) {
	state, err := store.SpoofStateOf(ctx, q, playerID, s.now())
	if err != nil {
		return false, false, err
	}
	return state.LockedUntil != nil && !state.Exempt, state.Exempt, nil
}

// OpenWalk is "I have WALK TOGETHER open": records the panel with the fix and returns who can be linked.
func (s *Service) OpenWalk(ctx context.Context, playerID, sessionID int64, in WalkFixIn) (WalkPanel, error) {
	settings := s.Settings(ctx)
	if err := s.walkEnabled(settings); err != nil {
		return WalkPanel{}, err
	}
	now := s.now()
	device, err := store.SessionDevice(ctx, s.db, sessionID)
	if err != nil {
		return WalkPanel{}, err
	}
	_, exempt, err := s.walkLockState(ctx, s.db, playerID)
	if err != nil {
		return WalkPanel{}, err
	}
	mine := s.walkFix(in)
	if err := store.OpenWalkPanel(ctx, s.db, playerID, device, walkPoint(mine), now); err != nil {
		return WalkPanel{}, err
	}
	return s.walkPanel(ctx, playerID, device, mine, exempt, settings)
}

func (s *Service) walkPanel(ctx context.Context, playerID int64, device []byte, mine game.WalkFix, exempt bool,
	settings game.Settings) (WalkPanel, error) {
	now := s.now()
	out := WalkPanel{Nearby: []store.PlayerRef{}, Asks: []store.PlayerRef{}}
	// The LINK list: friends with the panel open, within WalkLinkFormM, on another phone. Never further:
	// it only tells what the player could see with their own eyes.
	if game.WalkFixUsable(settings, mine, now, exempt) {
		friends, err := store.FriendsWithPanelOpen(ctx, s.db, playerID, now.Add(-time.Duration(settings.WalkLinkOpenSec)*time.Second))
		if err != nil {
			return out, err
		}
		for _, f := range friends {
			_, theirExempt, err := s.walkLockState(ctx, s.db, f.PlayerID)
			if err != nil {
				return out, err
			}
			theirs := fromPoint(f.Fix)
			if !game.WalkFixUsable(settings, theirs, now, theirExempt) || bytes.Equal(f.Device, device) {
				continue
			}
			if game.WalkTogether(settings, mine, theirs, settings.WalkLinkFormM) {
				out.Nearby = append(out.Nearby, f.Player)
			}
		}
	}
	asks, err := store.WalkAsksTo(ctx, s.db, playerID, now.Add(-time.Duration(settings.WalkLinkAskSec)*time.Second))
	if err != nil {
		return out, err
	}
	out.Asks = asks
	linkID, err := store.LiveWalkLink(ctx, s.db, playerID)
	if err != nil {
		return out, err
	}
	if linkID != 0 {
		view, err := s.walkLinkView(ctx, s.db, linkID, playerID, settings)
		if err != nil {
			return out, err
		}
		out.Link = view
	}
	return out, nil
}

// AskWalk asks a friend to walk together. The friend must have their panel open and stand within
// WalkLinkFormM, on another phone, and neither may be signal-locked.
func (s *Service) AskWalk(ctx context.Context, playerID, sessionID int64, code string, in WalkFixIn) error {
	settings := s.Settings(ctx)
	if err := s.walkEnabled(settings); err != nil {
		return err
	}
	now := s.now()
	targetID, err := s.PlayerIDByCode(ctx, code)
	if err != nil {
		return err
	}
	if targetID == playerID {
		return apperr.BadRequest.With("ask yourself")
	}
	if ok, err := store.AreFriends(ctx, s.db, playerID, targetID); err != nil {
		return err
	} else if !ok {
		return apperr.WalkNotFriends
	}
	locked, exempt, err := s.walkLockState(ctx, s.db, playerID)
	if err != nil {
		return err
	}
	theirLocked, theirExempt, err := s.walkLockState(ctx, s.db, targetID)
	if err != nil {
		return err
	}
	if locked || theirLocked {
		return apperr.WalkLocked
	}
	mine := s.walkFix(in)
	if !game.WalkFixUsable(settings, mine, now, exempt) {
		return apperr.WalkWeakFix
	}
	device, err := store.SessionDevice(ctx, s.db, sessionID)
	if err != nil {
		return err
	}
	if err := store.OpenWalkPanel(ctx, s.db, playerID, device, walkPoint(mine), now); err != nil {
		return err
	}
	panel, err := store.WalkPanelOf(ctx, s.db, targetID, now.Add(-time.Duration(settings.WalkLinkOpenSec)*time.Second))
	if errors.Is(err, store.ErrNotFound) {
		return apperr.WalkTooFar
	}
	if err != nil {
		return err
	}
	theirs := fromPoint(panel.Fix)
	if !game.WalkFixUsable(settings, theirs, now, theirExempt) || !game.WalkTogether(settings, mine, theirs, settings.WalkLinkFormM) {
		return apperr.WalkTooFar
	}
	// L6: two accounts on one phone never link.
	if bytes.Equal(panel.Device, device) {
		return apperr.WalkSameDevice
	}
	if theirs, err := store.LiveWalkLink(ctx, s.db, targetID); err != nil {
		return err
	} else if theirs != 0 {
		return apperr.WalkBusy
	}
	if linkID, err := store.LiveWalkLink(ctx, s.db, playerID); err != nil {
		return err
	} else if linkID != 0 {
		members, err := store.WalkMembers(ctx, s.db, linkID)
		if err != nil {
			return err
		}
		if len(members) >= settings.WalkLinkMaxSize {
			return apperr.WalkGroupFull
		}
		for _, m := range members {
			if bytes.Equal(m.Device, panel.Device) {
				return apperr.WalkSameDevice
			}
		}
	}
	return store.InsertWalkAsk(ctx, s.db, playerID, targetID, now)
}

// AcceptWalk joins the asker's group, or forms one with them, after the proximity check.
func (s *Service) AcceptWalk(ctx context.Context, playerID, sessionID int64, askerCode string, in WalkFixIn) (WalkPanel, error) {
	settings := s.Settings(ctx)
	if err := s.walkEnabled(settings); err != nil {
		return WalkPanel{}, err
	}
	now := s.now()
	askerID, err := s.PlayerIDByCode(ctx, askerCode)
	if err != nil {
		return WalkPanel{}, err
	}
	device, err := store.SessionDevice(ctx, s.db, sessionID)
	if err != nil {
		return WalkPanel{}, err
	}
	locked, exempt, err := s.walkLockState(ctx, s.db, playerID)
	if err != nil {
		return WalkPanel{}, err
	}
	askerLocked, askerExempt, err := s.walkLockState(ctx, s.db, askerID)
	if err != nil {
		return WalkPanel{}, err
	}
	if locked || askerLocked {
		return WalkPanel{}, apperr.WalkLocked
	}
	mine := s.walkFix(in)
	if !game.WalkFixUsable(settings, mine, now, exempt) {
		return WalkPanel{}, apperr.WalkWeakFix
	}

	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, min(playerID, askerID)); err != nil {
			return err
		}
		if _, err := store.GetPlayerForUpdate(ctx, tx, max(playerID, askerID)); err != nil {
			return err
		}
		if ok, err := store.WalkAskExists(ctx, tx, askerID, playerID, now.Add(-time.Duration(settings.WalkLinkAskSec)*time.Second)); err != nil {
			return err
		} else if !ok {
			return apperr.WalkAskGone
		}
		if ok, err := store.AreFriends(ctx, tx, playerID, askerID); err != nil {
			return err
		} else if !ok {
			return apperr.WalkNotFriends
		}
		if own, err := store.LiveWalkLink(ctx, tx, playerID); err != nil {
			return err
		} else if own != 0 {
			return apperr.WalkBusy
		}

		linkID, err := store.LiveWalkLink(ctx, tx, askerID)
		if err != nil {
			return err
		}
		var askerFix *store.WalkPoint
		var askerDevice []byte
		var members []store.WalkMember
		if linkID != 0 {
			if live, err := store.LockWalkLink(ctx, tx, linkID); err != nil {
				return err
			} else if !live {
				return apperr.WalkAskGone
			}
			if members, err = store.WalkMembers(ctx, tx, linkID); err != nil {
				return err
			}
			if len(members) >= settings.WalkLinkMaxSize {
				return apperr.WalkGroupFull
			}
			for _, m := range members {
				if m.PlayerID == askerID {
					askerFix, askerDevice = m.Fix, m.Device
				}
				if bytes.Equal(m.Device, device) {
					return apperr.WalkSameDevice
				}
			}
		} else {
			panel, err := store.WalkPanelOf(ctx, tx, askerID, now.Add(-time.Duration(settings.WalkLinkOpenSec)*time.Second))
			if errors.Is(err, store.ErrNotFound) {
				return apperr.WalkTooFar
			}
			if err != nil {
				return err
			}
			askerFix, askerDevice = &panel.Fix, panel.Device
		}
		if bytes.Equal(askerDevice, device) {
			return apperr.WalkSameDevice
		}
		if askerFix == nil {
			return apperr.WalkTooFar
		}
		theirs := fromPoint(*askerFix)
		if !game.WalkFixUsable(settings, theirs, now, askerExempt) || !game.WalkTogether(settings, mine, theirs, settings.WalkLinkFormM) {
			return apperr.WalkTooFar
		}

		if linkID == 0 {
			if linkID, err = store.CreateWalkLink(ctx, tx, now); err != nil {
				return err
			}
			if err := store.AddWalkMember(ctx, tx, linkID, askerID, askerDevice, *askerFix, now); err != nil {
				return err
			}
		}
		if err := store.AddWalkMember(ctx, tx, linkID, playerID, device, walkPoint(mine), now); err != nil {
			return err
		}
		for _, id := range []int64{askerID, playerID} {
			if _, err := store.GrantTitle(ctx, tx, id, game.BadgeWalkFirstLink); err != nil {
				return err
			}
		}
		return store.DeleteWalkAsk(ctx, tx, askerID, playerID)
	})
	if err != nil {
		return WalkPanel{}, err
	}
	return s.walkPanel(ctx, playerID, device, mine, exempt, settings)
}

// WalkHeartbeat is every WalkLinkHeartbeatSec while linked: the fix is checked, the member's place in range
// decided (out of range for WalkLinkGraceSec leaves the link), the bonus decided (moving, under the daily
// cap), bonus time and metres counted. It returns the link as the phone draws it.
func (s *Service) WalkHeartbeat(ctx context.Context, playerID int64, in WalkFixIn, beat WalkBeatIn) (WalkPanel, error) {
	settings := s.Settings(ctx)
	now := s.now()
	linkID, err := store.LiveWalkLink(ctx, s.db, playerID)
	if err != nil {
		return WalkPanel{}, err
	}
	if linkID == 0 {
		return WalkPanel{}, apperr.WalkNotLinked
	}
	if settings.WalkLinkEnabled != 1 {
		if err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			return store.EndWalkLink(ctx, tx, linkID, game.WalkEndDisabled, now)
		}); err != nil {
			return WalkPanel{}, err
		}
		return WalkPanel{Nearby: []store.PlayerRef{}, Asks: []store.PlayerRef{}, Ended: game.WalkEndDisabled}, nil
	}
	_, exempt, err := s.walkLockState(ctx, s.db, playerID)
	if err != nil {
		return WalkPanel{}, err
	}
	mine := s.walkFix(in)
	usable := game.WalkFixUsable(settings, mine, now, exempt)
	ended := ""
	day, _ := game.MissionDay(settings, now)

	err = db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		// The player first (missions and badges need them locked), then the link: the same order as accept.
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			return err
		}
		live, err := store.LockWalkLink(ctx, tx, linkID)
		if err != nil {
			return err
		}
		if !live {
			ended = game.WalkEndAlone
			return nil
		}
		members, err := store.WalkMembers(ctx, tx, linkID)
		if err != nil {
			return err
		}
		me := -1
		fixes := make([]*game.WalkFix, len(members))
		for i, m := range members {
			if m.PlayerID == playerID {
				me = i
				if usable {
					f := mine
					fixes[i] = &f
				}
				continue
			}
			// A member silent past the grace time doesn't hold anyone in range.
			if m.Fix != nil && now.Sub(m.LastSeenAt) <= time.Duration(settings.WalkLinkGraceSec)*time.Second {
				f := fromPoint(*m.Fix)
				fixes[i] = &f
			}
		}
		if me < 0 {
			ended = game.WalkEndAlone
			return nil
		}
		member := members[me]
		inRange := game.WalkInRange(settings, fixes)[me]

		var outSince *time.Time
		if !inRange {
			outSince = &now
			if member.OutSince != nil {
				outSince = member.OutSince
			}
			if now.Sub(*outSince) >= time.Duration(settings.WalkLinkGraceSec)*time.Second {
				ended = game.WalkEndFellBehind
				return s.walkLeave(ctx, tx, linkID, playerID, game.WalkEndFellBehind, now)
			}
		}

		var trail []game.WalkTrailPoint
		window := time.Duration(settings.WalkLinkMoveWindowSec) * time.Second
		if usable {
			if err := store.AppendWalkTrail(ctx, tx, playerID, mine.Lat, mine.Lon, now, now.Add(-2*window)); err != nil {
				return err
			}
			points, err := store.WalkTrail(ctx, tx, playerID, now.Add(-window))
			if err != nil {
				return err
			}
			for _, p := range points {
				trail = append(trail, game.WalkTrailPoint{Lat: p.Lat, Lon: p.Lon, At: p.FixAt})
			}
		}
		moved := usable && game.WalkMoved(settings, trail, mine.Lat, mine.Lon, now)
		// L4 lockstep (W3, with G2's pattern checks): paths that match step for step make a report, once per member
		// per link. Report only; exempt testers skip it.
		if usable && !exempt && len(trail) > settings.WalkLockstepSteps {
			if err := s.checkLockstep(ctx, tx, linkID, playerID, members, trail, now, window, settings); err != nil {
				return err
			}
		}
		used, err := store.WalkBonusUsed(ctx, tx, playerID, day)
		if err != nil {
			return err
		}
		capLeft := game.WalkCapLeft(settings, used)

		// Bluetooth proof (W2): rotate the token, then confirm this member by what they heard or who heard them.
		token, prev, tokenAt := member.BleToken, member.BleTokenPrev, member.BleTokenAt
		if token == "" || tokenAt == nil || now.Sub(*tokenAt) >= time.Duration(settings.WalkLinkBleTokenSec)*time.Second {
			prev, token, tokenAt = token, newBleToken(), &now
		}
		othersTokens := map[string]bool{}
		var othersHeard [][]string
		heardWindow := time.Duration(settings.WalkLinkBleHeardSec) * time.Second
		for i, o := range members {
			if i == me {
				continue
			}
			othersTokens[o.BleToken] = true
			othersTokens[o.BleTokenPrev] = true
			if o.BleHeardAt != nil && now.Sub(*o.BleHeardAt) <= heardWindow {
				othersHeard = append(othersHeard, splitHeard(o.BleHeard))
			}
		}
		delete(othersTokens, "")
		heard := cleanHeard(beat.BleHeard)
		confirmedAt := member.BleConfirmedAt
		if beat.BleOn && game.WalkBleHeard(heard, othersTokens, []string{token, prev, member.BleToken}, othersHeard) {
			confirmedAt = &now
		}
		bleState := game.WalkBleState(settings, beat.BleOn, confirmedAt, now)

		// L3 twin fixes (exempt testers skip it): the same position and accuracy as another member, beat after
		// beat. The group's bonus stops for the rest of the link and Signal watch reports both.
		flagged, err := store.WalkLinkTwinFlagged(ctx, tx, linkID)
		if err != nil {
			return err
		}
		twinCount := 0
		var twinWith []int64
		if usable && !exempt {
			for i, o := range members {
				if i != me && o.Fix != nil && game.WalkTwin(settings, mine, fromPoint(*o.Fix)) {
					twinWith = append(twinWith, o.PlayerID)
				}
			}
			if len(twinWith) > 0 {
				twinCount = member.TwinCount + 1
			}
		}
		if twinCount >= settings.WalkLinkTwinFixes && !flagged {
			flagged = true
			if err := store.FlagWalkLinkTwins(ctx, tx, linkID); err != nil {
				return err
			}
			for _, id := range append([]int64{playerID}, twinWith...) {
				if err := s.fileWalkReport(ctx, tx, id, "walk_twin_fixes", "twin fixes in a walk link", now); err != nil {
					return err
				}
			}
		}

		// L8: GPS says together and Bluetooth is on, but nobody has heard anyone. Report once per link.
		together, reported := member.TogetherSince, member.BleReported
		switch {
		case inRange && beat.BleOn && bleState == game.WalkPausedBleConfirming:
			if together == nil {
				together = &now
			}
			if !reported && now.Sub(*together) >= time.Duration(settings.WalkLinkNoBleReportSec)*time.Second {
				reported = true
				if err := s.fileWalkReport(ctx, tx, playerID, "walk_no_bluetooth", "GPS together, no Bluetooth contact", now); err != nil {
					return err
				}
			}
		case bleState == "":
			together = nil
		}
		if err := store.UpdateWalkMemberBle(ctx, tx, linkID, playerID, store.WalkBleUpdate{
			BleOn: beat.BleOn, Token: token, Prev: prev, TokenAt: tokenAt, Heard: strings.Join(heard, ","), HeardAt: now,
			ConfirmedAt: confirmedAt, TwinCount: twinCount, TogetherSince: together, BleReported: reported,
		}); err != nil {
			return err
		}

		paused := game.WalkPaused(usable, inRange, flagged, bleState, moved, capLeft)
		active := paused == ""

		update := store.WalkMemberUpdate{SeenAt: now, OutSince: outSince, BonusActive: active, PausedReason: paused}
		if usable {
			p := walkPoint(mine)
			update.Fix = &p
		}
		// Bonus time and metres count only between two bonus-active beats.
		if active && member.BonusActive {
			update.AddBonusSec = game.WalkBonusSeconds(settings, now.Sub(member.LastSeenAt), capLeft)
			if member.Fix != nil {
				// A step longer than a brisk walk over two beats is not counted (the spoof checks judge it).
				step := game.DistanceM(member.Fix.Lat, member.Fix.Lon, mine.Lat, mine.Lon)
				if step <= float64(2*settings.WalkLinkHeartbeatSec)*4 {
					update.AddWalkedM = step
				}
			}
		}
		if err := store.UpdateWalkMember(ctx, tx, linkID, playerID, update); err != nil {
			return err
		}
		walkedBefore, err := store.WalkedTogether(ctx, tx, playerID)
		if err != nil {
			return err
		}
		if err := store.AddWalkBonus(ctx, tx, playerID, day, update.AddBonusSec, update.AddWalkedM); err != nil {
			return err
		}
		// X5 distance badges and X3 link missions count bonus-active metres only.
		for _, badge := range game.WalkMilestonesCrossed(settings, walkedBefore, walkedBefore+update.AddWalkedM) {
			if _, err := store.GrantTitle(ctx, tx, playerID, badge); err != nil {
				return err
			}
		}
		if metres := int(math.Round(update.AddWalkedM)); metres > 0 {
			return advanceMissions(ctx, tx, playerID, game.GoalWalkLinked, "", metres, settings, now)
		}
		return nil
	})
	if err != nil {
		return WalkPanel{}, err
	}
	out := WalkPanel{Nearby: []store.PlayerRef{}, Asks: []store.PlayerRef{}, Ended: ended}
	if ended != "" {
		return out, nil
	}
	view, err := s.walkLinkView(ctx, s.db, linkID, playerID, settings)
	if err != nil {
		return out, err
	}
	out.Link = view
	if out.SharedNodes, err = store.WalkFindNodes(ctx, s.db, linkID, mine.Lat, mine.Lon); err != nil {
		return out, err
	}
	if out.SharedDrops, err = store.WalkFindDrops(ctx, s.db, linkID, mine.Lat, mine.Lon); err != nil {
		return out, err
	}
	return out, nil
}

// walkLeave takes one member out; a link left with one player ends.
func (s *Service) walkLeave(ctx context.Context, tx *sql.Tx, linkID, playerID int64, reason string, now time.Time) error {
	if err := store.LeaveWalkLink(ctx, tx, linkID, playerID, reason, now); err != nil {
		return err
	}
	left, err := store.WalkLiveCount(ctx, tx, linkID)
	if err != nil {
		return err
	}
	if left < 2 {
		return store.EndWalkLink(ctx, tx, linkID, game.WalkEndAlone, now)
	}
	return nil
}

// LeaveWalk is LEAVE, and what the phone sends when the app goes to the background.
func (s *Service) LeaveWalk(ctx context.Context, playerID int64) error {
	now := s.now()
	if err := store.CloseWalkPanel(ctx, s.db, playerID); err != nil {
		return err
	}
	linkID, err := store.LiveWalkLink(ctx, s.db, playerID)
	if err != nil || linkID == 0 {
		return err
	}
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if live, err := store.LockWalkLink(ctx, tx, linkID); err != nil || !live {
			return err
		}
		return s.walkLeave(ctx, tx, linkID, playerID, game.WalkEndLeft, now)
	})
}

// endWalkFor ends the whole link the player is in (L1/L2: a member signal-locked; the link ends for
// everyone).
func (s *Service) endWalkFor(ctx context.Context, q db.Querier, playerID int64, reason string) error {
	linkID, err := store.LiveWalkLink(ctx, q, playerID)
	if err != nil || linkID == 0 {
		return err
	}
	return store.EndWalkLink(ctx, q, linkID, reason, s.now())
}

// endWalkOnAttack: no truce between friends, but attacking a linked member (their Tether Node, them, or
// later their faction's base) ends the link (owner 2026-11-11). Called in the shared attack path.
func (s *Service) endWalkOnAttack(ctx context.Context, q db.Querier, attackerID, victimID int64) error {
	if attackerID == victimID {
		return nil
	}
	mine, err := store.LiveWalkLink(ctx, q, attackerID)
	if err != nil || mine == 0 {
		return err
	}
	theirs, err := store.LiveWalkLink(ctx, q, victimID)
	if err != nil || theirs != mine {
		return err
	}
	return store.EndWalkLink(ctx, q, mine, game.WalkEndAttack, s.now())
}

// walkTick is the walk's share of the world tick: members not heard from for WalkLinkGraceSec leave
// (missed heartbeats); history past WalkLinkKeepDays and old asks and panels go.
func (s *Service) walkTick(ctx context.Context) {
	settings := s.Settings(ctx)
	now := s.now()
	silent, err := store.SilentWalkMembers(ctx, s.db, now.Add(-time.Duration(settings.WalkLinkGraceSec)*time.Second))
	if err != nil {
		if ctx.Err() == nil {
			slog.Error("E1900 walk tick failed", "err", err)
		}
		return
	}
	for _, pair := range silent {
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			if live, err := store.LockWalkLink(ctx, tx, pair[0]); err != nil || !live {
				return err
			}
			return s.walkLeave(ctx, tx, pair[0], pair[1], game.WalkEndSilent, now)
		})
		if err != nil {
			slog.Error("E1900 walk member end failed", "link", pair[0], "player", pair[1], "err", err)
		}
	}
	keep := now.AddDate(0, 0, -max(1, settings.WalkLinkKeepDays))
	asks := now.Add(-time.Duration(max(settings.WalkLinkAskSec, settings.WalkLinkOpenSec)) * time.Second * 2)
	if err := store.PurgeWalkHistory(ctx, s.db, keep, asks); err != nil && ctx.Err() == nil {
		slog.Error("E1900 walk history purge failed", "err", err)
	}
}

// walkLinkView is the link as one member's phone draws it.
func (s *Service) walkLinkView(ctx context.Context, q db.Querier, linkID, playerID int64, settings game.Settings) (*WalkLinkView, error) {
	members, err := store.WalkMembers(ctx, q, linkID)
	if err != nil {
		return nil, err
	}
	day, _ := game.MissionDay(settings, s.now())
	used, err := store.WalkBonusUsed(ctx, q, playerID, day)
	if err != nil {
		return nil, err
	}
	view := &WalkLinkView{
		Size: len(members), Members: make([]WalkMemberView, 0, len(members)),
		Bonus: game.WalkLinkPcts(settings, len(members)), HeartbeatSec: settings.WalkLinkHeartbeatSec,
		BonusSecondsLeft: game.WalkCapLeft(settings, used),
		BleListen:        []string{},
		BleRequired:      settings.WalkLinkRequireBle == 1,
	}
	if view.ScanRangeM, view.ScanTag, err = s.linkedScanRange(ctx, q, playerID, settings); err != nil {
		return nil, err
	}
	for _, m := range members {
		mv := WalkMemberView{Player: m.Player, Me: m.PlayerID == playerID, OutOfRange: m.OutSince != nil}
		if m.PlayerID == playerID {
			view.BonusActive = m.BonusActive
			view.PausedReason = m.PausedReason
			view.WalkedM = m.WalkedM
			view.BleToken = m.BleToken
		} else {
			if m.Fix != nil {
				lat, lon := round6(m.Fix.Lat), round6(m.Fix.Lon)
				mv.Lat, mv.Lon = &lat, &lon
			}
			for _, t := range []string{m.BleToken, m.BleTokenPrev} {
				if t != "" {
					view.BleListen = append(view.BleListen, t)
				}
			}
		}
		view.Members = append(view.Members, mv)
	}
	if !view.BonusActive && view.PausedReason == "" {
		view.PausedReason = game.WalkPausedKeepWalking
	}
	return view, nil
}

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

// ---- W2b extras (walk-together-plan.md X1, X2, X5) -----------------------------------------------

// groupScanPerk is the best scan perk in the player's live link (X1: only the scan perk is shared), with its tag
// ("x2 SĒMA (LINKED)"). The player's own perks and no tag when they aren't linked or nobody's beats theirs.
func (s *Service) groupScanPerk(ctx context.Context, q db.Querier, playerID int64, own game.Perks,
	settings game.Settings) (game.Perks, string, error) {
	linkID, err := store.LiveWalkLink(ctx, q, playerID)
	if err != nil || linkID == 0 {
		return own, "", err
	}
	members, err := store.WalkMembers(ctx, q, linkID)
	if err != nil {
		return own, "", err
	}
	best, tag := own, ""
	for _, m := range members {
		if m.PlayerID == playerID {
			continue
		}
		perks, lines, err := s.perksOf(ctx, q, m.PlayerID, settings)
		if err != nil {
			return own, "", err
		}
		if perks.ScanRangeMult <= best.ScanRangeMult {
			continue
		}
		best.ScanRangeMult = perks.ScanRangeMult
		best.RareRangeMult = math.Max(best.RareRangeMult, perks.RareRangeMult)
		perkTag := game.PerkTagIn(lines, game.PerkScan)
		name := ""
		if i := strings.Index(perkTag, " "); i > 0 {
			name = perkTag[i+1:]
		}
		tag = game.SharedScanTag(perks.ScanRangeMult, perkTag, name)
	}
	return best, tag, nil
}

// linkedScanRange is the player's scan range while linked, as the server scans: the gear's range with the group's
// best scan perk and the link's percent added. 0 when the player has no scanner.
func (s *Service) linkedScanRange(ctx context.Context, q db.Querier, playerID int64, settings game.Settings) (float64, string, error) {
	stats, perks, err := s.equipped(ctx, q, playerID, settings)
	if err != nil || stats.ScanRangeM <= 0 || perks.ScanRangeMult <= 0 {
		return 0, "", err
	}
	base := stats.ScanRangeM / perks.ScanRangeMult
	best, tag, err := s.groupScanPerk(ctx, q, playerID, perks, settings)
	if err != nil {
		return 0, "", err
	}
	mult := math.Max(perks.ScanRangeMult, best.ScanRangeMult)
	return math.Min(settings.NearbyMaxRadiusM, game.WalkScanRange(base, mult, s.walkBonus(ctx, q, playerID, settings).Scan)), tag, nil
}

// checkGroupNode is X2's rule at harvest time: the player's link is at least the node's size, they haven't taken
// their share, and that many members are at the node (within harvest range) and Bluetooth-confirmed.
func (s *Service) checkGroupNode(ctx context.Context, tx *sql.Tx, playerID int64, node store.Node, req HarvestRequest,
	settings game.Settings) error {
	locked := apperr.GroupNodeLocked.WithData(map[string]any{"size": node.GroupSize})
	linkID, err := store.LiveWalkLink(ctx, tx, playerID)
	if err != nil {
		return err
	}
	if linkID == 0 {
		return locked
	}
	members, err := store.WalkMembers(ctx, tx, linkID)
	if err != nil {
		return err
	}
	if len(members) < node.GroupSize {
		return locked
	}
	if taken, err := store.GroupNodeHarvested(ctx, tx, node.ID, playerID); err != nil {
		return err
	} else if taken {
		return apperr.GroupNodeTaken
	}
	now := s.now()
	grace := time.Duration(settings.WalkLinkGraceSec) * time.Second
	bleGrace := time.Duration(settings.WalkLinkBleGraceSec) * time.Second
	ready := 0
	for _, m := range members {
		lat, lon, acc := 0.0, 0.0, 0.0
		switch {
		case m.PlayerID == playerID:
			lat, lon, acc = req.Lat, req.Lon, req.AccuracyM
		case m.Fix != nil && now.Sub(m.LastSeenAt) <= grace:
			lat, lon, acc = m.Fix.Lat, m.Fix.Lon, m.Fix.AccuracyM
		default:
			continue
		}
		if game.DistanceM(lat, lon, node.Lat, node.Lon) > settings.HarvestRangeM+math.Min(acc, settings.HarvestRangeM) {
			continue
		}
		if settings.WalkLinkRequireBle == 1 && (m.BleConfirmedAt == nil || now.Sub(*m.BleConfirmedAt) > bleGrace) {
			continue
		}
		ready++
	}
	if !game.GroupNodeOpen(node.GroupSize, len(members), ready) {
		return apperr.GroupNodeGather.WithData(map[string]any{"size": node.GroupSize, "ready": ready})
	}
	return nil
}

// checkLockstep compares the player's trail with each other member's (L4) and reports both once when they match
// step for step.
func (s *Service) checkLockstep(ctx context.Context, tx *sql.Tx, linkID, playerID int64, members []store.WalkMember,
	trail []game.WalkTrailPoint, now time.Time, window time.Duration, settings game.Settings) error {
	for _, o := range members {
		if o.PlayerID == playerID {
			continue
		}
		points, err := store.WalkTrail(ctx, tx, o.PlayerID, now.Add(-window))
		if err != nil {
			return err
		}
		other := make([]game.WalkTrailPoint, 0, len(points))
		for _, p := range points {
			other = append(other, game.WalkTrailPoint{Lat: p.Lat, Lon: p.Lon, At: p.FixAt})
		}
		if !game.WalkLockstep(trail, other, settings.WalkLockstepSteps, settings.WalkLockstepM, settings.WalkLockstepPairSec) {
			continue
		}
		for _, id := range []int64{playerID, o.PlayerID} {
			done, err := store.WalkLockstepReported(ctx, tx, linkID, id)
			if err != nil {
				return err
			}
			if done {
				continue
			}
			if err := s.fileWalkReport(ctx, tx, id, "walk_lockstep", "moves in lockstep with a linked member", now); err != nil {
				return err
			}
			if err := store.MarkWalkLockstepReported(ctx, tx, linkID, id); err != nil {
				return err
			}
		}
	}
	return nil
}
