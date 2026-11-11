-- 0053 anti GPS spoofing, G0 (docs/social-signal-plan.md Part C). Impossible jumps (and, from G1, the phone's
-- fake-location flag) refuse the action and start a signal lock: a sanction of kind spoof_lock by the
-- "Signal watch" system account, stepping up 30 / 120 / 720 minutes within SpoofRepeatDays. Every incident
-- makes a report in the mod queue. Old or future fixes are refused and noted, with no lock.

-- Signal watch: the system account that files the automatic reports and gives the locks. In the held range.
-- No callsign (system accounts have none); the app names its reports "Signal watch" from their reason.
INSERT INTO td.players (id, display_name, type, country_code)
OVERRIDING SYSTEM VALUE VALUES (9, 'signal_watch', 'system', 'MV')
ON CONFLICT (id) DO NOTHING;

-- New kinds of refused location check. hard: it started (or would start) a lock. dismissed: a mod called it a
-- false alarm, so it no longer counts toward the step-up. evidence: the two fixes, distance and speed; only
-- admins ever see it (moderators get detail, which never holds a position).
ALTER TABLE td.infringements
    DROP CONSTRAINT infringements_kind_check,
    ADD CONSTRAINT infringements_kind_check
        CHECK (kind IN ('gps_weak', 'travel_speed', 'mock_location', 'teleport', 'pattern', 'stale_fix')),
    ADD COLUMN hard      boolean NOT NULL DEFAULT false,
    ADD COLUMN dismissed boolean NOT NULL DEFAULT false,
    ADD COLUMN evidence  jsonb NOT NULL DEFAULT '{}';
CREATE INDEX infringements_hard_idx ON td.infringements (player_id, created_at DESC) WHERE hard AND NOT dismissed;

ALTER TABLE td.player_sanctions
    DROP CONSTRAINT player_sanctions_kind_check,
    ADD CONSTRAINT player_sanctions_kind_check CHECK (kind IN ('warn', 'suspend', 'ban', 'spoof_lock'));
CREATE INDEX player_sanctions_lock_idx ON td.player_sanctions (player_id, until DESC)
    WHERE kind = 'spoof_lock' AND lifted_at IS NULL;

-- Reports from Signal watch carry reason spoof; their context lists the incident ids.
ALTER TABLE td.player_reports
    DROP CONSTRAINT player_reports_reason_check,
    ADD CONSTRAINT player_reports_reason_check CHECK (reason IN ('griefing', 'spoof'));

-- Testers who use a fake location on purpose are never locked (set by an admin, logged).
ALTER TABLE td.players ADD COLUMN spoof_exempt boolean NOT NULL DEFAULT false;

-- The last fix the server accepted from each player, for the teleport check.
CREATE TABLE td.player_last_fix (
    player_id  bigint PRIMARY KEY REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    lat        double precision NOT NULL,
    lon        double precision NOT NULL,
    accuracy_m double precision NOT NULL,
    fixed_at   timestamptz NOT NULL
);

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('SpoofLockEnabled',   1,   false, 'anticheat', 'Signal lock on',                         0, 1,    1,
     '1: a hard signal locks location actions. 0: report only, no lock (if false alarms show up)'),
    ('SpoofCheckMockFlag', 1,   false, 'anticheat', 'Act on the phone''s fake-location flag',  0, 1,    1,
     '1: a fix the phone marks as fake is a hard signal'),
    ('SpoofLockMinutes1',  30,  false, 'anticheat', 'Signal lock: first (minutes)',            1, 10080, 1,
     'Lock for the first incident inside the repeat window'),
    ('SpoofLockMinutes2',  120, false, 'anticheat', 'Signal lock: second (minutes)',           1, 10080, 1,
     'Lock for the second incident inside the repeat window'),
    ('SpoofLockMinutes3',  720, false, 'anticheat', 'Signal lock: third and later (minutes)',  1, 10080, 1,
     'Lock for the third and every later incident inside the repeat window'),
    ('SpoofRepeatDays',    7,   false, 'anticheat', 'Signal lock: repeat window (days)',       1, 90,   1,
     'Incidents this many days apart or less step the lock up'),
    ('SpoofTeleportKmh',   300, false, 'anticheat', 'Teleport: speed (km/h)',                  100, 2000, 10,
     'A jump faster than this (and longer than the shortest jump) is a teleport'),
    ('SpoofTeleportMinKm', 5,   false, 'anticheat', 'Teleport: shortest jump (km)',            1, 100,  0.5,
     'After both fixes'' accuracy; shorter jumps are GPS glitches, never a teleport'),
    ('FixMaxAgeSeconds',   120, false, 'anticheat', 'Oldest fix accepted (seconds)',           10, 900,  5,
     'A fix older than this, or from the future, is refused and noted (no lock)')
ON CONFLICT (key) DO NOTHING;
