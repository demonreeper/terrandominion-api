-- 0018 combat: weapons against Tether Nodes, node contests, and in-game notices.
ALTER TABLE td.tether_nodes
    ADD COLUMN jammed_until timestamptz,
    ADD COLUMN attacked_at  timestamptz;

-- At most one armed spike per Tether Node. Fuses resolve in the world tick.
CREATE TABLE td.tether_spikes (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tn_id      bigint NOT NULL REFERENCES td.tether_nodes (id) ON DELETE CASCADE,
    planted_by bigint NOT NULL REFERENCES td.players (id),
    fires_at   timestamptz NOT NULL,
    status     text NOT NULL DEFAULT 'armed' CHECK (status IN ('armed', 'fired', 'defused', 'void')),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX tether_spikes_one_armed ON td.tether_spikes (tn_id) WHERE status = 'armed';
CREATE INDEX tether_spikes_due ON td.tether_spikes (fires_at) WHERE status = 'armed';

-- The evidence for every weapon used, as td.harvests is for harvesting.
CREATE TABLE td.tether_attacks (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tn_id       bigint NOT NULL REFERENCES td.tether_nodes (id) ON DELETE CASCADE,
    attacker_id bigint NOT NULL REFERENCES td.players (id),
    weapon      text NOT NULL,
    reported    geography(Point, 4326) NOT NULL,
    accuracy_m  double precision NOT NULL,
    distance_m  double precision NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX tether_attacks_tn_idx ON td.tether_attacks (tn_id, created_at DESC);

-- 'open' runs for ContestSeconds. The winner's row becomes 'held' while they own the node's next
-- harvests, then 'done'.
CREATE TABLE td.node_contests (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    node_id          bigint NOT NULL REFERENCES td.nodes (id) ON DELETE CASCADE,
    challenger_id    bigint NOT NULL REFERENCES td.players (id),
    defender_id      bigint NOT NULL REFERENCES td.players (id),
    status           text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'held', 'done')),
    ends_at          timestamptz NOT NULL,
    winner_id        bigint REFERENCES td.players (id),
    challenger_score integer,
    defender_score   integer,
    prize_left       integer NOT NULL DEFAULT 0,
    hold_until       timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX node_contests_one_open ON td.node_contests (node_id) WHERE status = 'open';
CREATE UNIQUE INDEX node_contests_one_held ON td.node_contests (node_id) WHERE status = 'held';
CREATE INDEX node_contests_due ON td.node_contests (ends_at) WHERE status = 'open';

CREATE TABLE td.node_contest_joins (
    contest_id bigint NOT NULL REFERENCES td.node_contests (id) ON DELETE CASCADE,
    player_id  bigint NOT NULL REFERENCES td.players (id),
    side       text NOT NULL CHECK (side IN ('challenger', 'defender')),
    joined_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contest_id, player_id)
);

CREATE TABLE td.notices (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    kind       text NOT NULL,
    body       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notices_player_idx ON td.notices (player_id, id DESC);

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('WeaponRangeM',         40, true,  'Max distance from a Tether Node to use a weapon or defuse a spike'),
    ('EmPulseDamage',        25, true,  'EM Pulse: damage at once'),
    ('JammerMinutes',        30, true,  'Signal Jammer: minutes the Tether Node collects nothing'),
    ('SpikeDamage',          40, true,  'Overload Spike: damage when it fires'),
    ('SpikeFuseMinutes',     10, true,  'Overload Spike: minutes before it fires'),
    ('CellsEmPulse',         1,  true,  'Energy cells per EM Pulse'),
    ('CellsJammer',          1,  true,  'Energy cells per Signal Jammer'),
    ('CellsSpike',           1,  true,  'Energy cells per Overload Spike'),
    ('ContestSeconds',       60, true,  'How long a node contest runs'),
    ('ContestPrizeHarvests', 3,  true,  'Harvests of the node the winner holds'),
    ('CellsContest',         1,  true,  'Energy cells to start a contest'),
    ('ContestRangeM',        40, true,  'Max distance from the node to start or join a contest'),
    ('ContestRecentMinutes', 5,  true,  'A node can be contested this long after another player harvested it'),
    ('ContestHoldMinutes',   30, true,  'Minutes a won node stays held, so a winner who leaves cannot lock it for ever'),
    ('ContestGearPerTier',   5,  false, 'Contest score per tier of working equipped gear'),
    ('ContestAllyPoints',    5,  false, 'Contest score per faction ally who joined'),
    ('ContestJoinPoints',    5,  false, 'Contest score for a defender who showed up');
