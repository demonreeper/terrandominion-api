-- 0065 faction ranks v2 and Nomads, R1 (docs/faction-ranks-plan.md, owner 2026-11-12): eight ranks as data
-- (internal/game/ranks.go), Prospect, Captain squads, the Assistant Global Head seat (empty), Nomads.
--
-- Rank levels: Prospect 5, Member 10, Captain 30, Warden 50, Atoll/City Lead 70, Regional Commander 90, Assistant
-- Global Head 95, Global Head 100 (the faction's power). Existing members become Member; appointed ranks stay.
UPDATE td.faction_members SET rank = 10 WHERE rank < 10;

-- Contribution points follow the player (a Nomad keeps them when moving); B2 earns them by giving.
ALTER TABLE td.players ADD COLUMN contribution_points bigint NOT NULL DEFAULT 0 CHECK (contribution_points >= 0);

-- Nomads: nomad_cooldown_from is the last move, or when they became a Nomad again (the 30 days start over).
ALTER TABLE td.faction_members
    ADD COLUMN nomad               boolean NOT NULL DEFAULT false,
    ADD COLUMN nomad_cooldown_from timestamptz,
    ADD COLUMN was_nomad           boolean NOT NULL DEFAULT false;

-- A Captain's squad: up to CaptainSquadSize Members and Prospects of their region; a player is in one squad at most.
CREATE TABLE td.squad_members (
    player_id  bigint PRIMARY KEY REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    captain_id bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    joined_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (player_id <> captain_id)
);
CREATE INDEX squad_members_captain ON td.squad_members (captain_id);

-- A Nomad's ask to move home regions. The receiving region's Commander, a Warden there, or the Global Head answers.
CREATE TABLE td.nomad_asks (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id   bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE ON UPDATE CASCADE,
    faction_id  smallint NOT NULL REFERENCES td.factions (id),
    from_region char(2) NOT NULL,
    to_region   char(2) NOT NULL,
    status      text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'approved', 'declined', 'expired', 'cancelled')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    decided_by  bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE,
    decided_at  timestamptz
);
CREATE UNIQUE INDEX nomad_asks_one_open ON td.nomad_asks (player_id) WHERE status = 'open';
CREATE INDEX nomad_asks_to ON td.nomad_asks (faction_id, to_region) WHERE status = 'open';

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('ProspectDays',          7,    true,  'ranks', 'Prospect becomes Member after (days)', 0, 90, 1,
     'Every new pledge starts as Prospect; Member after this many days or the points below, whichever first'),
    ('ProspectPoints',        100,  true,  'ranks', 'Prospect becomes Member at (points)', 0, 100000, 10,
     'Contribution points that end the trial early (0: days only)'),
    ('CaptainSquadSize',      10,   true,  'ranks', 'Captain squad size', 1, 50, 1, 'Members and Prospects in one squad'),
    ('CaptainVaultDailyCap',  500,  true,  'ranks', 'Captain vault limit a day', 0, 100000, 10,
     'Resources a Captain may give from a base vault on site per day, all kinds together'),
    ('RankPointsCaptain',     1000, true,  'ranks', 'Captain at (points)', 0, 1000000, 50,
     'Contribution points that promote a Member to Captain when a slot is free (0: never by points)'),
    ('RankPointsWarden',      2000, true,  'ranks', 'Warden at (points)', 0, 1000000, 50,
     'Contribution points that promote a Captain to Warden when a slot is free (0: never by points)'),
    ('RankPointsLead',        5000, true,  'ranks', 'Atoll/City Lead at (points)', 0, 1000000, 50,
     'Contribution points that promote a Warden to Lead when a slot is free (0: never by points)'),
    ('RankSlotsCaptain',      12,   true,  'ranks', 'Captains per region', 0, 500, 1,
     'Most Captains a faction may have in a region; when full, promotions queue by points'),
    ('RankSlotsWarden',       6,    true,  'ranks', 'Wardens per region', 0, 500, 1, 'Most Wardens per faction per region'),
    ('RankSlotsLead',         3,    true,  'ranks', 'Atoll/City Leads per region', 0, 100, 1, 'Most Leads per faction per region'),
    ('NomadMoveCooldownDays', 30,   true,  'ranks', 'Days between Nomad moves', 0, 365, 1,
     'Also counted from becoming a Nomad again'),
    ('NomadAskHours',         48,   true,  'ranks', 'Nomad ask lasts (hours)', 1, 336, 1, 'An unanswered ask to move expires')
ON CONFLICT (key) DO NOTHING;
