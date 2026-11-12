// dev2live is the Dev2LiveSync job (pre-launch only): it resets the live database's game data and carries
// the test players over from dev, so their accounts, sign-ins and callsigns are reserved for launch.
//
// What it does, in one transaction on live:
//   - carries every dev player with a Google or Apple sign-in (not -skip, not banned or deleted): the account
//     (id, callsign, share code, admin and mod flags, switches) and its sign-in links. Dev-only sign-ins
//     ("dev") never go to live.
//   - wipes every game table (wipeTables) and removes all sessions and push tokens: everyone signs in again.
//   - resets every player to level 1, 0 XP, no faction, no sanctions; unlocks nothing (factions back to
//     locked, no head); removes admin test nodes (scatter), rare and group nodes, refills the rest.
//   - gives each player what a new account gets: starter gear, the welcome resources and energy cells.
//   - gives the carried players the tester badge (-badge), except -no-badge.
//
// Without -apply it runs the same transaction and rolls it back, printing what would change: a dry run
// that also proves the SQL works. Run it through scripts/dev2live-sync.ps1, which backs live up first.
//
// Env: TD_DEV_DATABASE_URL, TD_LIVE_DATABASE_URL.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// keepTables survive the reset: reference data, the map, settings, the audit log and the accounts.
// players and player_identities are kept and handled on their own.
var keepTables = set(
	"admin_actions", "announcement", "callsign_blocked", "countries", "devices", "factions", "gear_types",
	"live_regions", "mission_types", "nodes", "osm_blocked", "osm_land", "osm_sensitive", "osm_walkways",
	"player_identities", "players", "resource_types", "schema_migrations", "seed_areas", "seed_cells", "settings",
	"signal_echoes",
)

// wipeTables are game data, emptied. A table in neither list stops the job: decide which it is first.
var wipeTables = set(
	"base_collector_links", "base_contributions", "base_hacks", "base_projects", "base_proposals", "base_slots",
	"base_strikes", "base_vault_log", "base_vaults", "base_zones", "bases", "captain_missions", "commander_ballots",
	"commander_votes", "device_marks", "drone_strikes", "echo_reads", "email_searches", "faction_branches",
	"faction_members", "friend_declines", "friendships", "group_node_harvests", "harvests", "infringements",
	"job_takes", "jobs", "map_drops", "node_contest_joins", "node_contests", "nomad_asks", "notices",
	"player_badges", "player_calls", "player_fix_trail", "player_gear", "player_invitations", "player_items",
	"player_last_fix", "player_messages", "player_missions", "player_playstyle", "player_reports",
	"player_resources", "player_sanctions", "player_uploaded", "push_tokens", "sessions", "squad_members",
	"tether_attacks", "tether_links", "tether_nodes", "tether_spikes", "tether_stock", "tether_transfers",
	"walk_bonus_days", "walk_link_asks", "walk_link_finds", "walk_link_members", "walk_link_trail", "walk_links",
	"walk_open", "xp_events",
)

// playerColumns are what a carried account keeps from dev. Progress (level, XP, points) is reset.
var playerColumns = []string{
	"id", "display_name", "type", "status", "is_admin", "is_mod", "country_code", "photo_file", "last_app_version",
	"last_seen_at", "created_at", "nakama_user_id", "push_attacks", "push_results", "public_code", "callsign",
	"callsign_changed_at", "friend_name", "findable_by_email", "accept_friends", "spoof_exempt",
}

func main() {
	apply := flag.Bool("apply", false, "commit the change (default: dry run, rolled back)")
	skip := flag.String("skip", "", "dev player ids not to carry, comma separated")
	noBadge := flag.String("no-badge", "1", "carried player ids that get no tester badge, comma separated")
	badge := flag.String("badge", "first signal", "the tester badge's kind (shown in capitals); empty for none")
	keepTestNodes := flag.Bool("keep-test-nodes", false, "keep admin test (scatter), rare and group nodes")
	allowLiveOnly := flag.Bool("allow-live-only", false, "go on although live has players dev doesn't (after launch: never)")
	flag.Parse()

	if err := run(*apply, ids(*skip), ids(*noBadge), strings.TrimSpace(*badge), *keepTestNodes, *allowLiveOnly); err != nil {
		fmt.Fprintln(os.Stderr, "dev2live: "+err.Error())
		os.Exit(1)
	}
}

func run(apply bool, skip, noBadge []int64, badge string, keepTestNodes, allowLiveOnly bool) error {
	ctx := context.Background()
	dev, err := open("TD_DEV_DATABASE_URL")
	if err != nil {
		return err
	}
	defer dev.Close()
	live, err := open("TD_LIVE_DATABASE_URL")
	if err != nil {
		return err
	}
	defer live.Close()

	devName, liveName := dbName(ctx, dev), dbName(ctx, live)
	fmt.Printf("dev  = %s\nlive = %s\n", devName, liveName)
	if devName == liveName {
		return errors.New("dev and live are the same database")
	}
	if a, b := migrations(ctx, dev), migrations(ctx, live); a != b {
		return fmt.Errorf("migrations differ (dev %s, live %s): deploy the same build to both first", a, b)
	}

	// The players to carry, with their Google and Apple sign-ins.
	players, idents, err := devPlayers(ctx, dev, skip)
	if err != nil {
		return err
	}
	carried := make([]int64, 0, len(players))
	for _, p := range players {
		carried = append(carried, p.ID)
	}

	tx, err := live.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '15s'`); err != nil {
		return err
	}

	if err := checkTables(ctx, tx); err != nil {
		return err
	}
	if err := checkConflicts(ctx, tx, players, idents, allowLiveOnly); err != nil {
		return err
	}

	fmt.Println("\nCarry from dev:")
	for _, p := range players {
		fmt.Printf("  %-4d %-18s level %-3d %s\n", p.ID, p.Name, p.Level, providers(idents[p.ID]))
	}
	if len(skip) > 0 {
		fmt.Printf("  skipped: %v\n", skip)
	}

	// 1. Wipe the game tables.
	counts, err := tableCounts(ctx, tx)
	if err != nil {
		return err
	}
	var wipe []string
	for t := range wipeTables {
		if _, ok := counts[t]; ok {
			wipe = append(wipe, "td."+t)
		}
	}
	sort.Strings(wipe)
	fmt.Println("\nWipe (rows now on live):")
	for _, t := range wipe {
		if n := counts[strings.TrimPrefix(t, "td.")]; n > 0 {
			fmt.Printf("  %-26s %d\n", strings.TrimPrefix(t, "td."), n)
		}
	}
	if _, err := tx.ExecContext(ctx, "TRUNCATE "+strings.Join(wipe, ", ")); err != nil {
		return fmt.Errorf("wipe: %w", err)
	}

	// 2. The accounts and their sign-ins.
	playerJSON, err := devPlayerJSON(ctx, dev, carried)
	if err != nil {
		return err
	}
	update := make([]string, 0, len(playerColumns))
	for _, c := range playerColumns[1:] {
		update = append(update, c+" = EXCLUDED."+c)
	}
	cols := strings.Join(playerColumns, ", ")
	if _, err := tx.ExecContext(ctx, `INSERT INTO td.players (`+cols+`) OVERRIDING SYSTEM VALUE
		SELECT `+cols+` FROM json_populate_recordset(NULL::td.players, $1::json)
		ON CONFLICT (id) DO UPDATE SET `+strings.Join(update, ", "), playerJSON); err != nil {
		return fmt.Errorf("players: %w", err)
	}
	identJSON, err := devIdentityJSON(ctx, dev, carried)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO td.player_identities (provider, subject, player_id, email, created_at)
		SELECT provider, subject, player_id, email, created_at FROM json_populate_recordset(NULL::td.player_identities, $1::json)
		ON CONFLICT (provider, subject) DO UPDATE SET email = EXCLUDED.email`, identJSON); err != nil {
		return fmt.Errorf("sign-ins: %w", err)
	}

	// 3. Everyone starts again: level 1, no faction, no sanctions, nothing pending.
	res, err := tx.ExecContext(ctx, `UPDATE td.players SET level = 1, xp = 0, contribution_points = 0,
		walked_together_m = 0, protection_lost_until = NULL, suspended_until = NULL, bound_faction_code = NULL,
		status = 'active', updated_at = now()
		WHERE type = 'player' AND status IN ('active', 'suspended')`)
	if err != nil {
		return fmt.Errorf("reset players: %w", err)
	}
	reset, _ := res.RowsAffected()
	if _, err := tx.ExecContext(ctx, `UPDATE td.players SET level = 1, xp = 0 WHERE type <> 'player'`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE td.factions SET status = 'locked', leader_player_id = NULL, claimed_at = NULL`); err != nil {
		return fmt.Errorf("factions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `SELECT setval(pg_get_serial_sequence('td.players', 'id'),
		GREATEST((SELECT max(id) FROM td.players), 10))`); err != nil {
		return fmt.Errorf("id counter: %w", err)
	}

	// 4. The map: test nodes go, the rest are full again.
	var removed int64
	if !keepTestNodes {
		res, err := tx.ExecContext(ctx, `DELETE FROM td.nodes WHERE source IN ('scatter', 'rare', 'group')`)
		if err != nil {
			return fmt.Errorf("test nodes: %w", err)
		}
		removed, _ = res.RowsAffected()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE td.nodes SET reserve = 1, updated_at = now() WHERE reserve < 1`); err != nil {
		return fmt.Errorf("refill nodes: %w", err)
	}

	// 5. What a new account gets (service/accounts.go SignIn): starter gear, welcome resources and cells.
	active := `SELECT id FROM td.players WHERE type = 'player' AND status = 'active'`
	if _, err := tx.ExecContext(ctx, `INSERT INTO td.player_gear (player_id, gear_type_id, slot, durability, equipped, acquired_from)
		SELECT p.id, gt.id, gt.slot, gt.max_durability, true, 'starter'
		FROM (`+active+`) p CROSS JOIN td.gear_types gt WHERE gt.id IN ('scanner_mk1', 'extractor_mk1')`); err != nil {
		return fmt.Errorf("starter gear: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO td.player_uploaded (player_id, resource, amount)
		SELECT p.id, w.resource, s.value::bigint
		FROM (`+active+`) p
		CROSS JOIN (VALUES ('crystal', 'WelcomeCrystal'), ('alloy', 'WelcomeAlloy'), ('data', 'WelcomeData'),
		                   ('credit', 'WelcomeCredit')) w(resource, key)
		JOIN td.settings s ON s.key = w.key WHERE s.value::numeric > 0`); err != nil {
		return fmt.Errorf("welcome resources: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO td.player_items (player_id, item, count)
		SELECT p.id, 'energy_cell', s.value::int FROM (`+active+`) p
		JOIN td.settings s ON s.key = 'WelcomeCells' WHERE s.value::numeric > 0`); err != nil {
		return fmt.Errorf("welcome cells: %w", err)
	}

	// 6. The tester badge.
	var badged []int64
	if badge != "" {
		for _, id := range carried {
			if !contains(noBadge, id) {
				badged = append(badged, id)
			}
		}
		for _, id := range badged {
			if _, err := tx.ExecContext(ctx, `INSERT INTO td.player_badges (player_id, badge, faction_id, region)
				VALUES ($1, $2, NULL, '') ON CONFLICT DO NOTHING`, id, badge); err != nil {
				return fmt.Errorf("badge: %w", err)
			}
		}
	}

	detail, _ := json.Marshal(map[string]any{"carried": carried, "skipped": skip, "badge": badge, "badged": badged,
		"testNodesRemoved": removed, "from": devName})
	if _, err := tx.ExecContext(ctx, `INSERT INTO td.admin_actions (admin_id, action, target, detail)
		VALUES (1, 'dev2live.sync', 'live', $1::jsonb)`, string(detail)); err != nil {
		return fmt.Errorf("admin log: %w", err)
	}

	fmt.Printf("\nPlayers reset to level 1 with the starter kit: %d\n", reset)
	fmt.Printf("Test, rare and group nodes removed: %d\n", removed)
	if badge != "" {
		fmt.Printf("Badge %q (%s) to: %v\n", badge, strings.ToUpper(badge), badged)
	}
	fmt.Println("All sessions and push tokens removed: every phone signs in again.")

	if !apply {
		fmt.Println("\nDRY RUN: rolled back, live is unchanged. Run again with -apply to do it.")
		return nil
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Printf("\nDONE: live synced from dev at %s.\n", time.Now().UTC().Format(time.RFC3339))
	return nil
}

type devPlayer struct {
	ID    int64
	Name  string
	Level int
}

// devPlayers lists the players to carry and, per player, their sign-ins as "provider:md5(subject)" (the
// subject itself is never printed).
func devPlayers(ctx context.Context, dev *sql.DB, skip []int64) ([]devPlayer, map[int64][]string, error) {
	rows, err := dev.QueryContext(ctx, `
		SELECT p.id, COALESCE(p.callsign, 'OPERATIVE-' || p.public_code), p.level, i.provider || ':' || md5(i.subject)
		FROM td.players p JOIN td.player_identities i ON i.player_id = p.id
		WHERE p.type = 'player' AND p.status IN ('active', 'suspended') AND i.provider IN ('google', 'apple')
		  AND NOT (p.id = ANY($1))
		ORDER BY p.id, i.provider`, skip)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var out []devPlayer
	idents := map[int64][]string{}
	for rows.Next() {
		var p devPlayer
		var ident string
		if err := rows.Scan(&p.ID, &p.Name, &p.Level, &ident); err != nil {
			return nil, nil, err
		}
		if _, seen := idents[p.ID]; !seen {
			out = append(out, p)
		}
		idents[p.ID] = append(idents[p.ID], ident)
	}
	return out, idents, rows.Err()
}

func providers(idents []string) string {
	out := make([]string, 0, len(idents))
	for _, i := range idents {
		out = append(out, strings.SplitN(i, ":", 2)[0])
	}
	return strings.Join(out, ", ")
}

func devPlayerJSON(ctx context.Context, dev *sql.DB, ids []int64) (string, error) {
	var out sql.NullString
	err := dev.QueryRowContext(ctx, `SELECT json_agg(row_to_json(p)) FROM td.players p WHERE p.id = ANY($1)`, ids).Scan(&out)
	if !out.Valid {
		return "[]", err
	}
	return out.String, err
}

func devIdentityJSON(ctx context.Context, dev *sql.DB, ids []int64) (string, error) {
	var out sql.NullString
	err := dev.QueryRowContext(ctx, `SELECT json_agg(row_to_json(i)) FROM td.player_identities i
		WHERE i.player_id = ANY($1) AND i.provider IN ('google', 'apple')`, ids).Scan(&out)
	if !out.Valid {
		return "[]", err
	}
	return out.String, err
}

// checkTables stops on a table the job doesn't know: new tables need a decision, keep or wipe.
func checkTables(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'td'`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var unknown []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return err
		}
		if !keepTables[t] && !wipeTables[t] {
			unknown = append(unknown, t)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("tables not in keepTables or wipeTables (cmd/dev2live/main.go): %s", strings.Join(unknown, ", "))
	}
	return nil
}

// checkConflicts makes sure a dev account lands on the same person on live: no sign-in that live gives
// to another id, no id that live gives to someone else, and (unless allowed) no live player dev doesn't have.
func checkConflicts(ctx context.Context, tx *sql.Tx, players []devPlayer, idents map[int64][]string, allowLiveOnly bool) error {
	carried := map[int64]bool{}
	for _, p := range players {
		carried[p.ID] = true
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT p.id, p.type::text, i.provider || ':' || md5(i.subject)
		FROM td.players p LEFT JOIN td.player_identities i ON i.player_id = p.id`)
	if err != nil {
		return err
	}
	liveKind := map[int64]string{}
	liveIdents := map[int64][]string{}
	identOwner := map[string]int64{} // a sign-in on live and the id it opens
	for rows.Next() {
		var id int64
		var kind string
		var ident sql.NullString
		if err := rows.Scan(&id, &kind, &ident); err != nil {
			rows.Close()
			return err
		}
		liveKind[id] = kind
		if ident.Valid {
			liveIdents[id] = append(liveIdents[id], ident.String)
			identOwner[ident.String] = id
		}
	}
	rows.Close()

	for _, p := range players {
		// The same sign-in must open the same id on both.
		for _, ident := range idents[p.ID] {
			if owner, ok := identOwner[ident]; ok && owner != p.ID {
				return fmt.Errorf("dev player %d's %s sign-in opens player %d on live", p.ID, providers([]string{ident}), owner)
			}
		}
		// An id live already has must be the same person (one of dev's sign-ins).
		if kind, ok := liveKind[p.ID]; ok {
			if kind != "player" {
				return fmt.Errorf("id %d is a %s account on live", p.ID, kind)
			}
			for _, ident := range liveIdents[p.ID] {
				if !containsString(idents[p.ID], ident) {
					return fmt.Errorf("id %d on live is someone else (a %s sign-in dev doesn't have)", p.ID, providers([]string{ident}))
				}
			}
		}
	}

	var liveOnly []string
	for id, kind := range liveKind {
		if kind == "player" && !carried[id] && len(liveIdents[id]) > 0 {
			liveOnly = append(liveOnly, strconv.FormatInt(id, 10))
		}
	}
	if len(liveOnly) > 0 {
		sort.Strings(liveOnly)
		if !allowLiveOnly {
			return fmt.Errorf("live has players dev doesn't (%s): after launch this job must never run; "+
				"before launch, pass -allow-live-only to keep them (reset like the rest)", strings.Join(liveOnly, ", "))
		}
		fmt.Printf("Live-only players kept and reset: %s\n", strings.Join(liveOnly, ", "))
	}
	return nil
}

func tableCounts(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'td'`)
	if err != nil {
		return nil, err
	}
	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			rows.Close()
			return nil, err
		}
		tables = append(tables, t)
	}
	rows.Close()
	out := map[string]int64{}
	for _, t := range tables {
		var n int64
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM td.`+t).Scan(&n); err != nil {
			return nil, err
		}
		out[t] = n
	}
	return out, nil
}

func open(env string) (*sql.DB, error) {
	url := os.Getenv(env)
	if url == "" {
		return nil, errors.New(env + " is not set")
	}
	return sql.Open("pgx", url)
}

func dbName(ctx context.Context, db *sql.DB) string {
	var name string
	if err := db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		return "?" + err.Error()
	}
	return name
}

func migrations(ctx context.Context, db *sql.DB) string {
	var n int
	var last sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT count(*), max(name) FROM td.schema_migrations`).Scan(&n, &last); err != nil {
		return "?" + err.Error()
	}
	return fmt.Sprintf("%d up to %s", n, last.String)
}

func set(items ...string) map[string]bool {
	out := make(map[string]bool, len(items))
	for _, i := range items {
		out[i] = true
	}
	return out
}

func ids(list string) []int64 {
	out := []int64{}
	for _, part := range strings.Split(list, ",") {
		if id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func contains(list []int64, id int64) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}
