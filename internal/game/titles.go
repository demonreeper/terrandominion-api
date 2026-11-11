package game

// Unbound titles (docs/freelancer-path-plan.md F2): past the faction level a freelancer keeps
// earning titles, each a profile badge given in a message from the seventh voice. A member earns no
// more; titles already earned stay.

// Title is one of them: its badge, the level setting it comes at, and the seventh voice's line.
type Title struct {
	Badge string
	Level func(Settings) int
	Line  string
}

// Titles are the Unbound titles in order. Lines word for word as approved.
var Titles = []Title{
	{Badge: "drifter", Level: func(s Settings) int { return s.TitleDrifterLevel }, Line: "Drifter. You've earned it."},
	{Badge: "wayfarer", Level: func(s Settings) int { return s.TitleWayfarerLevel }, Line: "Wayfarer. They can't place you now."},
	{Badge: "unbound", Level: func(s Settings) int { return s.TitleUnboundLevel }, Line: "Unbound. Like me."},
}

// TitleMessageKind is the kind of the message that gives a title (one per title per player).
func TitleMessageKind(badge string) string { return "langis_title_" + badge }

// TitlesDue lists the titles a player at this level should hold: every title at or below their
// level, for a freelancer who has heard from the seventh voice. Granting is idempotent, so a player
// already past several levels gets each one once.
func TitlesDue(s Settings, level int, freelancer, contact bool) []Title {
	if !freelancer || !contact || level <= s.LeadFactionLevel-1 {
		return nil
	}
	var due []Title
	for _, t := range Titles {
		if at := t.Level(s); at > 0 && level >= at {
			due = append(due, t)
		}
	}
	return due
}
