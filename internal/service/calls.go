package service

import (
	"context"
	"database/sql"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// The powers' calls and written messages (docs/lore-correction-plan.md). One power speaks to a
// player: the one that matches what they mostly do. Nothing here tells players the six are one.

const (
	callPushBatch = 20
	callLogLimit  = 30
)

// scorePlaystyle counts an action toward the powers it belongs to. Call it inside the action's
// transaction, with the player locked.
func scorePlaystyle(ctx context.Context, q db.Querier, playerID int64, action, key string) error {
	for power, points := range game.PlaystylePoints(action, key) {
		if err := store.AddPlaystyle(ctx, q, playerID, power, points); err != nil {
			return err
		}
	}
	return nil
}

// scoreDistance counts the ground covered since the player's last harvest.
func scoreDistance(ctx context.Context, q db.Querier, playerID int64, metres float64) error {
	points := game.DistancePoints(metres)
	if points == 0 {
		return nil
	}
	return store.AddPlaystyle(ctx, q, playerID, game.PowerTohu, points)
}

// callingPower is the one power that speaks to this player: their highest playstyle score, a tie
// going to the power whose missions they finished most. A player with nothing to go on gets one at
// random, so somebody always calls.
func callingPower(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	scores, err := store.PlaystyleScores(ctx, q, playerID)
	if err != nil {
		return 0, err
	}
	missions, err := store.FollowCounts(ctx, q, playerID)
	if err != nil {
		return 0, err
	}
	if power, ok := game.CallingPower(scores, missions); ok {
		return power, nil
	}
	return game.Powers[rand.IntN(len(game.Powers))], nil
}

// offerPower is the power behind the level 19 offer, and so the one that invites at the faction
// level: the same power both times, even if the player's habits changed in between.
func offerPower(ctx context.Context, q db.Querier, playerID int64) (int, error) {
	caller, err := store.CallerOf(ctx, q, playerID, game.CallLevel19)
	if err != nil || caller != 0 {
		return caller, err
	}
	return callingPower(ctx, q, playerID)
}

// startCalls sets off the calls an XP gain earns a freelancer: the path call halfway through
// CallPathLevel, and halfway through the level before the faction level the offer, with its repeat
// call and written message already scheduled. Call it with the player locked.
func startCalls(ctx context.Context, q db.Querier, playerID int64, beforeXP, afterXP int64, settings game.Settings,
	now time.Time) error {
	path, offer := game.CallsDue(beforeXP, afterXP, settings)
	if !path && !offer {
		return nil
	}
	membership, err := store.MembershipOf(ctx, q, playerID)
	if err != nil || membership != nil {
		return err
	}
	power, err := callingPower(ctx, q, playerID)
	if err != nil {
		return err
	}
	if path {
		if _, err := store.InsertCall(ctx, q, playerID, power, game.CallLevel10,
			game.PowerLine(power, game.CallLevel10), now); err != nil {
			return err
		}
	}
	if !offer {
		return nil
	}
	fresh, err := store.InsertCall(ctx, q, playerID, power, game.CallLevel19, game.PowerLine(power, game.CallLevel19), now)
	if err != nil || !fresh {
		return err
	}
	repeatAt, messageAt := game.OfferSchedule(now, settings)
	if _, err := store.InsertCall(ctx, q, playerID, power, game.CallLevel19Repeat,
		game.PowerLine(power, game.CallLevel19Repeat), repeatAt); err != nil {
		return err
	}
	return store.InsertMessage(ctx, q, playerID, power, game.MessageLevel19,
		game.PowerLine(power, game.MessageLevel19), messageAt)
}

// CallsView is GET /v1/calls: the calls waiting for the player and their written messages.
type CallsView struct {
	Calls []store.Call `json:"calls"`
	// MissedCalls are the calls the player rejected and has not played (for builds before CallLog).
	MissedCalls []store.Call `json:"missedCalls"`
	// CallLog is every call the player answered or rejected, latest first, for ALERTS.
	CallLog  []store.Call    `json:"callLog"`
	Messages []store.Message `json:"messages"`
	Unread   int             `json:"unread"`
}

// Calls lists the calls that have come due and the messages that have arrived. Fetching a call is
// what delivers it: after this no push follows for it.
func (s *Service) Calls(ctx context.Context, playerID int64) (CallsView, error) {
	settings := s.Settings(ctx)
	now := s.now()
	out := CallsView{}
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		calls, err := store.DueCalls(ctx, tx, playerID, now)
		if err != nil {
			return err
		}
		for i := range calls {
			calls[i].Voice = game.PowerVoice(calls[i].FactionID)
			calls[i].Audio = game.CallAudioKey(calls[i].Kind)
			calls[i].Missed = game.CallMissed(calls[i].DueAt, now, settings)
		}
		out.Calls = calls
		if len(calls) > 0 {
			if err := store.MarkCallsDelivered(ctx, tx, playerID, now); err != nil {
				return err
			}
		}
		log, err := store.CallLog(ctx, tx, playerID, callLogLimit)
		if err != nil {
			return err
		}
		out.MissedCalls = []store.Call{}
		for i := range log {
			log[i].Voice = game.PowerVoice(log[i].FactionID)
			log[i].Audio = game.CallAudioKey(log[i].Kind)
			log[i].Missed = log[i].Rejected
			if log[i].Rejected && !log[i].Played {
				out.MissedCalls = append(out.MissedCalls, log[i])
			}
		}
		out.CallLog = log
		out.Messages, err = store.Messages(ctx, tx, playerID, now, 50)
		if err != nil {
			return err
		}
		for _, m := range out.Messages {
			if !m.Read {
				out.Unread++
			}
		}
		return nil
	})
	return out, err
}

// PlayCall marks a call heard: it never rings again. Played from ALERTS, a rejected call is spent
// after one play and an answered call after one replay.
func (s *Service) PlayCall(ctx context.Context, playerID, callID int64) error {
	done, err := store.MarkCallPlayed(ctx, s.db, playerID, callID, s.now())
	if err != nil {
		return err
	}
	if !done {
		return apperr.CallGone
	}
	return nil
}

// RejectCall turns down a ringing call: it stops ringing and waits in the missed calls, to be
// played once. The power's repeat call and message still come as scheduled.
func (s *Service) RejectCall(ctx context.Context, playerID, callID int64) error {
	done, err := store.MarkCallRejected(ctx, s.db, playerID, callID, s.now())
	if err != nil {
		return err
	}
	if !done {
		return apperr.CallGone
	}
	return nil
}

// ReadMessage clears a message's unread mark.
func (s *Service) ReadMessage(ctx context.Context, playerID, messageID int64) error {
	done, err := store.MarkMessageRead(ctx, s.db, playerID, messageID, s.now())
	if err != nil {
		return err
	}
	if !done {
		return apperr.MessageGone
	}
	return nil
}

// pushUnansweredCalls writes a "call" notice into the push outbox for every call that came due and
// that the app has not fetched within CallPushDelaySeconds: the app is closed.
func (s *Service) pushUnansweredCalls(ctx context.Context, settings game.Settings) {
	now := s.now()
	cutoff := now.Add(-time.Duration(settings.CallPushDelaySeconds) * time.Second)
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		calls, err := store.ClaimUnansweredCalls(ctx, tx, cutoff, callPushBatch)
		if err != nil {
			return err
		}
		for _, call := range calls {
			if err := notify(ctx, tx, store.NewNotice{
				PlayerID: call.PlayerID, Kind: "call", Body: game.NoticeCall(call.Persona),
			}); err != nil {
				return err
			}
			if err := store.MarkCallPushed(ctx, tx, call.ID, now); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		slog.Error("E1900 call push failed", "err", err)
	}
}
