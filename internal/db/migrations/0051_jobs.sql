-- 0051 the JOBS board (docs/freelancer-path-plan.md F6, phase 6): faction players (Warden and above)
-- post paid jobs that only freelancers take: hit a Tether Node, scout an area, guard a node. The
-- reward for every slot is taken from the poster's stored resources when the job is posted (escrow),
-- paid when the server sees the job done, and what is left goes back to the poster when it expires.
CREATE TABLE td.jobs (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    poster_id   bigint REFERENCES td.players (id) ON DELETE SET NULL,
    faction_id  smallint NOT NULL REFERENCES td.factions (id),
    kind        text NOT NULL CHECK (kind IN ('hit', 'scout', 'guard')),
    tn_id       bigint REFERENCES td.tether_nodes (id) ON DELETE SET NULL,  -- hit: the target
    node_id     bigint REFERENCES td.nodes (id) ON DELETE SET NULL,         -- guard: the node
    lat         double precision NOT NULL,                                   -- where (the target's place)
    lon         double precision NOT NULL,
    radius_m    real NOT NULL,
    reward      jsonb NOT NULL,                                              -- per slot: resource -> amount
    slots       integer NOT NULL CHECK (slots > 0),
    done        integer NOT NULL DEFAULT 0,
    status      text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed', 'expired')),
    deadline    timestamptz NOT NULL,
    refunded    jsonb,                                                       -- what went back at the end
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX jobs_open_idx ON td.jobs (deadline) WHERE status = 'open';
CREATE INDEX jobs_faction_idx ON td.jobs (faction_id, created_at DESC);

-- One row per freelancer per job. guard_seconds counts time near the node; last_fix_at the last fix
-- that counted.
CREATE TABLE td.job_takes (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    job_id        bigint NOT NULL REFERENCES td.jobs (id) ON DELETE CASCADE,
    taker_id      bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    status        text NOT NULL DEFAULT 'taken' CHECK (status IN ('taken', 'done', 'failed')),
    guard_seconds integer NOT NULL DEFAULT 0,
    last_fix_at   timestamptz,
    taken_at      timestamptz NOT NULL DEFAULT now(),
    done_at       timestamptz,
    UNIQUE (job_id, taker_id)
);
CREATE INDEX job_takes_taker_idx ON td.job_takes (taker_id) WHERE status = 'taken';

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('JobSlotsMax',             3,    true, 'factions', 'Jobs: slots per job',                1, 10,    1, 'Most freelancers one job can pay'),
    ('JobHoursMin',             1,    true, 'factions', 'Jobs: shortest deadline (hours)',    1, 48,    1, 'Shortest time a job may run'),
    ('JobHoursMax',             24,   true, 'factions', 'Jobs: longest deadline (hours)',     1, 168,   1, 'Longest time a job may run'),
    ('JobRewardMin',            10,   true, 'factions', 'Jobs: smallest reward per slot',     1, 1000,  1, 'A job must pay at least this much, all resources together, per slot'),
    ('JobRewardMax',            2000, true, 'factions', 'Jobs: largest reward per slot',      10, 100000, 10, 'A job may pay at most this much, all resources together, per slot'),
    ('JobsOpenPerFaction',      20,   true, 'factions', 'Jobs: open per faction',             1, 200,   1, 'Most jobs a faction may have open at once'),
    ('JobsPostedPerDay',        5,    true, 'factions', 'Jobs: posted per player per day',    1, 50,    1, 'Most jobs one player may post in 24 hours'),
    ('JobsTakenMax',            3,    true, 'factions', 'Jobs: taken at once per freelancer', 1, 20,    1, 'Most jobs a freelancer may have taken and not finished'),
    ('JobScoutRadiusM',         150,  true, 'factions', 'Jobs: scout area radius (m)',        50, 2000, 10, 'How big the area of a scout job is'),
    ('JobGuardMinutes',         60,   true, 'factions', 'Jobs: guard time (minutes)',         5, 240,   5, 'How long a guard job needs the freelancer near the node'),
    ('JobGuardRadiusM',         40,   true, 'factions', 'Jobs: guard distance (m)',           10, 200,  5, 'How near the node a guard has to be for the time to count'),
    ('JobGuardGapSeconds',      90,   true, 'factions', 'Jobs: guard fix gap (seconds)',      15, 600,  5, 'Time between two location fixes near the node counts only up to this')
ON CONFLICT (key) DO NOTHING;
