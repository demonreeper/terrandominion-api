package service

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Friends (owner, 2026-11-11): find a player by callsign, by their whole email, or from the faction
// branch (their profile), send a request, and once they accept both see each other in FRIENDS with
// the name each typed for friends. The Google/Apple name never leaves the server; an email is only
// matched whole and never shown back.

const (
	friendsFound = 20
	dayLength    = 24 * time.Hour
)

// FriendsView is GET /v1/friends: requests to answer first, then friends, then requests sent, and the
// player's own friend settings.
type FriendsView struct {
	Incoming []store.FriendRow    `json:"incoming"`
	Friends  []store.FriendRow    `json:"friends"`
	Outgoing []store.FriendRow    `json:"outgoing"`
	Settings store.FriendSettings `json:"settings"`
}

func (s *Service) Friends(ctx context.Context, playerID int64) (FriendsView, error) {
	view := FriendsView{Incoming: []store.FriendRow{}, Friends: []store.FriendRow{}, Outgoing: []store.FriendRow{}}
	rows, err := store.Friends(ctx, s.db, playerID)
	if err != nil {
		return view, err
	}
	for _, r := range rows {
		switch r.Status {
		case "friend":
			view.Friends = append(view.Friends, r)
		case "incoming":
			view.Incoming = append(view.Incoming, r)
		default:
			view.Outgoing = append(view.Outgoing, r)
		}
	}
	sort.SliceStable(view.Incoming, func(i, j int) bool { return view.Incoming[i].Since.After(view.Incoming[j].Since) })
	view.Settings, err = store.FriendSettingsOf(ctx, s.db, playerID)
	return view, err
}

// FindPlayers is the friends search. Text with an @ is an email: only the whole address matches,
// only players who allow it, and only EmailSearchesPerDay a day. Anything else is a piece of a
// callsign or a share code.
func (s *Service) FindPlayers(ctx context.Context, playerID int64, typed string) ([]store.FoundFriend, error) {
	typed = strings.TrimSpace(typed)
	if game.IsEmailSearch(typed) {
		email := game.NormaliseEmail(typed)
		if len(email) < 5 || len(email) > 254 || strings.Count(email, "@") != 1 {
			return nil, apperr.BadRequest.Say("Type the whole email.")
		}
		settings := s.Settings(ctx)
		var found []store.FoundFriend
		err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
			if _, err := store.GetPlayerForUpdate(ctx, tx, playerID); err != nil {
				return err
			}
			n, err := store.EmailSearchesSince(ctx, tx, playerID, s.now().Add(-dayLength))
			if err != nil {
				return err
			}
			if n >= settings.EmailSearchesPerDay {
				return apperr.SearchLimit
			}
			if err := store.RecordEmailSearch(ctx, tx, playerID); err != nil {
				return err
			}
			found, err = store.FindPlayerByEmail(ctx, tx, playerID, email)
			return err
		})
		return found, err
	}
	q, ok := game.CleanPlayerSearch(typed)
	if !ok {
		return nil, apperr.BadRequest.Say("Type at least 2 characters.")
	}
	return store.FindPlayersByCallsign(ctx, s.db, playerID, q, friendsFound)
}

// RequestFriend sends a request to the player behind a share code. If they had already asked this
// player, it accepts theirs instead.
func (s *Service) RequestFriend(ctx context.Context, playerID int64, code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if !game.ValidPublicCode(code) {
		return "", apperr.NotFound
	}
	settings := s.Settings(ctx)
	now := s.now()
	status := ""
	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		me, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if err != nil {
			return err
		}
		target, err := store.FriendTargetByCode(ctx, tx, code)
		if errors.Is(err, store.ErrNotFound) || (err == nil && !target.Active) {
			return apperr.NotFound
		}
		if err != nil {
			return err
		}
		low, high := game.FriendPair(playerID, target.ID)
		existing, err := store.FriendshipForUpdate(ctx, tx, low, high)
		exists := err == nil
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		// They asked first: this is a yes.
		if exists && existing.Status == "pending" && existing.RequestedBy == target.ID {
			status = "friend"
			return s.acceptFriend(ctx, tx, playerID, me.Name, target.ID, low, high)
		}
		mine, err := store.FriendCount(ctx, tx, playerID)
		if err != nil {
			return err
		}
		theirs, err := store.FriendCount(ctx, tx, target.ID)
		if err != nil {
			return err
		}
		sent, err := store.FriendRequestsSince(ctx, tx, playerID, now.Add(-dayLength))
		if err != nil {
			return err
		}
		why := game.CheckFriendRequest(settings, game.FriendRequest{
			Self: target.ID == playerID, Exists: exists, TargetAccepts: target.AcceptFriends,
			TargetPlayer: target.IsPlayer && me.Type == "player", MineCount: mine, TheirsCount: theirs, SentToday: sent,
		})
		if why != game.FriendOK {
			return apperr.FriendRefused.Say("%s", friendRefusal(why))
		}
		if err := store.InsertFriendRequest(ctx, tx, low, high, playerID); err != nil {
			return err
		}
		status = "outgoing"
		return store.InsertNotice(ctx, tx, store.NewNotice{
			PlayerID: target.ID, ActorID: playerID, Kind: "friend_request",
			Body: me.Name + " wants to be friends.", PushState: game.NoticePushState("friend_request"),
		})
	})
	return status, err
}

func friendRefusal(why string) string {
	switch why {
	case game.FriendSelf:
		return "That's you."
	case game.FriendAlready:
		return "You are already friends, or a request is waiting."
	case game.FriendClosed:
		return "That player isn't taking friend requests."
	case game.FriendFull:
		return "One of you has as many friends as the game allows."
	case game.FriendTooMany:
		return "That is enough friend requests for today."
	}
	return "That player can't be added."
}

func (s *Service) acceptFriend(ctx context.Context, tx *sql.Tx, playerID int64, myName string, otherID, low, high int64) error {
	if err := store.AcceptFriendship(ctx, tx, low, high, s.now()); err != nil {
		return err
	}
	return store.InsertNotice(ctx, tx, store.NewNotice{
		PlayerID: otherID, ActorID: playerID, Kind: "friend_accepted",
		Body: myName + " accepted your friend request.", PushState: game.NoticePushState("friend_accepted"),
	})
}

// AnswerFriend accepts or declines a request from the player behind a code.
func (s *Service) AnswerFriend(ctx context.Context, playerID int64, code string, accept bool) error {
	otherID, err := playerIDByCode(ctx, s.db, code)
	if err != nil {
		return err
	}
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		me, err := store.GetPlayerForUpdate(ctx, tx, playerID)
		if err != nil {
			return err
		}
		low, high := game.FriendPair(playerID, otherID)
		f, err := store.FriendshipForUpdate(ctx, tx, low, high)
		if errors.Is(err, store.ErrNotFound) || (err == nil && (f.Status != "pending" || f.RequestedBy != otherID)) {
			return apperr.FriendNoRequest
		}
		if err != nil {
			return err
		}
		if !accept {
			return store.DeleteFriendship(ctx, tx, low, high)
		}
		return s.acceptFriend(ctx, tx, playerID, me.Name, otherID, low, high)
	})
}

// RemoveFriend ends a friendship, or takes back a request sent. Nobody is told.
func (s *Service) RemoveFriend(ctx context.Context, playerID int64, code string) error {
	otherID, err := playerIDByCode(ctx, s.db, code)
	if err != nil {
		return err
	}
	low, high := game.FriendPair(playerID, otherID)
	return store.DeleteFriendship(ctx, s.db, low, high)
}

// SetFriendSettings saves the name a player shows friends and who may find them.
func (s *Service) SetFriendSettings(ctx context.Context, playerID int64, in store.FriendSettings) (store.FriendSettings, error) {
	blocked, err := store.BlockedCallsignWords(ctx, s.db)
	if err != nil {
		return in, err
	}
	name, ok := game.CleanFriendName(in.FriendName, blocked)
	if !ok {
		return in, apperr.FriendNameBad
	}
	in.FriendName = name
	if err := store.SetFriendSettings(ctx, s.db, playerID, in); err != nil {
		return in, err
	}
	return in, nil
}
