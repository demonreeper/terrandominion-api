package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
	ID      int64 `json:"id"`
	OwnerID int64 `json:"-"`
	// Owner is who built it, as other players see them (their profile opens from the code).
	Owner     PlayerRef `json:"owner"`
	Mine      bool      `json:"mine"`
	FactionID *int      `json:"factionId"`
	// FactionColor is the owner's faction colour (#rrggbb), empty for a freelancer. Looks only: the
	// living Tether Nodes in AR wear it (no-ar-support-plan.md milestone 5); profiles show the faction anyway.
	FactionColor string       `json:"factionColor,omitempty"`
	Lat          float64      `json:"lat"`
	Lon          float64      `json:"lon"`
	Tier         int          `json:"tier"`
	Status       string       `json:"status"`
	Health       float64      `json:"health"`
	BuiltAt      time.Time    `json:"builtAt"`
	ExpiresAt    time.Time    `json:"expiresAt"`
	DistanceM    float64      `json:"distanceM"`
	Links        []TetherLink `json:"links"`

	JammedUntil    *time.Time `json:"jammedUntil,omitempty"`
	AttackedAt     *time.Time `json:"attackedAt,omitempty"`
	SpikeFiresAt   *time.Time `json:"spikeFiresAt,omitempty"`
	DroneArrivesAt *time.Time `json:"droneArrivesAt,omitempty"`
	// Stock is the held half. Only the owner is told what is inside.
	Stock map[string]int64 `json:"stock,omitempty"`
	// ShieldTier is the fitted shield's Mk (0 = none); everyone sees it.
	ShieldTier int `json:"shieldTier"`
	// Drones flying at it, soonest first, with where each was launched from. Only the owner is told.
	Drones []InboundDroneView `json:"drones,omitempty"`
	// SpeedFrom is when the slowdown clock started: the owner's last collect, or the build.
	// SpeedPct is how fast it gathers now (100 = full speed); the service fills it, for the owner only.
	SpeedFrom time.Time `json:"-"`
	SpeedPct  int       `json:"speedPct,omitempty"`
}

// InboundDroneView is a drone flying at a Tether Node, as its owner sees it: where it came from,
// not who sent it.
type InboundDroneView struct {
	ID          int64     `json:"id"`
	Mk          int       `json:"mk"`
	LaunchedAt  time.Time `json:"launchedAt"`
	ArrivesAt   time.Time `json:"arrivesAt"`
	From        *Point    `json:"from"`
	Intercepted bool      `json:"intercepted"`
}

// inboundDronesSQL is the JSON list of drones flying at the Tether Node aliased t.
const inboundDronesSQL = `COALESCE((
	    SELECT jsonb_agg(jsonb_build_object(
	        'id', ds.id, 'mk', ds.mk, 'launchedAt', ds.created_at, 'arrivesAt', ds.arrives_at,
	        'from', CASE WHEN ds.from_lat IS NULL THEN NULL
	                     ELSE jsonb_build_object('lat', ds.from_lat, 'lon', ds.from_lon) END,
	        'intercepted', ds.intercepted) ORDER BY ds.arrives_at)
	    FROM td.drone_strikes ds WHERE ds.tn_id = t.id AND ds.status = 'flying'), '[]'::jsonb)`

// tetherColumns must stay in step with scanTether; the distance column is added by each query.
const tetherColumns = `
	t.id, t.owner_id, ` + playerRefP + `, t.faction_id, ST_Y(t.location::geometry), ST_X(t.location::geometry),
	t.tier, t.status, t.health, t.built_at, t.expires_at,
	COALESCE((
	    SELECT jsonb_agg(jsonb_build_object(
	        'slot', l.slot, 'nodeId', n.id, 'resource', n.resource,
	        'lat', ST_Y(n.location::geometry), 'lon', ST_X(n.location::geometry)) ORDER BY l.slot)
	    FROM td.tether_links l JOIN td.nodes n ON n.id = l.node_id
	    WHERE l.tn_id = t.id), '[]'::jsonb),
	COALESCE((SELECT jsonb_object_agg(s.resource, s.amount) FROM td.tether_stock s WHERE s.tn_id = t.id), '{}'::jsonb),
	t.jammed_until, t.attacked_at,
	(SELECT sp.fires_at FROM td.tether_spikes sp WHERE sp.tn_id = t.id AND sp.status = 'armed'),
	(SELECT min(ds.arrives_at) FROM td.drone_strikes ds WHERE ds.tn_id = t.id AND ds.status = 'flying'),
	t.shield_tier, ` + inboundDronesSQL + `, COALESCE(t.collected_at, t.built_at),
	COALESCE((SELECT f.color FROM td.factions f WHERE f.id = t.faction_id), '')`

func scanTether(row interface{ Scan(...any) error }, viewerID int64) (Tether, error) {
	var t Tether
	var faction sql.NullInt32
	var links, stock, drones []byte
	var jammed, attacked, spike, drone sql.NullTime
	if err := row.Scan(&t.ID, &t.OwnerID, &t.Owner.Code, &t.Owner.Name, &faction, &t.Lat, &t.Lon, &t.Tier, &t.Status, &t.Health,
		&t.BuiltAt, &t.ExpiresAt, &links, &stock, &jammed, &attacked, &spike, &drone, &t.ShieldTier, &drones,
		&t.SpeedFrom, &t.FactionColor, &t.DistanceM); err != nil {
		return t, err
	}
	t.JammedUntil, t.AttackedAt, t.SpikeFiresAt = timePtr(jammed), timePtr(attacked), timePtr(spike)
	t.DroneArrivesAt = timePtr(drone)
	if faction.Valid {
		id := int(faction.Int32)
		t.FactionID = &id
	}
	t.Mine = t.OwnerID == viewerID
	if t.Mine {
		if err := json.Unmarshal(stock, &t.Stock); err != nil {
			return t, err
		}
		if err := json.Unmarshal(drones, &t.Drones); err != nil {
			return t, err
		}
	}
	return t, json.Unmarshal(links, &t.Links)
}

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
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

// TetherNearby says whether an active Tether Node stands within the distance of a point.
func TetherNearby(ctx context.Context, q db.Querier, lat, lon, distanceM float64) (bool, error) {
	var found bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS (SELECT 1 FROM td.tether_nodes
		WHERE status = 'active'
		  AND ST_DWithin(location, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $3))`,
		lat, lon, distanceM).Scan(&found)
	return found, err
}

func CountActiveTethers(ctx context.Context, q db.Querier, ownerID int64) (int, error) {
	var n int
	err := q.QueryRowContext(ctx,
		`SELECT count(*) FROM td.tether_nodes WHERE owner_id = $1 AND status = 'active'`, ownerID).Scan(&n)
	return n, err
}

// FreeNode is an active node no Tether Node holds.
type FreeNode struct {
	NodeID   int64
	Resource string
}

// freeNodeCandidates caps how many free nodes one pick looks at and locks.
const freeNodeCandidates = 200

// FreeNodes lists active nodes in range that no Tether Node holds, in random order. They are
// locked and locked ones skipped, so two simultaneous builds cannot pick the same node.
func FreeNodes(ctx context.Context, q db.Querier, lat, lon, radiusM, minReserve float64) ([]FreeNode, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT n.id, n.resource FROM td.nodes n
		WHERE n.status = 'active' AND n.reserve >= $4
		  AND ST_DWithin(n.location, ST_SetSRID(ST_MakePoint($2, $1), 4326)::geography, $3)
		  AND NOT EXISTS (SELECT 1 FROM td.tether_links l WHERE l.node_id = n.id)
		ORDER BY random()
		LIMIT $5
		FOR UPDATE OF n SKIP LOCKED`, lat, lon, radiusM, minReserve, freeNodeCandidates)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]FreeNode, 0, 16)
	for rows.Next() {
		var c FreeNode
		if err := rows.Scan(&c.NodeID, &c.Resource); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// TetherLinkResources lists the resource of each linked node, leaving out one slot (use -1 for
// none), which is how a link being moved ignores the depleted node it is leaving.
func TetherLinkResources(ctx context.Context, q db.Querier, id int64, exceptSlot int) ([]string, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT n.resource FROM td.tether_links l JOIN td.nodes n ON n.id = l.node_id
		WHERE l.tn_id = $1 AND l.slot <> $2`, id, exceptSlot)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0, 4)
	for rows.Next() {
		var resource string
		if err := rows.Scan(&resource); err != nil {
			return nil, err
		}
		out = append(out, resource)
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
	OwnerID    int64
	FactionID  *int
	Lat, Lon   float64
	Tier       int
	Status     string
	Spent      map[string]int64
	Health     float64
	ShieldTier int
	// SpeedFrom is when the slowdown clock started: the owner's last collect, or the build.
	SpeedFrom time.Time
}

func TetherForUpdate(ctx context.Context, q db.Querier, id int64) (TetherState, error) {
	var t TetherState
	var spent []byte
	var faction sql.NullInt32
	err := q.QueryRowContext(ctx, `
		SELECT owner_id, faction_id, ST_Y(location::geometry), ST_X(location::geometry), tier, status, spent,
		       health, shield_tier, COALESCE(collected_at, built_at)
		FROM td.tether_nodes WHERE id = $1 FOR UPDATE`, id).
		Scan(&t.OwnerID, &faction, &t.Lat, &t.Lon, &t.Tier, &t.Status, &spent, &t.Health, &t.ShieldTier, &t.SpeedFrom)
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

// MarkTetherCollected notes that the owner collected at the Tether Node now: it gathers at full
// speed again, and every link's wait is cut back to the usual cooldown.
func MarkTetherCollected(ctx context.Context, q db.Querier, id int64, at time.Time, cooldownSec int) error {
	if _, err := q.ExecContext(ctx, `UPDATE td.tether_nodes SET collected_at = $2 WHERE id = $1`, id, at); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `
		UPDATE td.tether_links SET next_harvest_at = LEAST(next_harvest_at, now() + make_interval(secs => $2))
		WHERE tn_id = $1`, id, cooldownSec)
	return err
}

// UpgradeTether raises the tier and records the new total spent.
func UpgradeTether(ctx context.Context, q db.Querier, id int64, tier int, spent map[string]int64) error {
	encoded, err := json.Marshal(spent)
	if err != nil {
		return err
	}
	_, err = q.ExecContext(ctx,
		`UPDATE td.tether_nodes SET tier = $2, spent = $3::jsonb WHERE id = $1`, id, tier, string(encoded))
	return err
}

// TetherLinkSlots lists the slots that hold a node.
func TetherLinkSlots(ctx context.Context, q db.Querier, id int64) ([]int, error) {
	rows, err := q.QueryContext(ctx, `SELECT slot FROM td.tether_links WHERE tn_id = $1 ORDER BY slot`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int, 0, 4)
	for rows.Next() {
		var slot int
		if err := rows.Scan(&slot); err != nil {
			return nil, err
		}
		out = append(out, slot)
	}
	return out, rows.Err()
}

// MoveTetherLink points a slot at a different node. What was gathered belonged to the old node.
func MoveTetherLink(ctx context.Context, q db.Querier, id int64, slot int, nodeID int64) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.tether_links SET node_id = $3, gathered = 0, linked_at = now()
		WHERE tn_id = $1 AND slot = $2`, id, slot, nodeID)
	return err
}

// TetherLinkInfo is what a player standing at a Tether Node sees for one link.
type TetherLinkInfo struct {
	Slot     int    `json:"slot"`
	NodeID   int64  `json:"nodeId"`
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Gathered int64  `json:"gathered"`
}

func TetherLinkInfos(ctx context.Context, q db.Querier, id int64) ([]TetherLinkInfo, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT l.slot, n.id, n.name, n.resource, l.gathered
		FROM td.tether_links l JOIN td.nodes n ON n.id = l.node_id
		WHERE l.tn_id = $1 ORDER BY l.slot`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]TetherLinkInfo, 0, 4)
	for rows.Next() {
		var l TetherLinkInfo
		if err := rows.Scan(&l.Slot, &l.NodeID, &l.Name, &l.Resource, &l.Gathered); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// DecayTethers takes the health lost since each active Tether Node last decayed. Running it twice
// in a row takes nothing extra.
func DecayTethers(ctx context.Context, q db.Querier, perHour float64) error {
	_, err := q.ExecContext(ctx, `
		UPDATE td.tether_nodes
		SET health = GREATEST(0, health - $1 * EXTRACT(EPOCH FROM now() - decayed_at) / 3600), decayed_at = now()
		WHERE status = 'active'`, perHour)
	return err
}

// SpentTethers lists active Tether Nodes out of health or past their lifetime.
func SpentTethers(ctx context.Context, q db.Querier) ([]int64, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id FROM td.tether_nodes
		WHERE status = 'active' AND (health <= 0 OR expires_at <= now()) ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]int64, 0, 8)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// DamageTether takes health from a Tether Node and returns what is left.
func DamageTether(ctx context.Context, q db.Querier, id int64, damage float64) (float64, error) {
	var health float64
	err := q.QueryRowContext(ctx,
		`UPDATE td.tether_nodes SET health = GREATEST(0, health - $2) WHERE id = $1 RETURNING health`,
		id, damage).Scan(&health)
	return health, err
}

// SetShieldTier fits a shield; it lasts as long as the Tether Node.
func SetShieldTier(ctx context.Context, q db.Querier, id int64, tier int) error {
	_, err := q.ExecContext(ctx, `UPDATE td.tether_nodes SET shield_tier = $2 WHERE id = $1`, id, tier)
	return err
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
		WHERE t.status = 'active' AND t.expires_at > now() AND t.health > 0
		  AND (t.jammed_until IS NULL OR t.jammed_until <= now())
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
	if _, err := q.ExecContext(ctx, `UPDATE td.tether_spikes SET status = 'void' WHERE tn_id = $1 AND status = 'armed'`, id); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `UPDATE td.drone_strikes SET status = 'void' WHERE tn_id = $1 AND status = 'flying'`, id); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `DELETE FROM td.tether_links WHERE tn_id = $1`, id)
	return err
}

// MyTetherLink is one node a player's own Tether Node links.
type MyTetherLink struct {
	NodeID   int64  `json:"nodeId"`
	Resource string `json:"resource"`
	Name     string `json:"name"`
}

// MyTether is one of the player's own Tether Nodes, for the My Tether Nodes screen. Timers are null
// when not running; EndedAt is null while active. MaxHealth is filled by the service.
type MyTether struct {
	ID             int64            `json:"id"`
	Name           string           `json:"name"`
	Tier           int              `json:"tier"`
	Lat            float64          `json:"lat"`
	Lon            float64          `json:"lon"`
	Health         float64          `json:"health"`
	MaxHealth      float64          `json:"maxHealth"`
	ExpiresAt      time.Time        `json:"expiresAt"`
	Status         string           `json:"status"`
	EndedAt        *time.Time       `json:"endedAt"`
	Stock          map[string]int64 `json:"stock"`
	Links          []MyTetherLink   `json:"links"`
	JammedUntil    *time.Time       `json:"jammedUntil"`
	SpikeFiresAt   *time.Time       `json:"spikeFiresAt"`
	DroneArrivesAt *time.Time       `json:"droneArrivesAt"`
	AttackedAt     *time.Time       `json:"attackedAt"`
	UnderAttack    bool             `json:"underAttack"`
	ShieldTier     int              `json:"shieldTier"`
	// Drones flying at it, with where each was launched from ([] when none).
	Drones []InboundDroneView `json:"drones"`
	// SpeedFrom is when the slowdown clock started; SpeedPct (filled by the service) is how fast it
	// gathers now, 100 = full speed.
	SpeedFrom time.Time `json:"-"`
	SpeedPct  int       `json:"speedPct"`
}

// MyTethers lists every Tether Node a player owns that is active or ended within endedWithin,
// wherever it is. One hit within attackWindow counts as under attack; placeRadiusM is how far to
// look for a node to name an ended one by.
func MyTethers(ctx context.Context, q db.Querier, ownerID int64, endedWithin, attackWindow time.Duration,
	placeRadiusM float64) ([]MyTether, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT t.id, `+fmt.Sprintf(tetherPlaceSQL, "$4")+`, t.tier,
		       ST_Y(t.location::geometry), ST_X(t.location::geometry), t.health, t.expires_at, t.status, t.ended_at,
		       COALESCE((SELECT jsonb_object_agg(s.resource, s.amount) FROM td.tether_stock s
		                 WHERE s.tn_id = t.id AND s.amount > 0), '{}'::jsonb),
		       COALESCE((SELECT jsonb_agg(jsonb_build_object('nodeId', n.id, 'resource', n.resource, 'name', n.name)
		                 ORDER BY l.slot)
		                 FROM td.tether_links l JOIN td.nodes n ON n.id = l.node_id WHERE l.tn_id = t.id), '[]'::jsonb),
		       CASE WHEN t.status = 'active' AND t.jammed_until > now() THEN t.jammed_until END,
		       (SELECT sp.fires_at FROM td.tether_spikes sp WHERE sp.tn_id = t.id AND sp.status = 'armed'),
		       (SELECT min(ds.arrives_at) FROM td.drone_strikes ds WHERE ds.tn_id = t.id AND ds.status = 'flying'),
		       t.attacked_at,
		       COALESCE(t.status = 'active' AND t.attacked_at > now() - make_interval(secs => $3), false),
		       t.shield_tier, `+inboundDronesSQL+`, COALESCE(t.collected_at, t.built_at)
		FROM td.tether_nodes t
		WHERE t.owner_id = $1
		  AND (t.status = 'active' OR t.ended_at >= now() - make_interval(secs => $2))`,
		ownerID, endedWithin.Seconds(), attackWindow.Seconds(), placeRadiusM)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]MyTether, 0, 4)
	for rows.Next() {
		var t MyTether
		var status string
		var ended, jammed, spike, drone, attacked sql.NullTime
		var stock, links, drones []byte
		if err := rows.Scan(&t.ID, &t.Name, &t.Tier, &t.Lat, &t.Lon, &t.Health, &t.ExpiresAt, &status, &ended,
			&stock, &links, &jammed, &spike, &drone, &attacked, &t.UnderAttack, &t.ShieldTier, &drones, &t.SpeedFrom); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(drones, &t.Drones); err != nil {
			return nil, err
		}
		t.Status = status
		t.EndedAt, t.JammedUntil, t.SpikeFiresAt = timePtr(ended), timePtr(jammed), timePtr(spike)
		t.DroneArrivesAt, t.AttackedAt = timePtr(drone), timePtr(attacked)
		if err := json.Unmarshal(stock, &t.Stock); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(links, &t.Links); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
