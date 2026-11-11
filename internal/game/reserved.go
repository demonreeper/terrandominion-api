package game

import "strings"

// Reserved player ids (owner, 2026-11-11; migration 0043):
//
//	1      the owner: the primary admin. A player like any other, counted as one, but always a
//	       freelancer: the owner never leads or joins a faction.
//	2-7    the six powers, one system account each, in faction order: 2 Signum, 3 Shingō, 4 Ishāra,
//	       5 Sēma, 6 Tohu, 7 Merki. Nobody signs in as them.
//	8      the seventh voice, Langis (signal_unbound, migration 0049). Never shown before its call.
//	9-10   held for later.
//	11+    everyone else, moderators and other admins included.
const (
	PrimaryAdminID = 1
	FirstPlayerID  = 11
	powerIDOffset  = 1 // a power's account id is its faction id + 1
	powerCount     = 6
)

// PowerAccountID is the system account of the power behind a faction (faction 1 is account 2), or
// 0 when the faction id is not one of the six.
func PowerAccountID(factionID int) int64 {
	if factionID < 1 || factionID > powerCount {
		return 0
	}
	return int64(factionID + powerIDOffset)
}

// IsReservedID says whether an id is one of the ten that are never given to a new account.
func IsReservedID(id int64) bool { return id >= 1 && id < FirstPlayerID }

// IsPrimaryAdminEmail says whether a sign-in's email is the owner's (TD_PRIMARY_ADMIN_EMAIL), who
// gets id 1 on an empty database. Never true when the owner's email is not configured.
func IsPrimaryAdminEmail(configured, email string) bool {
	configured, email = strings.TrimSpace(configured), strings.TrimSpace(email)
	return configured != "" && strings.EqualFold(configured, email)
}

// MayEnterFaction says whether this account may lead or join a faction at all: everyone but the
// primary admin, who plays as a freelancer. (Admins look inside a faction without joining it.)
func MayEnterFaction(playerID int64) bool { return playerID != PrimaryAdminID }
