package game

// Android notification channels, created by the app.
const (
	PushChannelAlerts  = "tn_alerts"
	PushChannelResults = "results"
)

// PushRule says how one kind of notice reaches the phone.
type PushRule struct {
	Title     string
	Channel   string
	Throttled bool
	// Always: sent whatever the player's two push switches say (a power's call is neither an
	// attack nor a result).
	Always bool
}

// Attacks reports whether the rule follows the player's "attacks on my Tether Nodes" switch;
// otherwise it follows "results of my attacks".
func (r PushRule) Attacks() bool { return r.Channel == PushChannelAlerts }

var pushRules = map[string]PushRule{
	"spike":         {Title: "Spike planted", Channel: PushChannelAlerts},
	"drone_inbound": {Title: "Drone inbound", Channel: PushChannelAlerts},
	"attacked":      {Title: "Tether Node under attack", Channel: PushChannelAlerts, Throttled: true},
	"destroyed":     {Title: "Tether Node destroyed", Channel: PushChannelAlerts},
	"jammed":        {Title: "Tether Node jammed", Channel: PushChannelAlerts, Throttled: true},

	"contest_won":       {Title: "Contest won", Channel: PushChannelResults},
	"contest_lost":      {Title: "Contest lost", Channel: PushChannelResults},
	"spike_result":      {Title: "Spike report", Channel: PushChannelResults},
	"drone_result":      {Title: "Drone report", Channel: PushChannelResults},
	"defused":           {Title: "Spike defused", Channel: PushChannelResults},
	"drone_intercepted": {Title: "Drone intercepted", Channel: PushChannelResults},
	"spike_fired":       {Title: "Spike went off", Channel: PushChannelResults},

	"friend_request":  {Title: "Friend request", Channel: PushChannelResults},
	"friend_accepted": {Title: "New friend", Channel: PushChannelResults},

	"base_proposal": {Title: "Base proposal", Channel: PushChannelResults},
	"base_approved": {Title: "Flag approved", Channel: PushChannelResults},
	"base_declined": {Title: "Flag declined", Channel: PushChannelResults},
	"base_expired":  {Title: "Flag expired", Channel: PushChannelResults},

	// A power's call the app did not pick up. The body is "Incoming call: <power>".
	"call": {Title: "Tether Rise", Channel: PushChannelResults, Always: true},
}

// PushRuleFor returns the rule for a notice kind; false means it stays in the game only.
func PushRuleFor(kind string) (PushRule, bool) {
	rule, ok := pushRules[kind]
	return rule, ok
}

// NoticePushState is the outbox state a new notice of this kind starts in.
func NoticePushState(kind string) string {
	if _, ok := pushRules[kind]; ok {
		return "pending"
	}
	return "none"
}
