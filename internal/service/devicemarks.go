package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/auth"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// The device record (docs/freelancer-path-plan.md phase 2): a pledge and a ban stick to the phone
// and the sign-in, so a new account can't undo them.

// signinHash is a sign-in's salted hash, as the device record keeps it.
func (s *Service) signinHash(provider, subject string) []byte {
	return auth.HashDevice("signin:"+provider+":"+subject, s.cfg.DevicePepper)
}

// playerHashes is every device and sign-in a player has used, hashed.
func (s *Service) playerHashes(ctx context.Context, q db.Querier, playerID int64) (devices, signins [][]byte, err error) {
	if devices, err = store.PlayerDeviceHashes(ctx, q, playerID); err != nil {
		return nil, nil, err
	}
	keys, err := store.PlayerSignInKeys(ctx, q, playerID)
	if err != nil {
		return nil, nil, err
	}
	for _, k := range keys {
		signins = append(signins, s.signinHash(k.Provider, k.Subject))
	}
	return devices, signins, nil
}

// markPledge binds a member's faction to every device and sign-in they use.
func (s *Service) markPledge(ctx context.Context, q db.Querier, playerID int64, factionCode string) error {
	devices, signins, err := s.playerHashes(ctx, q, playerID)
	if err != nil {
		return err
	}
	for kind, hashes := range map[string][][]byte{game.MarkDevice: devices, game.MarkSignIn: signins} {
		for _, hash := range hashes {
			if err := store.MarkPledge(ctx, q, kind, hash, factionCode, game.AppealCode(kind, hash)); err != nil {
				return err
			}
		}
	}
	return store.SetBoundFaction(ctx, q, playerID, "")
}

// markBan puts a ban on every device and sign-in a banned player used.
func (s *Service) markBan(ctx context.Context, q db.Querier, playerID, sanctionID int64) error {
	devices, signins, err := s.playerHashes(ctx, q, playerID)
	if err != nil {
		return err
	}
	for kind, hashes := range map[string][][]byte{game.MarkDevice: devices, game.MarkSignIn: signins} {
		for _, hash := range hashes {
			if err := store.MarkBan(ctx, q, kind, hash, sanctionID, game.AppealCode(kind, hash)); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkMarks reads the device record at sign-in: a ban on the phone or the sign-in refuses it
// (whether the account is new or old), and a pledge binds an account that hasn't pledged.
func (s *Service) checkMarks(ctx context.Context, q db.Querier, deviceHash, signinHash []byte) (game.MarkVerdict, error) {
	marks, err := store.MarksFor(ctx, q, deviceHash, signinHash)
	if err != nil {
		return game.MarkVerdict{}, err
	}
	found := make([]game.Mark, 0, len(marks))
	for _, m := range marks {
		found = append(found, game.Mark{Kind: m.Kind, FactionCode: m.FactionCode, Banned: m.Banned, AppealCode: m.AppealCode})
	}
	verdict := game.JudgeMarks(found)
	if verdict.Banned {
		return verdict, apperr.DeviceBanned.WithData(map[string]any{
			"appealCode": verdict.AppealCode, "appealUrl": game.AppealMailto(verdict.AppealCode),
		})
	}
	return verdict, nil
}

// applyMarks runs after sign-in has found the player: a member's pledge follows them to this phone
// and sign-in; an unpledged account on a marked phone or sign-in is bound to that faction.
func (s *Service) applyMarks(ctx context.Context, q db.Querier, playerID int64, verdict game.MarkVerdict,
	deviceHash, signinHash []byte) error {
	membership, err := store.MembershipOf(ctx, q, playerID)
	if err != nil {
		return err
	}
	if membership != nil {
		for kind, hash := range map[string][]byte{game.MarkDevice: deviceHash, game.MarkSignIn: signinHash} {
			if err := store.MarkPledge(ctx, q, kind, hash, membership.FactionCode, game.AppealCode(kind, hash)); err != nil {
				return err
			}
		}
		return nil
	}
	if verdict.BoundTo == "" {
		return nil
	}
	bound, err := store.BoundFaction(ctx, q, playerID)
	if err != nil || bound != "" {
		return err
	}
	return store.SetBoundFaction(ctx, q, playerID, verdict.BoundTo)
}

// releaseDeviceMarks forgets a released player's faction on their devices and sign-ins.
func releaseDeviceMarks(ctx context.Context, q db.Querier, s *Service, playerID int64) error {
	devices, signins, err := s.playerHashes(ctx, q, playerID)
	if err != nil {
		return err
	}
	if err := store.ForgetPledgeMarks(ctx, q, devices, signins); err != nil {
		return err
	}
	return store.SetBoundFaction(ctx, q, playerID, "")
}

// boundAppeal is the appeal code shown on the "Your pledge stands" screen: the mark on this player's
// device or sign-in that holds their faction.
func (s *Service) boundAppeal(ctx context.Context, q db.Querier, playerID int64, factionCode string) (string, error) {
	devices, signins, err := s.playerHashes(ctx, q, playerID)
	if err != nil {
		return "", err
	}
	marks, err := store.MarksOf(ctx, q, devices, signins)
	if err != nil {
		return "", err
	}
	for _, kind := range []string{game.MarkDevice, game.MarkSignIn} {
		for _, m := range marks {
			if m.Kind == kind && m.FactionCode == factionCode {
				return m.AppealCode, nil
			}
		}
	}
	return "", nil
}

// AdminMark finds a mark by its appeal code (an appeal from someone who can't sign in at all).
func (s *Service) AdminMark(ctx context.Context, code string) (store.DeviceMark, error) {
	mark, err := store.MarkByAppeal(ctx, s.db, strings.ToUpper(strings.TrimSpace(code)))
	if errors.Is(err, store.ErrNotFound) {
		return mark, apperr.NotFound.With("appeal code %q", code)
	}
	return mark, err
}

// AdminClearMark removes a whole mark after an appeal (proof checked by the admin). A reason is
// required and it is logged; the proof itself is never stored here.
func (s *Service) AdminClearMark(ctx context.Context, adminID int64, code, reason string) error {
	reason = strings.TrimSpace(reason)
	if game.CheckSanction(s.Settings(ctx), game.SanctionWarn, 0, reason) != game.SanctionOK {
		return apperr.AdjustBad.Say("Give a reason (3 to 500 characters).")
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		mark, err := store.MarkByAppeal(ctx, tx, code)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.NotFound.With("appeal code %q", code)
		}
		if err != nil {
			return err
		}
		if err := store.DeleteMark(ctx, tx, mark.ID); err != nil {
			return err
		}
		detail, _ := json.Marshal(map[string]any{"kind": mark.Kind, "faction": mark.FactionCode, "banned": mark.Banned, "reason": reason})
		return store.LogAdminAction(ctx, tx, adminID, "mark.clear", fmt.Sprint(code), string(detail))
	})
}
