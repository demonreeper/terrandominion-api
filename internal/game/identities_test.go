package game

import "testing"

func TestDecideLink(t *testing.T) {
	cases := []struct {
		name       string
		ownerFound bool
		owner      int64
		provider   string
		linked     []string
		want       LinkOutcome
	}{
		{"new sign-in", false, 0, "google", []string{"apple"}, LinkNew},
		{"already on this player", true, 7, "google", []string{"apple", "google"}, LinkAlready},
		{"opens another account", true, 9, "google", []string{"apple"}, LinkTaken},
		{"a different Google already linked", false, 0, "google", []string{"apple", "google"}, LinkProviderUsed},
	}
	for _, c := range cases {
		if got := DecideLink(c.ownerFound, c.owner, 7, c.provider, c.linked); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDecideUnlink(t *testing.T) {
	cases := []struct {
		name     string
		linked   []string
		provider string
		want     UnlinkOutcome
	}{
		{"one of two", []string{"apple", "google"}, "google", UnlinkOK},
		{"the last one", []string{"apple"}, "apple", UnlinkLast},
		{"not linked", []string{"apple"}, "google", UnlinkNotLinked},
		{"dev counts as a sign-in", []string{"dev", "google"}, "google", UnlinkOK},
	}
	for _, c := range cases {
		if got := DecideUnlink(c.linked, c.provider); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIsLinkable(t *testing.T) {
	if !IsLinkable("google") || !IsLinkable("apple") || IsLinkable("dev") || IsLinkable("") {
		t.Error("linkable providers are google and apple only")
	}
}
