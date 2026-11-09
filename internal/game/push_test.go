package game

import "testing"

func TestNoticePushState(t *testing.T) {
	cases := map[string]string{
		"spike":        "pending",
		"attacked":     "pending",
		"drone_result": "pending",
		"contest":      "none", // a 60 s contest ends before anyone could walk there
		"made_up":      "none",
	}
	for kind, want := range cases {
		if got := NoticePushState(kind); got != want {
			t.Errorf("%s: got %s, want %s", kind, got, want)
		}
	}
}

func TestPushRules(t *testing.T) {
	for kind, wantThrottled := range map[string]bool{"attacked": true, "jammed": true, "spike": false, "destroyed": false} {
		rule, ok := PushRuleFor(kind)
		if !ok || !rule.Attacks() || rule.Throttled != wantThrottled {
			t.Errorf("%s: %+v, %v", kind, rule, ok)
		}
	}
	if rule, ok := PushRuleFor("spike_fired"); !ok || rule.Attacks() {
		t.Errorf("spike_fired should be a result: %+v, %v", rule, ok)
	}
}
