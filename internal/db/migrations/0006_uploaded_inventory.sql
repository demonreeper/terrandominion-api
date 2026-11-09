-- 0006 uploaded inventory: a second store per player, separate from carried. It has no carry
-- limit and is safe from the cut a lost fight takes. Tether Node output and uploads land here.
CREATE TABLE td.player_uploaded (
    player_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    resource  text NOT NULL REFERENCES td.resource_types (code),
    amount    bigint NOT NULL DEFAULT 0 CHECK (amount >= 0),
    PRIMARY KEY (player_id, resource)
);
