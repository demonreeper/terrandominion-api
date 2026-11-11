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

// Error is sent as {"code", "message", "data"}. Message is a sentence the app can show as it is;
// Data holds the numbers behind it (what is short, how far away), so the app can say it its own way.
type Error struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
	Status  int            `json:"-"`
	Detail  string         `json:"-"` // logged, never sent to the client
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

// WithData returns a copy carrying the facts behind the error for the client.
func (e *Error) WithData(data map[string]any) *Error {
	c := *e
	c.Data = data
	return &c
}

// Say returns a copy with a more specific message for the player.
func (e *Error) Say(format string, args ...any) *Error {
	c := *e
	c.Message = fmt.Sprintf(format, args...)
	return &c
}

// Is matches any copy of the same numbered error, so errors.Is(err, apperr.OutOfRange) works on
// one carrying data or detail.
func (e *Error) Is(target error) bool {
	var t *Error
	return errors.As(target, &t) && t.Code == e.Code
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
	AppOutdated  = def(1006, http.StatusUpgradeRequired, "This version of the game is no longer supported. Install the latest one.") // data: build, minBuild

	SettingLocked     = def(1007, http.StatusConflict, "That setting is locked.")
	SettingOutOfRange = def(1008, http.StatusBadRequest, "That value is outside the setting's range.") // data: min, max, step
	AnnouncementBad   = def(1009, http.StatusBadRequest, "An announcement is one line of up to 140 characters, a kind (info, event, warning) and 1 to 720 hours.")

	GoogleTokenInvalid = def(1100, http.StatusUnauthorized, "Google sign-in could not be verified.")
	SessionExpired     = def(1101, http.StatusUnauthorized, "Session expired. Sign in again.")
	AccountBanned      = def(1102, http.StatusForbidden, "This account has been banned.")
	AccountDeleted     = def(1103, http.StatusForbidden, "This account was deleted.")
	DeviceRequired     = def(1104, http.StatusBadRequest, "Device id is required.")
	AppleTokenInvalid  = def(1105, http.StatusUnauthorized, "Apple sign-in could not be verified.")
	IdentityTaken      = def(1106, http.StatusConflict, "That sign-in already has its own game account.")
	LastSignIn         = def(1107, http.StatusConflict, "This is your only way to sign in. Link another one before removing it.")
	AccountSuspended   = def(1112, http.StatusForbidden, "This account is suspended.") // data: until
	ReportSelf         = def(1113, http.StatusBadRequest, "You can't report yourself.")
	ReportBad          = def(1114, http.StatusBadRequest, "Pick what happened, and keep the note to 300 characters.")
	ReportOpen         = def(1115, http.StatusConflict, "You already reported this operative. A moderator will look at it.")
	ReportLimit        = def(1116, http.StatusTooManyRequests, "You have made as many reports as one day allows. Try again tomorrow.")                                // data: perDay
	SanctionBad        = def(1117, http.StatusBadRequest, "A sanction needs a kind (warn, suspend, ban), a reason, and for a suspension one of the allowed lengths.") // data: days
	SanctionNotYours   = def(1118, http.StatusForbidden, "You can't act on that account.")
	SanctionNotLive    = def(1119, http.StatusConflict, "That sanction is not running, so there is nothing to lift.")
	ReportNotDismissed = def(1120, http.StatusConflict, "Only a dismissed report can be reopened.")
	// The callsign errors carry data.error: "callsign_invalid", "callsign_taken" or "callsign_too_soon".
	CallsignInvalid = def(1109, http.StatusBadRequest, "Use 3 to 16 letters, digits, - or _, starting and ending with a letter or digit.")
	CallsignTaken   = def(1110, http.StatusConflict, "That callsign is taken or not allowed. Try another.")
	CallsignTooSoon = def(1111, http.StatusConflict, "You changed your callsign recently.") // data: changeAt, days
	ProviderLinked  = def(1108, http.StatusConflict, "A different account of that kind is already linked. Unlink it first.")

	HarvestTooFar      = def(1200, http.StatusConflict, "Move closer to the node.")                    // data: distanceM, rangeM
	HarvestGPSWeak     = def(1201, http.StatusConflict, "GPS signal too weak. Wait for a better fix.") // data: accuracyM, maxAccuracyM
	HarvestCooldown    = def(1202, http.StatusConflict, "This node is recharging for you.")            // data: readyInSec
	HarvestDepleted    = def(1203, http.StatusConflict, "This node is depleted. It will regrow.")
	HarvestCarryFull   = def(1204, http.StatusConflict, "You cannot carry any more.")
	HarvestImplausible = def(1205, http.StatusConflict, "Location could not be confirmed.")
	NodeInactive       = def(1206, http.StatusNotFound, "This node is not available.")

	FactionLocked        = def(1300, http.StatusConflict, "This faction has no Regional Commander here yet. Claim regional command first.")
	FactionLevelLow      = def(1301, http.StatusConflict, "Your level is too low for that.") // data: needLevel, level
	FactionMemberAlready = def(1302, http.StatusConflict, "You already belong to a faction.")
	FactionClaimed       = def(1303, http.StatusConflict, "This faction already has a Regional Commander here.")
	// 1304 is no longer used: a faction is claimed per region, so an active one can still be claimed elsewhere.
	FactionNotLocked = def(1304, http.StatusConflict, "This faction is already active.")
	// 1305 is log-only: account cleanup skipped a faction head (succession is decided by hand).

	BaseSpacingSame  = def(1402, http.StatusConflict, "Too close to another of your faction's bases.")
	BaseSpacingOther = def(1403, http.StatusConflict, "Too close to another faction's base.")

	TetherCap      = def(1600, http.StatusConflict, "You already own the most Tether Nodes you can.") // data: max
	TetherNoNode   = def(1601, http.StatusConflict, "No free resource node in range. Build closer to one.")
	TetherCost     = def(1602, http.StatusConflict, "You don't have enough resources for that.") // data: short
	TetherGone     = def(1603, http.StatusNotFound, "This Tether Node is gone.")
	TetherNotOwner = def(1604, http.StatusForbidden, "Only the owner can do that.")
	TetherNothing  = def(1605, http.StatusConflict, "Nothing to move right now.")
	TetherTooFar   = def(1606, http.StatusConflict, "Move closer to the Tether Node.") // data: distanceM, rangeM
	TetherNoAccess = def(1607, http.StatusForbidden, "This Tether Node belongs to another faction.")
	TetherMaxTier  = def(1608, http.StatusConflict, "This Tether Node is already fully upgraded.")
	DropGone       = def(1609, http.StatusNotFound, "This drop is gone.")
	DropTooFar     = def(1610, http.StatusConflict, "Move closer to the drop.") // data: distanceM, rangeM

	TetherCantAttack = def(1611, http.StatusForbidden, "You can't attack this Tether Node.")
	TetherTooClose   = def(1612, http.StatusConflict, "Too close to another Tether Node.") // data: spacingM

	CraftUnknown  = def(1700, http.StatusBadRequest, "That can't be crafted.")
	GearMissing   = def(1701, http.StatusNotFound, "You don't own that gear.")
	GearNoUpgrade = def(1702, http.StatusConflict, "This gear is already at its top tier.")
	GearNotWorn   = def(1703, http.StatusConflict, "This gear doesn't need repair.")

	NoCells         = def(1710, http.StatusConflict, "You need more energy cells for that.") // data: need, have
	ItemMissing     = def(1711, http.StatusConflict, "You don't have that item.")            // data: item
	OutOfRange      = def(1712, http.StatusConflict, "Move closer.")                         // data: distanceM, rangeM
	AlreadyJammed   = def(1713, http.StatusConflict, "This Tether Node is already jammed.")
	SpikeArmed      = def(1714, http.StatusConflict, "A spike is already planted here.")
	NoSpike         = def(1715, http.StatusConflict, "There is no spike to defuse.")
	NodeContested   = def(1716, http.StatusConflict, "This node is contested. Another player holds its next harvests.")
	ContestRunning  = def(1717, http.StatusConflict, "A contest is already running here.")
	ContestNobody   = def(1718, http.StatusConflict, "There is nobody to contest this node with.")
	ContestOver     = def(1719, http.StatusConflict, "That contest is over.")
	ContestNotYours = def(1720, http.StatusForbidden, "You can't join this contest.")

	NoDroneBay = def(1730, http.StatusConflict, "You need a working Drone Bay for that.")
	DroneBusy  = def(1731, http.StatusConflict, "Your drone is already in flight.")

	NoDroneInbound     = def(1732, http.StatusConflict, "No drone is flying at this Tether Node.")
	AlreadyIntercepted = def(1733, http.StatusConflict, "Every drone flying at this Tether Node is already intercepted.")
	ShieldNotHigher    = def(1734, http.StatusConflict, "This Tether Node already has that shield or a better one.")

	PushTokenBad      = def(1740, http.StatusBadRequest, "That notification token isn't valid.")
	PushNotConfigured = def(1741, http.StatusServiceUnavailable, "Notifications aren't set up on this server.")
	PushNoToken       = def(1742, http.StatusConflict, "No phone is registered for notifications.")
	PushFailed        = def(1743, http.StatusBadGateway, "The notification could not be sent.")

	SeedNoMap = def(1750, http.StatusServiceUnavailable, "The map data for seeding hasn't been imported yet.")

	MissionGone          = def(1760, http.StatusNotFound, "That order is gone or already claimed.")
	MissionNotDone       = def(1761, http.StatusConflict, "That order isn't done yet.")
	NotRegionalCommander = def(1762, http.StatusForbidden, "Only your Regional Commander can do that.")
	NotInBranch          = def(1763, http.StatusConflict, "That player isn't in your faction's branch here.")
	AlreadyWarden        = def(1764, http.StatusConflict, "That player is already a Warden or above.")
	CallGone             = def(1765, http.StatusNotFound, "That call has ended.")
	MessageGone          = def(1766, http.StatusNotFound, "That message is gone.")

	Internal = def(1900, http.StatusInternalServerError, "Something went wrong. Try again.")
)
