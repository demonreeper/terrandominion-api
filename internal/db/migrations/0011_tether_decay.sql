-- 0011 tether decay: health falls over a Tether Node's lifetime, and when one ends what it held
-- drops on the map for anyone to pick up.

ALTER TABLE td.tether_nodes ADD COLUMN decayed_at timestamptz NOT NULL DEFAULT now();

CREATE TABLE td.map_drops (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    location   geography (Point, 4326) NOT NULL,
    contents   jsonb NOT NULL,
    tn_id      bigint REFERENCES td.tether_nodes (id) ON DELETE SET NULL,
    expires_at timestamptz NOT NULL,
    picked_by  bigint REFERENCES td.players (id) ON DELETE SET NULL,
    picked_at  timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX map_drops_location_gix ON td.map_drops USING gist (location) WHERE picked_at IS NULL;

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('DropLifetimeHours', 24, true, 'Hours a map drop stays before it decays');
