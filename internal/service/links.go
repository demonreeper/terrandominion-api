package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// LinkResult is the player's sign-ins after a link or unlink.
type LinkResult struct {
	Linked []string `json:"linked"`
}

// LinkGoogle adds a Google sign-in to the signed-in player, so either sign-in opens this account
// (an iPhone player signed in with Apple can then play on Android). The token is checked exactly
// as at sign-in; the nonce is required, since every app that links sends one.
func (s *Service) LinkGoogle(ctx context.Context, playerID int64, idToken, nonce string) (LinkResult, error) {
	if nonce == "" {
		return LinkResult{}, apperr.GoogleTokenInvalid.With("nonce is required")
	}
	identity, err := s.google.Verify(ctx, idToken)
	if err != nil {
		return LinkResult{}, apperr.GoogleTokenInvalid.With("%v", err)
	}
	if identity.Nonce != nonce {
		return LinkResult{}, apperr.GoogleTokenInvalid.With("nonce does not match")
	}
	return s.link(ctx, playerID, "google", identity.Subject, identity.Email)
}

// LinkApple adds a Sign in with Apple to the signed-in player. authorizationCode is accepted but
// not used yet (P3: kept for revoking the grant when the account is deleted).
func (s *Service) LinkApple(ctx context.Context, playerID int64, identityToken, authorizationCode, rawNonce string) (LinkResult, error) {
	_ = authorizationCode
	identity, err := s.apple.Verify(ctx, identityToken, rawNonce)
	if err != nil {
		return LinkResult{}, apperr.AppleTokenInvalid.With("%v", err)
	}
	return s.link(ctx, playerID, "apple", identity.Subject, identity.Email)
}

func (s *Service) link(ctx context.Context, playerID int64, provider, subject, email string) (LinkResult, error) {
	var result LinkResult
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		// The player's row is the lock: two links or a link and an unlink for one player run in turn.
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return apperr.Unauthorized
			}
			return err
		}
		linked, err := store.PlayerIdentities(ctx, tx, playerID)
		if err != nil {
			return err
		}
		owner, err := store.FindIdentity(ctx, tx, provider, subject)
		found := err == nil
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}

		switch game.DecideLink(found, owner, playerID, provider, linked) {
		case game.LinkTaken:
			return apperr.IdentityTaken.Say("This %s account already has its own game account.", providerName(provider)).
				With("%s sign-in belongs to player %d", provider, owner)
		case game.LinkProviderUsed:
			return apperr.ProviderLinked.Say("A different %s account is already linked. Unlink it first.", providerName(provider)).
				With("player already has a different %s sign-in", provider)
		case game.LinkNew:
			if err := store.LinkIdentity(ctx, tx, provider, subject, playerID, email); err != nil {
				return err
			}
			if linked, err = store.PlayerIdentities(ctx, tx, playerID); err != nil {
				return err
			}
		case game.LinkAlready:
		}
		result.Linked = linked
		return nil
	})
	return result, err
}

func providerName(provider string) string {
	switch provider {
	case "google":
		return "Google"
	case "apple":
		return "Apple"
	}
	return provider
}

// Unlink removes one of the player's sign-ins. The last one can't be removed, so nobody locks
// themselves out. (Once P3 exists, unlinking Apple also revokes the Apple grant.)
func (s *Service) Unlink(ctx context.Context, playerID int64, provider string) (LinkResult, error) {
	if !game.IsLinkable(provider) {
		return LinkResult{}, apperr.BadRequest.With("unknown sign-in %q", provider)
	}
	var result LinkResult
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return apperr.Unauthorized
			}
			return err
		}
		linked, err := store.PlayerIdentities(ctx, tx, playerID)
		if err != nil {
			return err
		}
		switch game.DecideUnlink(linked, provider) {
		case game.UnlinkNotLinked:
			return apperr.NotFound.With("no %s sign-in on this account", provider)
		case game.UnlinkLast:
			return apperr.LastSignIn
		}
		if err := store.UnlinkIdentity(ctx, tx, playerID, provider); err != nil {
			return fmt.Errorf("unlink %s: %w", provider, err)
		}
		result.Linked, err = store.PlayerIdentities(ctx, tx, playerID)
		return err
	})
	return result, err
}
