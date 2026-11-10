-- 0029 Orders and daily missions (docs/regional-command-plan.md, milestones 2-4).
--
-- Each faction's lore power speaks under its own name, a word for "signal" in a different language.
-- Players must not know the six are one AI, so nothing here or in the API joins them.

ALTER TABLE td.factions
    ADD COLUMN persona        text NOT NULL DEFAULT '',
    ADD COLUMN persona_invite text NOT NULL DEFAULT '';   -- the invitation at level 20

-- Placeholder factions by colour: 1 Lumen, 2 Forge, 3 Verdant, 4 Veil, 5 Slipstream, 6 Bastion.
UPDATE td.factions SET persona = 'Signum',
    persona_invite = 'You have walked with me long enough to see the light in what you carry. Lumen is waking. Come.' WHERE id = 1;
UPDATE td.factions SET persona = 'Shingō',
    persona_invite = 'You build, and what you build stands. Forge needs hands like yours. Come and command.' WHERE id = 2;
UPDATE td.factions SET persona = 'Ishaaraath',
    persona_invite = 'You know what things are worth. The Exchange is open to you. Come.' WHERE id = 3;
UPDATE td.factions SET persona = 'Sēma',
    persona_invite = 'You looked where others did not. The Veil has been watching you. Come closer.' WHERE id = 4;
UPDATE td.factions SET persona = 'Tohu',
    persona_invite = 'You moved before the others woke. The Vanguard rides at dawn. Ride with us.' WHERE id = 5;
UPDATE td.factions SET persona = 'Merki',
    persona_invite = 'You held what was yours. The Bastion needs those who hold. Stand with us.' WHERE id = 6;

-- The pool every day's missions are drawn from. audience: freelancer (the guide to level 20, all
-- six personas), member (from the player's own faction), command (Regional Commanders and Wardens).
-- goal_key narrows a goal: a resource for harvest, a recipe for craft.
CREATE TABLE td.mission_types (
    id         text PRIMARY KEY,
    faction_id smallint NOT NULL REFERENCES td.factions (id),
    audience   text NOT NULL CHECK (audience IN ('freelancer', 'member', 'command')),
    goal       text NOT NULL,
    goal_key   text,
    count      integer NOT NULL CHECK (count > 0),
    min_level  integer NOT NULL DEFAULT 1,
    max_level  integer NOT NULL DEFAULT 1000,
    weight     integer NOT NULL DEFAULT 1 CHECK (weight > 0),
    line       text NOT NULL
);

INSERT INTO td.mission_types (id, faction_id, audience, goal, goal_key, count, min_level, max_level, line) VALUES
    -- Signum (Lumen): patience and crystal.
    ('sig_f1', 1, 'freelancer', 'harvest', 'crystal', 3, 1, 9, 'Crystal holds the light of every hand that touched it. Bring me three shards.'),
    ('sig_f2', 1, 'freelancer', 'harvest', 'crystal', 10, 5, 19, 'Patience is a harvest too. Ten shards of crystal, gathered without hurry.'),
    ('sig_f3', 1, 'freelancer', 'collect', NULL, 2, 3, 19, 'What your Tether Node keeps is yours only when you claim it. Collect twice.'),
    ('sig_f4', 1, 'freelancer', 'harvest', NULL, 8, 10, 19, 'Walk the seams of the city. Eight harvests, any kind.'),
    -- Shingō (Forge): building.
    ('shi_f1', 2, 'freelancer', 'build_tether', NULL, 1, 1, 19, 'A thing built stands longer than a thing taken. Raise a Tether Node.'),
    ('shi_f2', 2, 'freelancer', 'craft', NULL, 2, 3, 19, 'Your hands should know the forge. Craft two items.'),
    ('shi_f3', 2, 'freelancer', 'upgrade_gear', NULL, 1, 5, 19, 'Gear that never improves is gear that fails. Upgrade one piece.'),
    ('shi_f4', 2, 'freelancer', 'upgrade_tether', NULL, 1, 8, 19, 'Strengthen what you have built. Upgrade a Tether Node.'),
    -- Ishaaraath (Verdant): value and trade.
    ('ish_f1', 3, 'freelancer', 'upload', NULL, 1, 1, 19, 'Value carried is value at risk. Upload at a Tether Node.'),
    ('ish_f2', 3, 'freelancer', 'harvest', 'data', 3, 1, 9, 'Information is the oldest currency. Bring me three data.'),
    ('ish_f3', 3, 'freelancer', 'harvest', 'credit', 3, 5, 19, 'Credit moves those who hold it. Gather three.'),
    ('ish_f4', 3, 'freelancer', 'harvest', 'data', 8, 10, 19, 'The market listens to those who know. Eight data.'),
    -- Sēma (Veil): looking closely.
    ('sem_f1', 4, 'freelancer', 'harvest', NULL, 4, 1, 9, 'Most never look closely at the ground they walk. Harvest four times, and look.'),
    ('sem_f2', 4, 'freelancer', 'harvest', 'data', 5, 5, 19, 'Hidden things leave traces. Five data, quietly.'),
    ('sem_f3', 4, 'freelancer', 'craft', 'signal_jammer', 1, 5, 19, 'A silent node is a blind node. Craft a Signal Jammer.'),
    ('sem_f4', 4, 'freelancer', 'harvest', NULL, 12, 10, 19, 'See more than they do. Twelve harvests.'),
    -- Tohu (Slipstream): speed.
    ('toh_f1', 5, 'freelancer', 'harvest', NULL, 6, 1, 9, 'The first to the node wins the dawn. Six harvests. Move.'),
    ('toh_f2', 5, 'freelancer', 'harvest', 'alloy', 6, 3, 19, 'Speed needs metal. Six alloy.'),
    ('toh_f3', 5, 'freelancer', 'drone', NULL, 1, 5, 19, 'Let your drone fly where your feet cannot. Launch one.'),
    ('toh_f4', 5, 'freelancer', 'harvest', NULL, 15, 10, 19, 'Cover ground. Fifteen harvests before the day is out.'),
    -- Merki (Bastion): holding.
    ('mer_f1', 6, 'freelancer', 'craft', 'energy_cell', 2, 1, 19, 'A defender without power is a statue. Craft two energy cells.'),
    ('mer_f2', 6, 'freelancer', 'build_tether', NULL, 1, 1, 9, 'Hold a place of your own. Raise a Tether Node.'),
    ('mer_f3', 6, 'freelancer', 'fit_shield', NULL, 1, 5, 19, 'Shield what you hold. Fit a shield to your Tether Node.'),
    ('mer_f4', 6, 'freelancer', 'collect', NULL, 3, 10, 19, 'Guard your stores. Collect from your Tether Nodes three times.'),

    ('sig_m1', 1, 'member', 'harvest', 'crystal', 20, 20, 1000, 'Lumen grows by what we gather. Twenty crystal today.'),
    ('sig_m2', 1, 'member', 'collect', NULL, 3, 20, 1000, 'Bring home what your Tether Nodes hold. Collect three times.'),
    ('sig_m3', 1, 'member', 'harvest', NULL, 15, 20, 1000, 'Fifteen harvests. The light is patient; you need not be.'),
    ('sig_m4', 1, 'member', 'upload', NULL, 2, 20, 1000, 'Store what you carry. Upload twice.'),
    ('shi_m1', 2, 'member', 'upgrade_tether', NULL, 1, 20, 1000, 'Forge does not stand still. Upgrade a Tether Node.'),
    ('shi_m2', 2, 'member', 'build_tether', NULL, 1, 20, 1000, 'Claim ground with something built. Raise a Tether Node.'),
    ('shi_m3', 2, 'member', 'craft', NULL, 4, 20, 1000, 'Four things made today. The forge stays hot.'),
    ('shi_m4', 2, 'member', 'harvest', 'alloy', 15, 20, 1000, 'Fifteen alloy. Forge builds with what you bring.'),
    ('ish_m1', 3, 'member', 'upload', NULL, 3, 20, 1000, 'Three uploads. The Exchange keeps its accounts in order.'),
    ('ish_m2', 3, 'member', 'harvest', 'credit', 6, 20, 1000, 'Six credit. Wealth is a signal others follow.'),
    ('ish_m3', 3, 'member', 'harvest', 'data', 12, 20, 1000, 'Twelve data. Know the market before it knows you.'),
    ('ish_m4', 3, 'member', 'collect', NULL, 3, 20, 1000, 'Collect three times. Idle value is lost value.'),
    ('sem_m1', 4, 'member', 'harvest', 'data', 10, 20, 1000, 'Ten data, gathered where no one is looking.'),
    ('sem_m2', 4, 'member', 'craft', 'signal_jammer', 2, 20, 1000, 'Two Signal Jammers. Blind them before they see us.'),
    ('sem_m3', 4, 'member', 'attack', NULL, 1, 20, 1000, 'Strike one Tether Node that is not ours. Leave no name.'),
    ('sem_m4', 4, 'member', 'harvest', NULL, 15, 20, 1000, 'Fifteen harvests. Every one tells us something.'),
    ('toh_m1', 5, 'member', 'drone', NULL, 2, 20, 1000, 'Two drones in the air today. The sky is ours first.'),
    ('toh_m2', 5, 'member', 'harvest', NULL, 20, 20, 1000, 'Twenty harvests. Faster than yesterday.'),
    ('toh_m3', 5, 'member', 'attack', NULL, 2, 20, 1000, 'Hit two Tether Nodes before their owners wake.'),
    ('toh_m4', 5, 'member', 'harvest', 'alloy', 12, 20, 1000, 'Twelve alloy. Speed is built from metal.'),
    ('mer_m1', 6, 'member', 'fit_shield', NULL, 1, 20, 1000, 'Shield a Tether Node. The Bastion does not fall.'),
    ('mer_m2', 6, 'member', 'craft', 'energy_cell', 4, 20, 1000, 'Four energy cells. A wall needs power behind it.'),
    ('mer_m3', 6, 'member', 'collect', NULL, 3, 20, 1000, 'Collect three times. Hold what is ours.'),
    ('mer_m4', 6, 'member', 'build_tether', NULL, 1, 20, 1000, 'Raise a Tether Node and keep it standing.'),

    ('sig_c1', 1, 'command', 'appoint_warden', NULL, 1, 20, 1000, 'Every light needs keepers. Name a Warden among your members.'),
    ('sig_c2', 1, 'command', 'upgrade_tether', NULL, 1, 20, 1000, 'Lead by example. Upgrade a Tether Node of your own.'),
    ('shi_c1', 2, 'command', 'appoint_warden', NULL, 1, 20, 1000, 'A builder cannot raise everything alone. Appoint a Warden.'),
    ('shi_c2', 2, 'command', 'build_tether', NULL, 2, 20, 1000, 'Two Tether Nodes raised today. Show them how Forge builds.'),
    ('ish_c1', 3, 'command', 'appoint_warden', NULL, 1, 20, 1000, 'Trust is the rarest currency. Spend it: appoint a Warden.'),
    ('ish_c2', 3, 'command', 'upload', NULL, 4, 20, 1000, 'Four uploads. A commander''s stores are the branch''s stores.'),
    ('sem_c1', 4, 'command', 'appoint_warden', NULL, 1, 20, 1000, 'Choose who else may see what we see. Appoint a Warden.'),
    ('sem_c2', 4, 'command', 'attack', NULL, 2, 20, 1000, 'Two strikes on Tether Nodes not ours. Quietly.'),
    ('toh_c1', 5, 'command', 'appoint_warden', NULL, 1, 20, 1000, 'Pick your fastest. Appoint a Warden.'),
    ('toh_c2', 5, 'command', 'drone', NULL, 3, 20, 1000, 'Three drones launched today. Lead from the front.'),
    ('mer_c1', 6, 'command', 'appoint_warden', NULL, 1, 20, 1000, 'A wall needs more than one stone. Appoint a Warden.'),
    ('mer_c2', 6, 'command', 'fit_shield', NULL, 1, 20, 1000, 'Shield a Tether Node. Show them how the Bastion holds.');

-- One row per mission a player was given on a day (the Maldives date). The reward is fixed when it
-- is given, so the player sees it in advance.
CREATE TABLE td.player_missions (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id       bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    mission_type_id text NOT NULL REFERENCES td.mission_types (id),
    faction_id      smallint NOT NULL REFERENCES td.factions (id),
    day             date NOT NULL,
    audience        text NOT NULL,
    goal            text NOT NULL,
    goal_key        text,
    count           integer NOT NULL,
    progress        integer NOT NULL DEFAULT 0,
    status          text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'done', 'claimed')),
    reward          jsonb NOT NULL,
    line            text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    done_at         timestamptz,
    claimed_at      timestamptz,
    UNIQUE (player_id, day, mission_type_id)
);
CREATE INDEX player_missions_day_idx ON td.player_missions (player_id, day);

-- The lore power that invites a freelancer at level 20: the one whose missions they finished most.
CREATE TABLE td.player_invitations (
    player_id  bigint PRIMARY KEY REFERENCES td.players (id) ON DELETE CASCADE,
    faction_id smallint NOT NULL REFERENCES td.factions (id),
    created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO td.settings (key, value, is_public, description) VALUES
    ('MissionsPerDay',          4,    true,  'Daily missions each player gets'),
    ('MissionDayOffsetMinutes', 300,  true,  'Missions refresh at midnight at this offset from UTC (300 = Maldives time)'),
    ('MissionXpBase',           30,   true,  'XP for a mission at level 0; more per level'),
    ('MissionXpPerLevel',       5,    true,  'Extra mission XP per player level'),
    ('MissionCreditBase',       5,    true,  'Credits for a mission at level 0; more per level'),
    ('MissionCreditPerLevel',   1,    true,  'Extra mission credits per player level'),
    ('MissionItemChance',       0.25, true,  'Chance a mission also gives a low-level item'),
    ('MissionCommandMult',      2,    true,  'Reward multiplier for command orders (Regional Commanders and Wardens)');
