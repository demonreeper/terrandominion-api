package game

import (
	"strings"
	"testing"
)

func TestAppealCode(t *testing.T) {
	a := AppealCode(MarkDevice, []byte("phone-one"))
	if len(a) != 8 || !ValidPublicCode(a) {
		t.Fatalf("appeal code %q is not 8 Crockford characters", a)
	}
	if a != AppealCode(MarkDevice, []byte("phone-one")) {
		t.Error("the same mark gave two codes")
	}
	if a == AppealCode(MarkSignIn, []byte("phone-one")) || a == AppealCode(MarkDevice, []byte("phone-two")) {
		t.Error("different marks gave the same code")
	}
}

func TestAppealMailto(t *testing.T) {
	link := AppealMailto("K7QX3M9P")
	if !strings.HasPrefix(link, "mailto:"+SupportEmail+"?") || !strings.Contains(link, "K7QX3M9P") {
		t.Errorf("appeal link %q", link)
	}
	if strings.Contains(link, " ") {
		t.Errorf("appeal link not escaped: %q", link)
	}
}

func TestJudgeMarks(t *testing.T) {
	if v := JudgeMarks(nil); v.Banned || v.BoundTo != "" {
		t.Errorf("no marks: %+v", v)
	}
	v := JudgeMarks([]Mark{
		{Kind: MarkSignIn, FactionCode: "faction_2", AppealCode: "SIGNIN01"},
		{Kind: MarkDevice, FactionCode: "faction_1", AppealCode: "DEVICE01"},
	})
	if v.Banned || v.BoundTo != "faction_1" || v.BoundAppeal != "DEVICE01" {
		t.Errorf("the device's pledge should win: %+v", v)
	}
	v = JudgeMarks([]Mark{{Kind: MarkSignIn, FactionCode: "faction_3"}, {Kind: MarkDevice, Banned: true, AppealCode: "BANNED01"}})
	if !v.Banned || v.AppealCode != "BANNED01" || v.BoundTo != "faction_3" {
		t.Errorf("a ban on the device refuses, the sign-in's pledge still binds: %+v", v)
	}
}

func TestMayPledgeTo(t *testing.T) {
	if !MayPledgeTo("", "faction_1") || !MayPledgeTo("faction_1", "faction_1") || MayPledgeTo("faction_1", "faction_2") {
		t.Error("a bound player may only pledge to their own faction")
	}
}
