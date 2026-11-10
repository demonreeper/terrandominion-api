package game

import "testing"

func TestAppBuildAllowed(t *testing.T) {
	s := Settings{MinAppBuild: 60, MinAppBuildIOS: 5}
	cases := []struct {
		platform string
		build    int
		want     bool
	}{
		{"android", 59, false},
		{"android", 60, true},
		{"android", 61, true},
		{"android", 0, true}, // no build sent: scripts and curl
		{"", 10, false},      // no platform: treated as Android
		{"ios", 4, false},
		{"iOS", 5, true},
	}
	for _, c := range cases {
		if _, got := AppBuildAllowed(s, c.platform, c.build); got != c.want {
			t.Errorf("%s build %d: allowed = %v, want %v", c.platform, c.build, got, c.want)
		}
	}
	if _, got := AppBuildAllowed(Settings{}, "android", 1); !got {
		t.Error("a minimum of 0 must lock nothing")
	}
}
