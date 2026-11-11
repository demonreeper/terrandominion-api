-- 0059 walk together, W1 (docs/walk-together-plan.md, approved 2026-11-11): two to four accepted friends who
-- are physically together form a signal link; while they keep walking, each gets a bigger scan range, harvest
-- and map drop amount, and harvest XP. The server checks that the phones are side by side on every heartbeat.

CREATE TABLE td.walk_links (
    id         bigserial PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now(),
    ended_at   timestamptz,
    -- left, fell_behind, missed_heartbeats, locked, attack, alone
    end_reason text
);

-- One row per player per link. The latest fix only (overwritten each heartbeat, wiped on leaving); who linked
-- with whom, when, for how long and how far is kept 90 days (moderation and the alt-account report, L5).
CREATE TABLE td.walk_link_members (
    link_id          bigint NOT NULL REFERENCES td.walk_links (id) ON DELETE CASCADE,
    player_id        bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    -- The session's device hash (L6: two accounts on one phone never link).
    device_hash      bytea NOT NULL,
    joined_at        timestamptz NOT NULL DEFAULT now(),
    left_at          timestamptz,
    left_reason      text,
    -- When the server last heard from this member, and their latest fix (fixAt is the phone's time of it).
    last_seen_at     timestamptz NOT NULL DEFAULT now(),
    last_fix_at      timestamptz,
    last_lat         double precision,
    last_lng         double precision,
    last_acc         double precision,
    -- Since when this member has been out of range of every other member (null while in range).
    out_since        timestamptz,
    -- What the last heartbeat decided: the bonus is on, or why not (keep_walking, cap_spent, ...).
    bonus_active     boolean NOT NULL DEFAULT false,
    paused_reason    text,
    bonus_seconds    int NOT NULL DEFAULT 0,
    walked_m         double precision NOT NULL DEFAULT 0,
    PRIMARY KEY (link_id, player_id)
);
CREATE UNIQUE INDEX walk_link_members_live ON td.walk_link_members (player_id) WHERE left_at IS NULL;

-- A member's recent fixes while linked, for "the group has moved WalkLinkMoveM in WalkLinkMoveWindowSec".
-- Pruned as it goes; never more than a few minutes kept.
CREATE TABLE td.walk_link_trail (
    player_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    at        timestamptz NOT NULL,
    lat       double precision NOT NULL,
    lng       double precision NOT NULL
);
CREATE INDEX walk_link_trail_player ON td.walk_link_trail (player_id, at);

-- An ask: from asks to to walk together. Gone after 2 minutes.
CREATE TABLE td.walk_link_asks (
    from_player bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    to_player   bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    created_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (from_player, to_player)
);

-- Who has the WALK TOGETHER panel open right now, with their fix, so the LINK list only offers friends who
-- also have it open and stand within reach. Overwritten on each open; stale after a minute.
CREATE TABLE td.walk_open (
    player_id   bigint PRIMARY KEY REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    device_hash bytea NOT NULL,
    opened_at   timestamptz NOT NULL DEFAULT now(),
    fix_at      timestamptz NOT NULL,
    lat         double precision NOT NULL,
    lng         double precision NOT NULL,
    acc         double precision NOT NULL
);

-- The daily bonus cap counts across links: bonus-active seconds per player per mission day.
CREATE TABLE td.walk_bonus_days (
    player_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    day       date NOT NULL,
    seconds   int NOT NULL DEFAULT 0,
    PRIMARY KEY (player_id, day)
);

-- The profile stat WALKED TOGETHER: bonus-active metres walked linked.
ALTER TABLE td.players ADD COLUMN walked_together_m double precision NOT NULL DEFAULT 0;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('WalkLinkEnabled',       1,   true,  'walk', 'Walk together on',                         0, 1,    1,
     '1: friends can form a signal link. 0: nobody can (existing links end on their next heartbeat)'),
    ('WalkLinkMaxSize',       4,   true,  'walk', 'Most players in a link',                   2, 4,    1,
     'A link holds 2 to this many friends'),
    ('WalkLinkFormM',         30,  true,  'walk', 'Link forms within (metres)',               5, 200,  5,
     'Two phones must be this close (after their accuracy) to link'),
    ('WalkLinkHoldM',         60,  true,  'walk', 'Link holds within (metres)',               10, 300, 5,
     'Every member must stay this close to at least one other member'),
    ('WalkLinkGraceSec',      45,  true,  'walk', 'Out of range or silent for (seconds)',     10, 600, 5,
     'A member out of range, or missing heartbeats, this long leaves the link'),
    ('WalkLinkHeartbeatSec',  15,  true,  'walk', 'Heartbeat every (seconds)',                5, 60,   1,
     'How often a linked phone reports its fix'),
    ('WalkLinkMaxAccuracyM',  25,  true,  'walk', 'Fix accuracy needed (metres)',             5, 100,  1,
     'A fix less accurate than this does not count for a link; also the most two accuracies may cover'),
    ('WalkLinkFixAgeSec',     20,  true,  'walk', 'Fix age allowed (seconds)',                5, 120,  1,
     'A fix older than this does not count, and two members'' fixes must be this close in time'),
    ('WalkLinkMoveM',         50,  true,  'walk', 'Must move (metres)',                       0, 500,  5,
     'The bonus pauses unless the player moved this far within the window below (0: never pauses)'),
    ('WalkLinkMoveWindowSec', 180, true,  'walk', 'Within (seconds)',                         30, 1800, 10,
     'The window for the distance above'),
    ('WalkLinkDailyMinutes',  120, true,  'walk', 'Bonus minutes a day',                      0, 1440, 5,
     'Bonus-active time per player per day; after it the link stays for company without the bonus'),
    ('WalkLinkAskSec',        120, true,  'walk', 'Ask expires after (seconds)',              30, 900, 10,
     'How long a WALK TOGETHER ask waits'),
    ('WalkLinkOpenSec',       60,  true,  'walk', 'Panel open counts for (seconds)',          15, 600, 5,
     'A friend shows in the LINK list this long after their panel last reported'),
    ('WalkLinkScanPct2',      25,  true,  'walk', 'Scan range bonus, 2 linked (%)',           0, 200,  5,
     'Added to the scan range (and any faction perk) for a link of 2'),
    ('WalkLinkScanPct3',      40,  true,  'walk', 'Scan range bonus, 3 linked (%)',           0, 200,  5, 'For a link of 3'),
    ('WalkLinkScanPct4',      50,  true,  'walk', 'Scan range bonus, 4 linked (%)',           0, 200,  5, 'For a link of 4'),
    ('WalkLinkYieldPct2',     15,  true,  'walk', 'Harvest and drop bonus, 2 linked (%)',     0, 200,  5,
     'Added to node harvests and map drop pickups for a link of 2 (not Tether Node collects)'),
    ('WalkLinkYieldPct3',     25,  true,  'walk', 'Harvest and drop bonus, 3 linked (%)',     0, 200,  5, 'For a link of 3'),
    ('WalkLinkYieldPct4',     30,  true,  'walk', 'Harvest and drop bonus, 4 linked (%)',     0, 200,  5, 'For a link of 4'),
    ('WalkLinkXpPct2',        10,  true,  'walk', 'Harvest XP bonus, 2 linked (%)',           0, 200,  5,
     'Added to harvest XP for a link of 2'),
    ('WalkLinkXpPct3',        20,  true,  'walk', 'Harvest XP bonus, 3 linked (%)',           0, 200,  5, 'For a link of 3'),
    ('WalkLinkXpPct4',        30,  true,  'walk', 'Harvest XP bonus, 4 linked (%)',           0, 200,  5, 'For a link of 4'),
    ('WalkLinkKeepDays',      90,  false, 'walk', 'Link history kept (days)',                 7, 365,  1,
     'Who linked with whom, when and how far, for moderation; then deleted')
ON CONFLICT (key) DO NOTHING;
