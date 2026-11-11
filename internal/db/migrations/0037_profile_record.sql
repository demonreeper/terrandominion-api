-- 0037 profile record and 48 h map drops (docs/profile-plan.md, P1 and side fix 3).

-- Who destroyed a Tether Node, for the profile's service record ("Tether Nodes destroyed"). Filled
-- from here on by the attack, spike and drone code; older ones come from the owner's notice.
ALTER TABLE td.tether_nodes
    ADD COLUMN destroyed_by bigint REFERENCES td.players (id) ON DELETE SET NULL;

UPDATE td.tether_nodes t
SET destroyed_by = n.actor_id
FROM (
    SELECT DISTINCT ON (tn_id) tn_id, actor_id
    FROM td.notices
    WHERE kind = 'destroyed' AND tn_id IS NOT NULL AND actor_id IS NOT NULL
    ORDER BY tn_id, id DESC
) n
WHERE t.id = n.tn_id AND t.status = 'destroyed';

CREATE INDEX tether_nodes_destroyed_by_idx ON td.tether_nodes (destroyed_by) WHERE destroyed_by IS NOT NULL;
CREATE INDEX tether_nodes_owner_all_idx ON td.tether_nodes (owner_id);
CREATE INDEX player_missions_done_idx ON td.player_missions (player_id) WHERE status IN ('done', 'claimed');

-- A map drop now lasts as long as a Tether Node (48 h); after that the Undersignal reclaims it.
-- Drops already on the map keep their old end time.
UPDATE td.settings
SET value = 48, description = 'Hours a map drop stays before the Undersignal reclaims it', updated_at = now()
WHERE key = 'DropLifetimeHours' AND value = 24;
