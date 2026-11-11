// Package web holds the pages tdapi serves itself, embedded in the binary like the migrations.
// For now that is one page: a player's public profile, behind a share link (docs/profile-plan.md, P2).
package web

import (
	_ "embed"
	"html/template"
	"io"
)

//go:embed profile.html
var profileHTML string

// html/template escapes every value for where it lands (text, attribute, URL, CSS), so a callsign
// can never inject markup.
var profileTemplate = template.Must(template.New("profile").Parse(profileHTML))

// RecordLine is one line of the service record: what, and how many.
type RecordLine struct {
	Label string
	Value string
}

// ProfilePage is everything the profile page shows. Only public fields belong here.
type ProfilePage struct {
	// Gone: the code is unknown, or the player is deleted or banned. Nothing else is shown.
	Gone        bool
	Title       string // the page and link-preview title
	Description string // the link-preview line
	URL         string // the share link itself
	Name        string
	Initials    string
	Level       int
	Standing    string
	// FactionColor is the faction's "#rrggbb", or empty for a freelancer.
	FactionColor string
	Perks        []string
	Badges       []string
	Record       []RecordLine
	// Store links; a button is left out when its link is empty.
	PlayURL     string
	AppStoreURL string
}

// RenderProfile writes the profile page.
func RenderProfile(w io.Writer, page ProfilePage) error {
	return profileTemplate.Execute(w, page)
}
