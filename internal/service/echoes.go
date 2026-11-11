package service

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/apperr"
	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

// Signal echoes (docs/no-ar-support-plan.md, "AR sees more", milestone 7). Looks and story only:
// nothing here gives resources, XP or combat information, so the client saying the AR view was open
// is enough; there is nothing to cheat for.

const (
	// At most this many echo cells are filled or moved by one request, so it stays quick.
	echoCellsPerRequest = 9
	echoListLimit       = 30
)

// NearbyEchoes lists the echoes within the AR view radius. The cells around are filled first: echoes
// whose time is up have moved on (they are removed) and the cell gets new ones beside its nodes and
// Tether Nodes, each carrying a random line.
func (s *Service) NearbyEchoes(ctx context.Context, playerID int64, lat, lon float64) ([]store.Echo, error) {
	if !game.ValidCoordinate(lat, lon) {
		return nil, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	if settings.EchoesPerCell <= 0 || settings.EchoCellM <= 0 || settings.EchoViewRadiusM <= 0 {
		return []store.Echo{}, nil
	}
	now := s.now()
	cells := game.SeedCellsAround(settings.EchoCellM, lat, lon, settings.EchoViewRadiusM)
	for i, c := range cells {
		if i >= echoCellsPerRequest || ctx.Err() != nil {
			break
		}
		if err := s.fillEchoCell(ctx, settings, c, now); err != nil {
			// Never shown: the player still sees the echoes that exist.
			slog.Error("E1900 echo cell fill failed", "cell", c.ID(settings.EchoCellM), "err", err)
		}
	}
	return store.NearbyEchoes(ctx, s.db, playerID, lat, lon, settings.EchoViewRadiusM, now, echoListLimit)
}

func (s *Service) fillEchoCell(ctx context.Context, settings game.Settings, c game.SeedCell, now time.Time) error {
	id := c.ID(settings.EchoCellM)
	return db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		if err := store.LockEchoCell(ctx, tx, id); err != nil {
			return err
		}
		have, err := store.ClearMovedEchoes(ctx, tx, id, now)
		if err != nil {
			return err
		}
		need := settings.EchoesPerCell - have
		if need <= 0 {
			return nil
		}
		anchors, err := store.EchoAnchors(ctx, tx, toBox(c.Bounds(settings.EchoCellM)), need)
		if err != nil {
			return err
		}
		for _, a := range anchors {
			bearing, distance := game.EchoOffset(settings, rand.Float64(), rand.Float64())
			lat, lon := game.OffsetPoint(a.Lat, a.Lon, bearing, distance)
			set, line := game.PickEchoLine(rand.Float64())
			if set == 0 {
				return nil
			}
			// Spread over three quarters to one and a quarter of the time, so a cell doesn't move all at once.
			stay := time.Duration(settings.EchoMoveHours * (0.75 + 0.5*rand.Float64()) * float64(time.Hour))
			if err := store.CreateEcho(ctx, tx, id, lat, lon, set, line, now.Add(stay)); err != nil {
				return err
			}
		}
		return nil
	})
}

// EchoReadOutcome is what reading an echo shows: the line, and where the CODEX stands.
type EchoReadOutcome struct {
	Set  int    `json:"set"`
	Line int    `json:"line"`
	Text string `json:"text"`
	// New: the line was not in the CODEX before.
	New   bool `json:"new"`
	Found int  `json:"found"`
	Total int  `json:"total"`
	// Badge is the profile badge this read earned (a finished set), or empty.
	Badge string `json:"badge,omitempty"`
}

// ReadEcho reads an echo within range, in the AR view. The line goes into the CODEX; a finished set
// gives its badge. Nothing else changes: no resources, no XP.
func (s *Service) ReadEcho(ctx context.Context, playerID, echoID int64, fix TetherFix, arView bool) (EchoReadOutcome, error) {
	var out EchoReadOutcome
	if !arView {
		return out, apperr.EchoNotInAr
	}
	if !game.ValidCoordinate(fix.Lat, fix.Lon) {
		return out, apperr.BadRequest.With("bad coordinate")
	}
	settings := s.Settings(ctx)
	now := s.now()

	err := db.InTx(ctx, s.db, func(tx *sql.Tx) error {
		echo, err := store.EchoByID(ctx, tx, echoID, now)
		if errors.Is(err, store.ErrNotFound) {
			return apperr.EchoGone
		}
		if err != nil {
			return err
		}

		lastFix, err := store.LastHarvestFix(ctx, tx, playerID)
		if err != nil {
			return err
		}
		var last *game.Fix
		if lastFix != nil {
			last = &game.Fix{Lat: lastFix.Lat, Lon: lastFix.Lon, AccuracyM: lastFix.AccuracyM, At: lastFix.At}
		}
		player := game.Fix{Lat: fix.Lat, Lon: fix.Lon, AccuracyM: fix.AccuracyM, At: now}
		if _, ruleErr := game.CheckAtEcho(settings, player, last, echo.Lat, echo.Lon); ruleErr != nil {
			return ruleErr
		}

		text, ok := game.EchoLine(echo.Set, echo.Line)
		if !ok {
			return apperr.EchoGone
		}
		isNew, err := store.AddEchoRead(ctx, tx, playerID, echo.Set, echo.Line)
		if err != nil {
			return err
		}
		reads, err := store.EchoReads(ctx, tx, playerID)
		if err != nil {
			return err
		}
		out = EchoReadOutcome{
			Set: echo.Set, Line: echo.Line, Text: text, New: isNew,
			Found: len(reads[echo.Set]), Total: game.EchoSetSize(echo.Set),
		}
		if isNew && out.Total > 0 && out.Found >= out.Total {
			badge := game.EchoBadge(echo.Set)
			granted, err := store.GrantTitle(ctx, tx, playerID, badge)
			if err != nil {
				return err
			}
			if granted {
				out.Badge = badge
			}
		}
		return nil
	})
	return out, err
}

// CodexSet is one set of five on the CODEX: the lines found have their text, the rest are locked
// (empty text; the phone shows static).
type CodexSet struct {
	Set      int         `json:"set"`
	Found    int         `json:"found"`
	Total    int         `json:"total"`
	Complete bool        `json:"complete"`
	Lines    []CodexLine `json:"lines"`
}

type CodexLine struct {
	Line int    `json:"line"`
	Text string `json:"text,omitempty"`
}

// codex is a player's CODEX for /v1/me: every set, found lines with their words.
func codex(ctx context.Context, q db.Querier, playerID int64) ([]CodexSet, error) {
	reads, err := store.EchoReads(ctx, q, playerID)
	if err != nil {
		return nil, err
	}
	out := make([]CodexSet, 0, len(game.EchoSets))
	for i, lines := range game.EchoSets {
		set := i + 1
		found := map[int]bool{}
		for _, line := range reads[set] {
			found[line] = true
		}
		cs := CodexSet{Set: set, Total: len(lines), Lines: make([]CodexLine, 0, len(lines))}
		for j, text := range lines {
			line := CodexLine{Line: j + 1}
			if found[j+1] {
				line.Text = text
				cs.Found++
			}
			cs.Lines = append(cs.Lines, line)
		}
		cs.Complete = cs.Found >= cs.Total
		out = append(out, cs)
	}
	return out, nil
}
