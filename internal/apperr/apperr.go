// Package apperr holds the numbered errors the API returns. Every error has a stable code that
// is sent to the client and written to the log as "E<code>", so a player's report or a log line
// can be traced straight to the place that raised it. Codes are never reused.
//
//	1000-1099 request and generic
//	1100-1199 auth and accounts
//	1200-1299 nodes and harvesting
//	1300-1399 factions
//	1400-1499 bases
//	1500-1599 gear
//	1600-1699 tether nodes
//	1900-1999 server
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
	Detail  string `json:"-"` // logged, never sent to the client
}

func (e *Error) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("E%d %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("E%d %s", e.Code, e.Message)
}

// With returns a copy carrying internal detail for the log.
func (e *Error) With(format string, args ...any) *Error {
	c := *e
	c.Detail = fmt.Sprintf(format, args...)
	return &c
}

// As unwraps err into an *Error, or wraps an unknown error as Internal.
func As(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal.With("%v", err)
}

func def(code, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

var (
	BadRequest   = def(1000, http.StatusBadRequest, "Bad request.")
	Unauthorized = def(1001, http.StatusUnauthorized, "Sign in again.")
	Forbidden    = def(1002, http.StatusForbidden, "Not allowed.")
	NotFound     = def(1003, http.StatusNotFound, "Not found.")
	RateLimited  = def(1004, http.StatusTooManyRequests, "Too many requests. Slow down.")
	TooLarge     = def(1005, http.StatusRequestEntityTooLarge, "Request too large.")

	GoogleTokenInvalid = def(1100, http.StatusUnauthorized, "Google sign-in could not be verified.")
	SessionExpired     = def(1101, http.StatusUnauthorized, "Session expired. Sign in again.")
	AccountBanned      = def(1102, http.StatusForbidden, "This account is suspended.")
	AccountDeleted     = def(1103, http.StatusForbidden, "This account was deleted.")
	DeviceRequired     = def(1104, http.StatusBadRequest, "Device id is required.")

	HarvestTooFar      = def(1200, http.StatusConflict, "Move closer to the node.")
	HarvestGPSWeak     = def(1201, http.StatusConflict, "GPS signal too weak. Wait for a better fix.")
	HarvestCooldown    = def(1202, http.StatusConflict, "This node is recharging for you.")
	HarvestDepleted    = def(1203, http.StatusConflict, "This node is depleted. It will regrow.")
	HarvestCarryFull   = def(1204, http.StatusConflict, "You cannot carry any more.")
	HarvestImplausible = def(1205, http.StatusConflict, "Location could not be confirmed.")
	NodeInactive       = def(1206, http.StatusNotFound, "This node is not available.")

	FactionLocked        = def(1300, http.StatusConflict, "This faction is dormant until a leader claims it.")
	FactionLevelLow      = def(1301, http.StatusConflict, "Your level is too low for that.")
	FactionMemberAlready = def(1302, http.StatusConflict, "You already belong to a faction.")
	FactionClaimed       = def(1303, http.StatusConflict, "This faction already has a leader.")
	FactionNotLocked     = def(1304, http.StatusConflict, "This faction is already active.")
	// 1305 is log-only: account cleanup skipped a faction head (succession is decided by hand).

	BaseSpacingSame  = def(1402, http.StatusConflict, "Too close to another of your faction's bases.")
	BaseSpacingOther = def(1403, http.StatusConflict, "Too close to another faction's base.")

	TetherCap      = def(1600, http.StatusConflict, "You already own the maximum number of Tether Nodes.")
	TetherNoNode   = def(1601, http.StatusConflict, "No free resource node in range. Build closer to one.")
	TetherCost     = def(1602, http.StatusConflict, "Not enough resources to do that.")
	TetherGone     = def(1603, http.StatusNotFound, "This Tether Node is gone.")
	TetherNotOwner = def(1604, http.StatusForbidden, "Only the owner can do that.")
	TetherNothing  = def(1605, http.StatusConflict, "Nothing to move right now.")
	TetherTooFar   = def(1606, http.StatusConflict, "Move closer to the Tether Node.")
	TetherNoAccess = def(1607, http.StatusForbidden, "This Tether Node belongs to another faction.")
	TetherMaxTier  = def(1608, http.StatusConflict, "This Tether Node is already fully upgraded.")
	DropGone       = def(1609, http.StatusNotFound, "This drop is gone.")
	DropTooFar     = def(1610, http.StatusConflict, "Move closer to the drop.")

	TetherCantAttack = def(1611, http.StatusForbidden, "You can't attack this Tether Node.")
	TetherTooClose   = def(1612, http.StatusConflict, "Too close to another Tether Node.")

	CraftUnknown  = def(1700, http.StatusBadRequest, "That can't be crafted.")
	GearMissing   = def(1701, http.StatusNotFound, "You don't own that gear.")
	GearNoUpgrade = def(1702, http.StatusConflict, "This gear is already at its top tier.")
	GearNotWorn   = def(1703, http.StatusConflict, "This gear doesn't need repair.")

	Internal = def(1900, http.StatusInternalServerError, "Something went wrong. Try again.")
)
