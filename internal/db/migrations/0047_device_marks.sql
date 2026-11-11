-- 0047 device record (docs/freelancer-path-plan.md phase 2, docs/profile-plan.md "Device record"):
-- a pledge and a ban are bound to the phone and the Google/Apple sign-in, so a new account can't undo
-- them. Only salted one-way hashes are kept (HMAC with the server's device pepper): the device id
-- (Android ANDROID_ID, iOS an id kept in the Keychain) and the sign-in subject. A mark is never tied
-- to a player row, so it outlives account deletion on purpose.
--
-- One row per hash (kind 'device' or 'signin'), so a ban can mark every phone and sign-in a player
-- used. faction_code: the faction pledged on it. banned / ban_sanction_id: the ban that marked it
-- (no foreign key: sanctions go with a deleted account, the mark must not). appeal_code is the short
-- code a player sends with an appeal; it is shown, the hashes never are.
CREATE TABLE td.device_marks (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind            text NOT NULL CHECK (kind IN ('device', 'signin')),
    hash            bytea NOT NULL,
    faction_code    text REFERENCES td.factions (code),
    banned          boolean NOT NULL DEFAULT false,
    ban_sanction_id bigint,
    appeal_code     text NOT NULL UNIQUE,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (kind, hash)
);

-- The faction a new account on a marked device or sign-in returns to at the faction level.
ALTER TABLE td.players ADD COLUMN bound_faction_code text REFERENCES td.factions (code);

-- Players who pledged before this: mark the phones and sign-ins they use now. The hashes come from
-- the server, so this is done the first time each of them signs in or refreshes (service.markMember).
