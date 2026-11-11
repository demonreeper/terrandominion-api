-- 0036 profiles (docs/profile-plan.md, P0): a share code and a player-chosen callsign.
--
-- Other players never see display_name (the Google or Apple account's real name): it stays for
-- support and admin use. Every player-facing name comes from td.player_name().

-- An 8-character share code in Crockford base32 (no I, L, O, U), random, never the counting id.
-- gen_random_uuid() is the randomness; the loop keeps it unique.
CREATE FUNCTION td.new_public_code() RETURNS text
LANGUAGE plpgsql VOLATILE AS $$
DECLARE
    alphabet constant text := '0123456789ABCDEFGHJKMNPQRSTVWXYZ';
    bytes bytea;
    code text;
BEGIN
    LOOP
        bytes := uuid_send(gen_random_uuid());
        code := '';
        FOR i IN 0..7 LOOP
            code := code || substr(alphabet, 1 + (get_byte(bytes, i) % 32), 1);
        END LOOP;
        EXIT WHEN NOT EXISTS (SELECT 1 FROM td.players WHERE public_code = code);
    END LOOP;
    RETURN code;
END $$;

ALTER TABLE td.players
    ADD COLUMN public_code         text,
    ADD COLUMN callsign            text,
    ADD COLUMN callsign_changed_at timestamptz;

-- One row at a time, so each new code is checked against the ones just written.
DO $$
DECLARE r record;
BEGIN
    FOR r IN SELECT id FROM td.players ORDER BY id LOOP
        UPDATE td.players SET public_code = td.new_public_code() WHERE id = r.id;
    END LOOP;
END $$;

ALTER TABLE td.players
    ALTER COLUMN public_code SET NOT NULL,
    ALTER COLUMN public_code SET DEFAULT td.new_public_code(),
    ADD CONSTRAINT players_public_code_key UNIQUE (public_code),
    ADD CONSTRAINT players_callsign_format CHECK (callsign IS NULL OR callsign ~ '^[A-Za-z0-9_-]{3,16}$');

CREATE UNIQUE INDEX players_callsign_key ON td.players (lower(callsign)) WHERE callsign IS NOT NULL;

-- The name other players see. A deleted or banned account reads "Deleted player" everywhere.
CREATE FUNCTION td.player_name(callsign text, public_code text, status td.account_status) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE WHEN status IN ('deleted', 'banned') THEN 'Deleted player'
                ELSE COALESCE(callsign, 'OPERATIVE-' || public_code) END
$$;

-- The share code other players may open a profile with: none for a deleted or banned account.
CREATE FUNCTION td.player_code(public_code text, status td.account_status) RETURNS text
LANGUAGE sql IMMUTABLE AS $$
    SELECT CASE WHEN status IN ('deleted', 'banned') THEN '' ELSE public_code END
$$;

-- Words a callsign may not contain (matched with letters and digits only, lower case). The
-- reserved names (staff words, the powers, the factions) are in the code: game.ReservedCallsigns.
CREATE TABLE td.callsign_blocked (
    word       text PRIMARY KEY CHECK (word = lower(word) AND length(word) >= 3),
    created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO td.callsign_blocked (word) VALUES
    ('fuck'), ('shit'), ('bitch'), ('cunt'), ('nigger'), ('nigga'), ('faggot'), ('whore'), ('slut'),
    ('nazi'), ('hitler'), ('penis'), ('vagina'), ('pussy'), ('asshole');

INSERT INTO td.settings (key, value, description, is_public) VALUES
    ('CallsignChangeDays', 30, 'Days a player must wait between callsign changes (the first pick is free)', true)
ON CONFLICT (key) DO NOTHING;
