package service

import (
	"context"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// CombatXP is what a fight paid, for the action's response. Zero XP (cap reached, nothing landed)
// leaves Progress nil.
type CombatXP struct {
	XP       int            `json:"xp"`
	Progress *game.Progress `json:"progress,omitempty"`
	LevelUp  bool           `json:"levelUp"`
}

// payAttackXP pays the attacker for a hit on targetID's Tether Node, within today's cap. The
// attacker's player row must already be locked.
func payAttackXP(ctx context.Context, q db.Querier, attackerID, targetID, tetherID int64, landed, destroyed bool,
	settings game.Settings, now time.Time) (CombatXP, error) {
	paid, err := store.PaidXPAgainst(ctx, q, attackerID, targetID, game.XPActionAttack, game.XPDayStart(settings, now))
	if err != nil {
		return CombatXP{}, err
	}
	return payCombatXP(ctx, q, attackerID, targetID, tetherID, game.XPActionAttack,
		game.AttackXP(settings, landed, destroyed, paid), settings)
}

// payInterceptXP pays the defender for intercepting attackerID's drone, within today's cap.
func payInterceptXP(ctx context.Context, q db.Querier, defenderID, attackerID, droneID int64,
	settings game.Settings, now time.Time) (CombatXP, error) {
	paid, err := store.PaidXPAgainst(ctx, q, defenderID, attackerID, game.XPActionIntercept, game.XPDayStart(settings, now))
	if err != nil {
		return CombatXP{}, err
	}
	return payCombatXP(ctx, q, defenderID, attackerID, droneID, game.XPActionIntercept,
		game.InterceptXP(settings, paid), settings)
}

func payCombatXP(ctx context.Context, q db.Querier, playerID, targetID, refID int64, action string, xp int,
	settings game.Settings) (CombatXP, error) {
	if xp <= 0 {
		return CombatXP{}, nil
	}
	progress, levelUp, err := awardXPAgainst(ctx, q, playerID, action, xp, refID, targetID, settings)
	if err != nil {
		return CombatXP{}, err
	}
	return CombatXP{XP: xp, Progress: &progress, LevelUp: levelUp}, nil
}

// lockPlayerPair locks two players lowest id first, the order every fight uses.
func lockPlayerPair(ctx context.Context, q db.Querier, a, b int64) error {
	first, second := min(a, b), max(a, b)
	if _, err := store.GetPlayerForUpdate(ctx, q, first); err != nil {
		return err
	}
	if second != first {
		if _, err := store.GetPlayerForUpdate(ctx, q, second); err != nil {
			return err
		}
	}
	return nil
}
