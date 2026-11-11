-- 0058 signal echoes (docs/no-ar-support-plan.md, "AR sees more", milestone 7): faint traces in the
-- Undersignal, seen and read only in the AR view, placed by the server beside nodes and Tether Nodes, a
-- few per area, moved now and then. Reading one stores the line in the player's CODEX; a finished set of
-- five gives a profile badge (codex_set_<n>). No resources, no XP, no combat information.

CREATE TABLE td.signal_echoes (
    id        bigserial PRIMARY KEY,
    location  geography(Point, 4326) NOT NULL,
    -- The seeding cell (EchoCellM), so a cell is filled and moved on its own.
    cell      text NOT NULL,
    -- Which line it carries: set and line count from 1 (game.EchoSets).
    echo_set  int NOT NULL CHECK (echo_set >= 1),
    line      int NOT NULL CHECK (line >= 1),
    placed_at timestamptz NOT NULL DEFAULT now(),
    -- When the Undersignal moves it: after this it is gone and the cell gets a new one.
    moves_at  timestamptz NOT NULL
);
CREATE INDEX signal_echoes_location_gix ON td.signal_echoes USING gist (location);
CREATE INDEX signal_echoes_cell_idx ON td.signal_echoes (cell);

-- The CODEX: one row per line a player has read, kept for good.
CREATE TABLE td.echo_reads (
    player_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    echo_set  int NOT NULL,
    line      int NOT NULL,
    read_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (player_id, echo_set, line)
);

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('EchoesPerCell',   3,   false, 'echoes', 'Signal echoes per cell',               0, 20,   1,
     'How many echoes stand in each echo cell at once (0 turns echoes off)'),
    ('EchoCellM',       400, false, 'echoes', 'Echo cell size (metres)',              100, 2000, 50,
     'The square each cell covers; echoes are filled and moved cell by cell'),
    ('EchoMoveHours',   24,  false, 'echoes', 'Echo moves after (hours)',             1, 720,  1,
     'How long an echo stays before the Undersignal moves it somewhere else'),
    ('EchoViewRadiusM', 150, false, 'echoes', 'Echoes seen within (metres)',          20, 1000, 10,
     'How far away the AR view shows echoes'),
    ('EchoOffsetMinM',  6,   false, 'echoes', 'Echo distance from its node, least (metres)', 0, 100, 1,
     'An echo is placed beside a node or Tether Node, at least this far from it'),
    ('EchoOffsetMaxM',  18,  false, 'echoes', 'Echo distance from its node, most (metres)',  0, 100, 1,
     'and at most this far')
ON CONFLICT (key) DO NOTHING;
