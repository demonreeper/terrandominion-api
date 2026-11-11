-- 0060 walk together, W2 (docs/walk-together-plan.md, "Bluetooth proof", L3, L8): every linked phone advertises a
-- rotating server token over Bluetooth LE and listens for the other members'. The bonus needs a member to be
-- Bluetooth-confirmed (heard, or hearing). Twin fixes (L3) stop the group's bonus and make a Signal watch report;
-- GPS together with Bluetooth on but nobody ever heard (L8) makes a report too.

ALTER TABLE td.walk_link_members
    ADD COLUMN ble_on           boolean NOT NULL DEFAULT false,
    -- The token this member advertises now and the one before it (tokens rotate every WalkLinkBleTokenSec;
    -- random, never tied to the device over the air).
    ADD COLUMN ble_token        text,
    ADD COLUMN ble_token_prev   text,
    ADD COLUMN ble_token_at     timestamptz,
    -- The tokens this member's phone heard in its last report, and when.
    ADD COLUMN ble_heard        text NOT NULL DEFAULT '',
    ADD COLUMN ble_heard_at     timestamptz,
    ADD COLUMN ble_confirmed_at timestamptz,
    -- L3: how many beats in a row this member's fix matched another member's exactly.
    ADD COLUMN twin_count       int NOT NULL DEFAULT 0,
    -- L8: since when GPS has said "together" with Bluetooth on but nobody heard, and whether it was reported.
    ADD COLUMN together_since   timestamptz,
    ADD COLUMN ble_reported     boolean NOT NULL DEFAULT false;

-- L3: a link with twin fixes gets no bonus for the rest of its life.
ALTER TABLE td.walk_links ADD COLUMN twin_flagged boolean NOT NULL DEFAULT false;

INSERT INTO td.settings (key, value, is_public, grp, label, min_value, max_value, step, description) VALUES
    ('WalkLinkRequireBle',     1,   true,  'walk', 'Bonus needs Bluetooth',                  0, 1,    1,
     '1: a member gets the bonus only while Bluetooth-confirmed with the group. 0: GPS alone'),
    ('WalkLinkBleGraceSec',    60,  true,  'walk', 'Bluetooth may drop for (seconds)',       10, 600, 5,
     'Without a Bluetooth confirmation this long the bonus pauses (CONFIRMING...); the link stays'),
    ('WalkLinkBleTokenSec',    60,  true,  'walk', 'Bluetooth token changes every (seconds)', 15, 600, 5,
     'How often each linked phone gets a new token to advertise'),
    ('WalkLinkBleHeardSec',    30,  true,  'walk', 'Heard tokens count for (seconds)',       5, 120,  1,
     'A phone reports the tokens it heard in this window'),
    ('WalkLinkTwinFixes',      3,   false, 'walk', 'Twin fixes in a row (L3)',               2, 20,   1,
     'Two members sending the same position and accuracy this many beats running stops the bonus and reports'),
    ('WalkLinkTwinM',          1,   false, 'walk', 'Twin fixes within (metres)',             0, 10,   0.5,
     'How close two fixes must be to count as the same'),
    ('WalkLinkNoBleReportSec', 300, false, 'walk', 'No Bluetooth contact report after (seconds)', 60, 3600, 30,
     'GPS says together, Bluetooth on, nobody heard for this long: Signal watch report (L8)')
ON CONFLICT (key) DO NOTHING;
