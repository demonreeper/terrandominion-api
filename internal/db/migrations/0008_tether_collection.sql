-- 0008 tether collection: what a Tether Node gathers on its own, the half it holds for its owner,
-- and uploads through it.

-- Lifetime total per link, for the info panel.
ALTER TABLE td.tether_links ADD COLUMN gathered bigint NOT NULL DEFAULT 0 CHECK (gathered >= 0);

-- The held half, waiting for the owner to collect it. No storage cap.
CREATE TABLE td.tether_stock (
    tn_id    bigint NOT NULL REFERENCES td.tether_nodes (id) ON DELETE CASCADE,
    resource text NOT NULL REFERENCES td.resource_types (code),
    amount   bigint NOT NULL DEFAULT 0 CHECK (amount >= 0),
    PRIMARY KEY (tn_id, resource)
);

-- Every collect and upload with the position the phone reported: the audit trail for GPS spoofing.
CREATE TABLE td.tether_transfers (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tn_id      bigint NOT NULL REFERENCES td.tether_nodes (id) ON DELETE CASCADE,
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('collect', 'upload')),
    amounts    jsonb NOT NULL,   -- what the player received
    fees       jsonb NOT NULL DEFAULT '{}',   -- what the Tether Node's owner received
    reported   geography (Point, 4326) NOT NULL,
    accuracy_m real NOT NULL,
    distance_m real NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX tether_transfers_player_idx ON td.tether_transfers (player_id, created_at DESC);

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('TnOwnerShare', 0.5,  true, 'Share of each Tether Node collection sent straight to the owner''s uploaded inventory'),
    ('TnUploadFee',  0.05, true, 'Charge on other players'' uploads through a Tether Node, paid to its owner');
