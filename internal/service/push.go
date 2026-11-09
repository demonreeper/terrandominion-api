package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/push"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

const (
	pushTickInterval = 2 * time.Second
	pushBatch        = 20
	pushBackoff      = 30 * time.Second
	maxPushToken     = 4096
)

// pushSender is the FCM client; a fake can stand in for it.
type pushSender interface {
	Send(ctx context.Context, m push.Message) (push.Result, error)
}

// notify records a notice inside the action's own transaction, so a failed action never pushes.
// tetherID is 0 when the notice is not about a Tether Node.
func notify(ctx context.Context, tx *sql.Tx, playerID, tetherID int64, kind, body string) error {
	return store.InsertNotice(ctx, tx, playerID, tetherID, kind, body, game.NoticePushState(kind))
}

// RegisterPushToken remembers this phone for the signed-in session.
func (s *Service) RegisterPushToken(ctx context.Context, playerID, sessionID int64, token, platform string) error {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > maxPushToken || (platform != "android" && platform != "ios") {
		return apperr.PushTokenBad
	}
	return store.SavePushToken(ctx, s.db, token, playerID, sessionID, platform)
}

func (s *Service) RemovePushToken(ctx context.Context, playerID, sessionID int64) error {
	return store.DeleteSessionPushTokens(ctx, s.db, sessionID, playerID)
}

func (s *Service) SetPushPrefs(ctx context.Context, playerID int64, attacks, results bool) error {
	return store.SetPushPrefs(ctx, s.db, playerID, attacks, results)
}

// AdminPushTest sends a test notification to the admin's own phones.
func (s *Service) AdminPushTest(ctx context.Context, playerID int64) (int, error) {
	if s.push == nil {
		return 0, apperr.PushNotConfigured
	}
	tokens, err := store.ActivePushTokens(ctx, s.db, playerID)
	if err != nil {
		return 0, err
	}
	if len(tokens) == 0 {
		return 0, apperr.PushNoToken
	}
	sent := 0
	for _, token := range tokens {
		result, sendErr := s.push.Send(ctx, push.Message{
			Token: token, Title: "Test push", Body: "Push notifications are working.",
			Channel: game.PushChannelResults, Data: map[string]string{"kind": "test"},
		})
		if result == push.Sent {
			sent++
			continue
		}
		slog.Warn("push test failed", "err", sendErr)
	}
	if sent == 0 {
		return 0, apperr.PushFailed
	}
	return sent, nil
}

// RunPushLoop drains the outbox until the server stops.
func (s *Service) RunPushLoop(ctx context.Context) {
	ticker := time.NewTicker(pushTickInterval)
	defer ticker.Stop()
	var backoffUntil time.Time
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.now().Before(backoffUntil) {
				continue
			}
			retry, err := s.sendPushes(ctx, s.Settings(ctx))
			if err != nil && ctx.Err() == nil {
				slog.Error("E1900 push tick failed", "err", err)
			}
			if retry {
				backoffUntil = s.now().Add(pushBackoff)
			}
		}
	}
}

// sendPushes handles one batch, and reports true when FCM asked us to wait.
func (s *Service) sendPushes(ctx context.Context, settings game.Settings) (bool, error) {
	retry := false
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		pending, err := store.ClaimPendingPushes(ctx, tx, pushBatch)
		if err != nil {
			return err
		}
		for _, notice := range pending {
			state, err := s.deliverPush(ctx, tx, notice, settings)
			if err != nil {
				return err
			}
			if state == "" {
				retry = true
				return nil
			}
			if err := store.MarkPush(ctx, tx, notice.ID, state); err != nil {
				return err
			}
		}
		return nil
	})
	return retry, err
}

// deliverPush returns the notice's final state, or "" to leave it pending and try again later.
func (s *Service) deliverPush(ctx context.Context, tx *sql.Tx, notice store.PendingPush, settings game.Settings) (string, error) {
	rule, ok := game.PushRuleFor(notice.Kind)
	if !ok || s.push == nil || settings.PushEnabled == 0 {
		return "skipped", nil
	}
	now := s.now()
	if now.Sub(notice.CreatedAt) > time.Duration(settings.PushMaxAgeMinutes)*time.Minute {
		return "skipped", nil
	}
	if (rule.Attacks() && !notice.Attacks) || (!rule.Attacks() && !notice.Results) {
		return "skipped", nil
	}
	if rule.Throttled && notice.TetherID != 0 {
		since := now.Add(-time.Duration(settings.PushAttackThrottleMinutes) * time.Minute)
		recent, err := store.PushedSince(ctx, tx, notice.PlayerID, notice.TetherID, notice.Kind, since)
		if err != nil {
			return "", err
		}
		if recent {
			return "skipped", nil
		}
	}

	tokens, err := store.ActivePushTokens(ctx, tx, notice.PlayerID)
	if err != nil {
		return "", err
	}
	if len(tokens) == 0 {
		return "skipped", nil
	}

	message := pushMessage(notice, rule)
	sent, retry := 0, false
	for _, token := range tokens {
		message.Token = token
		result, sendErr := s.push.Send(ctx, message)
		switch result {
		case push.Sent:
			sent++
		case push.BadToken:
			if err := store.DeletePushToken(ctx, tx, token); err != nil {
				return "", err
			}
		case push.Retry:
			retry = true
			slog.Warn("push will retry", "notice", notice.ID, "err", sendErr)
		default:
			slog.Error("E1900 push rejected", "notice", notice.ID, "err", sendErr)
		}
	}
	switch {
	case sent > 0:
		return "sent", nil
	case retry:
		return "", nil
	default:
		return "failed", nil
	}
}

func pushMessage(notice store.PendingPush, rule game.PushRule) push.Message {
	data := map[string]string{"kind": notice.Kind, "noticeId": strconv.FormatInt(notice.ID, 10)}
	group := ""
	if notice.TetherID != 0 {
		data["tnId"] = strconv.FormatInt(notice.TetherID, 10)
		// Per kind: a later "under attack" must not wipe out a spike warning the owner can act on.
		group = fmt.Sprintf("tn-%d-%s", notice.TetherID, notice.Kind)
	}
	return push.Message{
		Title: rule.Title, Body: notice.Body, Channel: rule.Channel,
		High: rule.Channel == game.PushChannelAlerts, Group: group, Data: data,
	}
}
