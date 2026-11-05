// Package jobs holds the scheduled tasks. Each runs once and exits, started by cron or a systemd
// timer (see deploy/). Every job is safe to run late, twice, or concurrently with the API.
package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"

	"github.com/demonreeper/terrandominion/api/internal/db"
	"github.com/demonreeper/terrandominion/api/internal/game"
	"github.com/demonreeper/terrandominion/api/internal/store"
)

type Job func(ctx context.Context, conn *sql.DB) error

var registry = map[string]Job{
	"node-regrow":     nodeRegrow,
	"account-cleanup": accountCleanup,
	"session-prune":   sessionPrune,
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Run(ctx context.Context, conn *sql.DB, name string) error {
	job, ok := registry[name]
	if !ok {
		return fmt.Errorf("unknown job %q (have %v)", name, Names())
	}
	return job(ctx, conn)
}

func nodeRegrow(ctx context.Context, conn *sql.DB) error {
	n, err := store.RegrowNodes(ctx, conn)
	if err != nil {
		return err
	}
	slog.Info("job node-regrow", "regrown", n)
	return nil
}

func sessionPrune(ctx context.Context, conn *sql.DB) error {
	n, err := store.PruneSessions(ctx, conn)
	if err != nil {
		return err
	}
	slog.Info("job session-prune", "deleted", n)
	return nil
}

// accountCleanup schedules long-inactive accounts for deletion, then erases accounts whose grace
// period has ended. Faction heads are skipped and reported: what happens to a faction when its
// head leaves (next rank or a vote) is not decided yet, so a person decides for now.
func accountCleanup(ctx context.Context, conn *sql.DB) error {
	values, err := store.LoadSettings(ctx, conn)
	if err != nil {
		return err
	}
	settings := game.SettingsFrom(values)

	if settings.InactiveDeleteDays > 0 {
		n, err := store.ScheduleInactive(ctx, conn, settings.InactiveDeleteDays, settings.DeleteGraceDays)
		if err != nil {
			return err
		}
		slog.Info("job account-cleanup: inactive accounts scheduled", "count", n)
	}

	due, err := store.DueDeletions(ctx, conn, 500)
	if err != nil {
		return err
	}
	deleted := 0
	for _, id := range due {
		err := db.InTx(ctx, conn, func(tx *sql.Tx) error {
			leads, err := store.LeadsFaction(ctx, tx, id)
			if err != nil {
				return err
			}
			if leads {
				slog.Warn("E1305 deletion skipped: player heads a faction; decide succession by hand", "player", id)
				return nil
			}
			deleted++
			return store.AnonymizePlayer(ctx, tx, id)
		})
		if err != nil {
			return fmt.Errorf("delete player %d: %w", id, err)
		}
	}
	slog.Info("job account-cleanup: accounts deleted", "count", deleted, "due", len(due))
	return nil
}
