-- 0070 Captain squad missions (docs/faction-ranks-plan.md, Captain and Squads; docs/base-building-plan.md 10).
--
-- A Captain posts one of the base mission kinds (give at a base, stand at it, repair a module there) to their squad,
-- naming one of the faction's standing bases in their region. Each squad member then has it as an extra mission for the
-- day, in their own power's words, counted only at that base. At most CaptainMissionsPerDay posts a day.

-- One row per post. The squad members it went to each have a player_missions row pointing here; a member who leaves
-- the squad keeps theirs, and one who joins later doesn't get it. Posts are never deleted (the missions point at them).
CREATE TABLE td.captain_missions (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    captain_id bigint REFERENCES td.players (id) ON DELETE SET NULL ON UPDATE CASCADE,
    faction_id smallint NOT NULL REFERENCES td.factions (id),
    region     char(2) NOT NULL,
    base_id    bigint NOT NULL,        -- the named base (no foreign key: a base that is gone simply never counts)
    day        date NOT NULL,          -- the mission day it was posted on
    goal       text NOT NULL CHECK (goal IN ('base_give', 'base_stand', 'base_repair')),
    goal_key   text,                   -- the resource of a give
    count      integer NOT NULL CHECK (count > 0),
    members    integer NOT NULL DEFAULT 0,   -- squad members it went to
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX captain_missions_captain_day ON td.captain_missions (captain_id, day);

-- The words each power says a Captain's mission in. Never drawn as ordinary or base missions.
ALTER TABLE td.mission_types ADD COLUMN captain_mission boolean NOT NULL DEFAULT false;
INSERT INTO td.mission_types (id, faction_id, audience, goal, goal_key, count, min_level, max_level, line, captain_mission) VALUES
    ('cap_give_1',   1, 'member', 'base_give',   NULL, 1, 1, 1000, 'Your Captain asks it of you. Carry it to the stake, and let it be counted.', true),
    ('cap_stand_1',  1, 'member', 'base_stand',  NULL, 1, 1, 1000, 'Your Captain wants you at the stake. Go, and be patient there.', true),
    ('cap_repair_1', 1, 'member', 'base_repair', NULL, 1, 1, 1000, 'Your Captain has seen the damage. Mend it gently.', true),
    ('cap_give_2',   2, 'member', 'base_give',   NULL, 1, 1, 1000, 'The Captain has asked. Bring the material. Build.', true),
    ('cap_stand_2',  2, 'member', 'base_stand',  NULL, 1, 1, 1000, 'The Captain needs hands on site. Be there.', true),
    ('cap_repair_2', 2, 'member', 'base_repair', NULL, 1, 1, 1000, 'The Captain found a broken module. Make it stand again.', true),
    ('cap_give_3',   3, 'member', 'base_give',   NULL, 1, 1, 1000, 'Your Captain has set the price. Pay it into the base; it pays back.', true),
    ('cap_stand_3',  3, 'member', 'base_stand',  NULL, 1, 1, 1000, 'Your Captain wants you seen at the base. Presence is worth something.', true),
    ('cap_repair_3', 3, 'member', 'base_repair', NULL, 1, 1, 1000, 'Your Captain counted the damage. Mending costs less than losing.', true),
    ('cap_give_4',   4, 'member', 'base_give',   NULL, 1, 1, 1000, 'A quiet word from your Captain. Leave it at the base, and say nothing.', true),
    ('cap_stand_4',  4, 'member', 'base_stand',  NULL, 1, 1, 1000, 'A quiet word from your Captain: eyes at the base. Wait there. Watch.', true),
    ('cap_repair_4', 4, 'member', 'base_repair', NULL, 1, 1, 1000, 'A quiet word from your Captain: there is an opening. Close it before others do.', true),
    ('cap_give_5',   5, 'member', 'base_give',   NULL, 1, 1, 1000, 'Captain''s call. Get it to the base. Fast.', true),
    ('cap_stand_5',  5, 'member', 'base_stand',  NULL, 1, 1, 1000, 'Captain''s call. Hold the base, then move on.', true),
    ('cap_repair_5', 5, 'member', 'base_repair', NULL, 1, 1, 1000, 'Captain''s call. A module is down. Fix it and go.', true),
    ('cap_give_6',   6, 'member', 'base_give',   NULL, 1, 1, 1000, 'The one who leads your squad holds this line. Give to the base that holds it with you.', true),
    ('cap_stand_6',  6, 'member', 'base_stand',  NULL, 1, 1, 1000, 'The one who leads your squad calls you to the base. Stand there. Hold.', true),
    ('cap_repair_6', 6, 'member', 'base_repair', NULL, 1, 1, 1000, 'The one who leads your squad wants the wall whole. Repair the module.', true)
ON CONFLICT (id) DO NOTHING;

-- A player's Captain missions: which post it came from and the base it counts at (base_id stays NULL on every other
-- mission, which counts at any of the faction's bases as before).
ALTER TABLE td.player_missions
    ADD COLUMN captain_mission_id bigint REFERENCES td.captain_missions (id),
    ADD COLUMN base_id            bigint;

-- One mission of a type a day still holds for the drawn missions; a Captain's posts may share a type (two give missions
-- on one day), so they are kept apart: one row per member per post.
DO $$
DECLARE c text;
BEGIN
    FOR c IN SELECT conname FROM pg_constraint WHERE conrelid = 'td.player_missions'::regclass AND contype = 'u' LOOP
        EXECUTE format('ALTER TABLE td.player_missions DROP CONSTRAINT %I', c);
    END LOOP;
END $$;
CREATE UNIQUE INDEX player_missions_one_type ON td.player_missions (player_id, day, mission_type_id)
    WHERE captain_mission_id IS NULL;
CREATE UNIQUE INDEX player_missions_one_post ON td.player_missions (player_id, captain_mission_id)
    WHERE captain_mission_id IS NOT NULL;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('CaptainMissionsPerDay',     2,   true, 'ranks', 'Squad missions a Captain posts a day', 0, 20, 1,
     'Each goes to every member of the squad as an extra mission; it pays what a base mission pays, no more'),
    ('CaptainMissionGiveMax',     500, true, 'ranks', 'Squad mission: most to give', 1, 100000, 10,
     'The most of one resource a Captain''s give mission may ask'),
    ('CaptainMissionStandMaxMin', 30,  true, 'ranks', 'Squad mission: longest stand (minutes)', 1, 240, 1,
     'The longest a Captain''s stand-at-the-base mission may ask')
ON CONFLICT (key) DO NOTHING;
