package game

import (
	"crypto/sha256"
	"net/url"
	"strings"
)

// The device record (docs/freelancer-path-plan.md phase 2): a pledge and a ban are bound to the
// phone and the sign-in, so deleting the account and making a new one undoes neither.

const (
	MarkDevice = "device"
	MarkSignIn = "signin"
)

// SupportEmail is where an APPEAL goes (the address on the published legal pages).
const SupportEmail = "thehussha@gmail.com"

// Player-facing lines (approved in the plan).
const (
	DeviceBannedLine = "This device is banned."
	PledgeStandsLine = "Your pledge stands. A tether to a power never breaks."
)

// AppealCode is the short code shown on the banned and pledge-stands screens, from a mark's hash: 8
// characters of Crockford base32. It identifies the mark to an admin without showing the hash.
func AppealCode(kind string, hash []byte) string {
	sum := sha256.Sum256(append([]byte(kind+":"), hash...))
	var b strings.Builder
	var acc uint64
	bits := 0
	for _, by := range sum[:5] {
		acc = acc<<8 | uint64(by)
		bits += 8
		for bits >= 5 {
			bits -= 5
			b.WriteByte(publicCodeChars[(acc>>uint(bits))&31])
		}
	}
	return b.String()
}

// AppealMailto is the APPEAL link: an email to support with the appeal code filled in.
func AppealMailto(code string) string {
	subject := "Terran Dominion appeal " + code
	body := "Appeal code: " + code + "\n\nTell us what happened. If you bought this phone second-hand, attach a proof of purchase."
	return "mailto:" + SupportEmail + "?subject=" + url.PathEscape(subject) + "&body=" + url.PathEscape(body)
}

// Mark is what the device record knows about one device or sign-in.
type Mark struct {
	Kind        string
	FactionCode string
	Banned      bool
	AppealCode  string
}

// MarkVerdict is what a sign-in's marks mean: refuse it (a ban), and the faction a pledge is bound to.
type MarkVerdict struct {
	Banned      bool
	AppealCode  string
	BoundTo     string
	BoundAppeal string
}

// JudgeMarks reads the marks found for a sign-in's device and account. A ban on either refuses it. A
// pledge on either binds the account to that faction; the device's pledge wins over the sign-in's,
// since the phone is what changes hands least often.
func JudgeMarks(marks []Mark) MarkVerdict {
	var v MarkVerdict
	for _, m := range marks {
		if m.Banned && !v.Banned {
			v.Banned, v.AppealCode = true, m.AppealCode
		}
	}
	for _, kind := range []string{MarkDevice, MarkSignIn} {
		for _, m := range marks {
			if v.BoundTo == "" && m.Kind == kind && m.FactionCode != "" {
				v.BoundTo, v.BoundAppeal = m.FactionCode, m.AppealCode
			}
		}
	}
	return v
}

// MayPledgeTo says whether a player bound to a faction by the device record may pledge to this one:
// only to the faction they are bound to. An unbound player may pledge anywhere.
func MayPledgeTo(boundTo, code string) bool {
	return boundTo == "" || boundTo == code
}
