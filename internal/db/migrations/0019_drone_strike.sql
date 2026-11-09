-- 0019 drone strike: the Drone Bay's decided numbers, and strikes in flight.
UPDATE td.gear_types SET stats = '{"drones": 1, "range_m": 1000, "damage": 15}' WHERE id = 'drone_bay_mk1';

-- A strike flies for a while so the owner gets a warning; the world tick lands it.
CREATE TABLE td.drone_strikes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tn_id      bigint NOT NULL REFERENCES td.tether_nodes (id) ON DELETE CASCADE,
    launched_by bigint NOT NULL REFERENCES td.players (id),
    damage     double precision NOT NULL,
    arrives_at timestamptz NOT NULL,
    status     text NOT NULL DEFAULT 'flying' CHECK (status IN ('flying', 'hit', 'void')),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX drone_strikes_due ON td.drone_strikes (arrives_at) WHERE status = 'flying';
CREATE INDEX drone_strikes_player ON td.drone_strikes (launched_by) WHERE status = 'flying';

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('DroneSpeedMps',      15, true, 'Drone flight speed in metres a second'),
    ('DroneMinFlightSec',  20, true, 'Shortest flight, so the owner always gets some warning'),
    ('CellsDrone',         1,  true, 'Energy cells per drone strike');
