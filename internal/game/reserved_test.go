package game

import "testing"

func TestReservedIDs(t *testing.T) {
	for id, want := range map[int64]bool{0: false, 1: true, 2: true, 7: true, 10: true, 11: false, 500: false} {
		if got := IsReservedID(id); got != want {
			t.Errorf("IsReservedID(%d) = %v", id, got)
		}
	}
	for faction, want := range map[int]int64{1: 2, 6: 7, 0: 0, 7: 0, -1: 0} {
		if got := PowerAccountID(faction); got != want {
			t.Errorf("PowerAccountID(%d) = %d, want %d", faction, got, want)
		}
	}
	// The powers' accounts are all reserved, and none is the primary admin.
	for faction := 1; faction <= 6; faction++ {
		id := PowerAccountID(faction)
		if !IsReservedID(id) || id == PrimaryAdminID {
			t.Errorf("power %d has account %d", faction, id)
		}
	}
}

func TestPrimaryAdminStaysAFreelancer(t *testing.T) {
	if MayEnterFaction(PrimaryAdminID) {
		t.Error("the primary admin may enter a faction")
	}
	// Other admins and moderators are ordinary accounts and may.
	if !MayEnterFaction(FirstPlayerID) || !MayEnterFaction(42) {
		t.Error("an ordinary account may not enter a faction")
	}
}

func TestIsPrimaryAdminEmail(t *testing.T) {
	for _, c := range []struct {
		configured, email string
		want              bool
	}{
		{"owner@example.com", "owner@example.com", true},
		{" Owner@Example.com ", "owner@example.COM", true},
		{"owner@example.com", "other@example.com", false},
		{"", "", false}, // not configured: nobody is the owner
		{"", "owner@example.com", false},
		{"owner@example.com", "", false},
	} {
		if got := IsPrimaryAdminEmail(c.configured, c.email); got != c.want {
			t.Errorf("IsPrimaryAdminEmail(%q, %q) = %v", c.configured, c.email, got)
		}
	}
}
