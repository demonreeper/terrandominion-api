package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/demonreeper/terrandominion/api/internal/db"
)

type TetherLink struct {
	Slot     int     `json:"slot"`
	NodeID   int64   `json:"nodeId"`
	Resource string  `json:"resource"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
}

type Tether struct {
	ID        int64        `json:"id"`
	OwnerID   int64        `json:"ownerId"`
	OwnerName string       `json:"ownerName"`
	Mine      bool         `json:"mine"`
	FactionID *int         `json:"factionId"`
	Lat       float64      `json:"lat"`
	Lon       float64      `json:"lon"`
	Tier      int          `json:"tier"`
	Health    float64      `json:"health"`
	BuiltAt   time.Time    `json:"builtAt"`
	ExpiresAt time.Time    `json:"expiresAt"`
	DistanceM float64      `json:"distanceM"`
	Links     []TetherLink `json:"links"`
	// Stock is the held half. Only the owner is told what is inside.
	Stock map[string]int64 `json:"stock,omitempty"`
}

// tetherColumns must stay in step with scanTether; the distance column is added by each query.
const tetherColumns = `
	t.id, t.owner_id, p.display_name, t.faction_id, ST_Y(t.location::geometry), ST_X(t.location::geometry),
	t.tier, t.health, t.built_at, t.expires_at,
	COALESCE((
	    SELECT jsonb_agg(jsonb_build_object(
	        'slot', l.slot, 'nodeId', n.id, 'resource', n.resource,
	        'lat', ST_Y(n.location::geometry), 'lon', ST_X(n.location::geometry)) ORDER BY l.slot)
	    FROM td.tether_links l JOIN td.nodes n ON n.id = l.node_id
	    WHERE l.tn_id = t.id), '[]'::jsonb),
	COALESCE((SELECT jsonb_object_agg(s.resource, s.amount) FROM td.tether_stock s WHERE s.tn_id = t.id), '{}'::jsonb)`

func scanTether(row interface{ Scan(...any) error }, viewerID int64) (Tether, error) {
	var t Tether
	var faction sql.NullInt32
	var links, stock []byte
	if err := row.Scan(&t.ID, &t.OwnerID, &t.OwnerName, &faction, &t.Lat, &t.Lon, &t.Tier, &t.Health,
		&t.BuiltAt, &t.ExpiresAt, &links, &stock, &t.DistanceM); err != nil {
		return t, err
	}
	if faction.Valid {
		id := int(faction.Int32)
		t.FactionID = &id
	}
	t.Mine = t.OwnerID == viewerID
	if t.Mine {
		if err := json.Unmarshal(stock, &t.Stock); err != nil {
			return t, err
		}
	}
	return t, json.Unmarshal(links, &t.Links)
}

// NearbyTethers lists active Tether Nodes around a point, nearest first.
func NearbyTethers(ctx context.Context, q db.Querier, viewerID int64, lat, lon, radiusM float64, limit int) ([]Tether, error) {
	rows, err := q.QueryContext(ctx, `
		WITH here AS (SELECT ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography AS g)
		SELECT `+tetherColumns+`, ST_Distance(t.location, here.g)
		FROM td.tether_nodes t
		JOIN td.players p ON p.id = t.owner_id
		CROSS JOIN here
		WHERE t.status = 'active' AND ST_DWithin(t.location, here.g, $3)
		ORDER BY t.location <-> here.g
		LIMIT $4`, lat, lon, radiusM, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Tether, 0, 8)
	for rows.Next() {
		t, err := scanTether(rows, viewerID)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// TetherByID reads one Tether Node; its distance is reported as 0.
func TetherByID(ctx context.Context, q db.Querier, viewerID, id int64) (Tether, error) {
	t, err := scanTether(q.QueryRowContext(ctx, `
		SELECT `+tetherColumns+`, 0::float8
		FROM td.tether_nodes t JOIN td.players p ON p.id = t.owner_id
		WHERE t.id = $1`, id), viewerID)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

func CountActiveTethers(ctx context.Context, q db.Querier, ownerID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx,
		`SELECT count(*) FROM td.tether_nodes WHERE owner_id = $1 AND status = 'active'`, ownerID).Scan(&n)
	return n, err
}

// PickFreeNodes chooses random active nodes in range that no Tether Node holds. Chosen nodes are
// locked and locked ones skipped, so two simultaneous builds cannot pick the same node.
func PickFreeNodes(ctx context.Context, q db.Querier, lat, lon, radiusM, minReserve float64, limit int) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT n.id FROM td.nodes n
		WHERE n.status = 'active' AND n.reserve >= $4
		  AND ST_DWithin(n.location, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $3)
		  AND NOT EXISTS (SELECT 1 FROM td.tether_links l WHERE l.node_id = n.id)
		ORDER BY random()
		LIMIT $5
		FOR UPDATE OF n SKIP LOCKED`, lat, lon, radiusM, minReserve, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0, limit)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

type NewTether struct {
	OwnerID   int64
	FactionID *int
	Lat, Lon  float64
	AccuracyM float64
	Health    float64
	Spent     map[string]int64
	ExpiresAt time.Time
}

func InsertTether(ctx context.Context, q db.Querier, t NewTether) (int64, error) {
	spent, err := json.Marshal(t.Spent)
	if err != nil {
		return 0, err
	}
	var id int64
	err = q.QueryRowContext(ctx, `
		INSERT INTO td.tether_nodes (owner_id, faction_id, location, accuracy_m, health, spent, expires_at)
		VALUES ($1, $2, ST_SetSRID(ST_MakePoint($4, $3), 4326)::geography, $5, $6, $7::jsonb, $8)
		RETURNING id`, t.OwnerID, t.FactionID, t.Lat, t.Lon, t.AccuracyM, t.Health, string(spent), t.ExpiresAt).Scan(&id)
	return id, err
}

func InsertTetherLink(ctx context.Context, q db.Querier, tetherID int64, slot int, nodeID int64) error {
	_, err := q.ExecContext(ctx,
		`INSERT INTO td.tether_links (tn_id, slot, node_id) VALUES ($1, $2, $3)`, tetherID, slot, nodeID)
	return err
}

// TetherState is the part of a Tether Node that decisions depend on.
type TetherState struct {
	OwnerID   int64
	FactionID *int
	Lat, Lon  float64
	Status    string
	Spent     map[string]int64
}

func TetherForUpdate(ctx context.Context, q db.Querier, id int64) (TetherState, error) {
	var t TetherState
	var spent []byte
	var faction sql.NullInt32
	err := q.QueryRowContext(ctx, `
		SELECT owner_id, faction_id, ST_Y(location::geometry), ST_X(location::geometry), status, spent
		FROM td.tether_nodes WHERE id = $1 FOR UPDATE`, id).
		Scan(&t.OwnerID, &faction, &t.Lat, &t.Lon, &t.Status, &spent)
	if errors.Is(err, sql.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, err
	}
	if faction.Valid {
		id := int(faction.Int32)
		t.FactionID = &id
	}
	return t, json.Unmarshal(spent, &t.Spent)
}

// TetherOwner reads who owns a Tether Node without locking it, so a transaction can lock the
// players involved before the node. Ownership never changes.
func TetherOwner(ctx context.Context, q db.Querier, id int64) (int64, error) {
	var owner int64
	err := q.QueryRowContext(ctx, `SELECT owner_id FROM td.tether_nodes WHERE id = $1`, id).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return owner, err
}

// DueTether is a Tether Node with at least one link ready to harvest.
type DueTether struct {
	ID, OwnerID int64
}

// DueTethers lists Tether Nodes with a link ready. One past its lifetime gathers nothing more.
func DueTethers(ctx context.Context, q db.Querier) ([]DueTether, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT t.id, t.owner_id FROM td.tether_nodes t
		WHERE t.status = 'active' AND t.expires_at > now()
		  AND EXISTS (SELECT 1 FROM td.tether_links l WHERE l.tn_id = t.id AND l.next_harvest_at <= now())
		ORDER BY t.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DueTether, 0, 16)
	for rows.Next() {
		var d DueTether
		if err := rows.Scan(&d.ID, &d.OwnerID); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DueLink is one link ready to harvest.
type DueLink struct {
	Slot   int
	NodeID int64
}

func DueLinksForUpdate(ctx context.Context, q db.Querier, tetherID int64) ([]DueLink, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT slot, node_id FROM td.tether_links
		WHERE tn_id = $1 AND next_harvest_at <= now()
		ORDER BY slot FOR UPDATE`, tetherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]DueLink, 0, 4)
	for rows.Next() {
		var l DueLink
		if err := rows.Scan(&l.Slot, &l.NodeID); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// RecordLinkHarvest adds to the link's total and starts its next cooldown.
func RecordLinkHarvest(ctx context.Context, q db.Querier, tetherID int64, slot int, amount int64, cooldownSec int) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.tether_links
		SET gathered = gathered + $3, next_harvest_at = now() + make_interval(secs => $4)
		WHERE tn_id = $1 AND slot = $2`, tetherID, slot, amount, cooldownSec)
	return err
}

func AddTetherStock(ctx context.Context, q db.Querier, tetherID int64, resource string, amount int64) error {
	_, err := q.ExecContext(ctx, `
		INSERT INTO td.tether_stock (tn_id, resource, amount) VALUES ($1, $2, $3)
		ON CONFLICT (tn_id, resource) DO UPDATE SET amount = td.tether_stock.amount + $3`,
		tetherID, resource, amount)
	return err
}

// TakeTetherStock empties a Tether Node's held half and returns what was in it.
func TakeTetherStock(ctx context.Context, q db.Querier, tetherID int64) (map[string]int64, error) {
	rows, err := q.QueryContext(ctx,
		`DELETE FROM td.tether_stock WHERE tn_id = $1 AND amount > 0 RETURNING resource, amount`, tetherID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var resource string
		var amount int64
		if err := rows.Scan(&resource, &amount); err != nil {
			return nil, err
		}
		out[resource] = amount
	}
	return out, rows.Err()
}

// TetherTransfer is the evidence kept for one collect or upload.
type TetherTransfer struct {
	TetherID, PlayerID             int64
	Kind                           string
	Amounts, Fees                  map[string]int64
	Lat, Lon, AccuracyM, DistanceM float64
}

func InsertTetherTransfer(ctx context.Context, q db.Querier, t TetherTransfer) error {
	amounts, err := json.Marshal(t.Amounts)
	if err != nil {
		return err
	}
	fees, err := json.Marshal(t.Fees)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx, `
		INSERT INTO td.tether_transfers (tn_id, player_id, kind, amounts, fees, reported, accuracy_m, distance_m)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, ST_SetSRID(ST_MakePoint($7, $6), 4326)::geography, $8, $9)`,
		t.TetherID, t.PlayerID, t.Kind, string(amounts), string(fees), t.Lat, t.Lon, t.AccuracyM, t.DistanceM)
	return err
}

// EndTether closes a Tether Node with a final status and frees the nodes it held.
func EndTether(ctx context.Context, q db.Querier, id int64, status string) error {
	if _, err := q.ExecContext(ctx, `
		UPDATE td.tether_nodes SET status = $2::td.tether_status, ended_at = now() WHERE id = $1`, id, status); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `DELETE FROM td.tether_links WHERE tn_id = $1`, id)
	return err
}
