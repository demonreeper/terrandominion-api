-- 0001 core: players, identities, sessions, devices, settings, XP and admin audit.
--
-- Everything lives in its own schema (td) so this database can share a Postgres server with
-- other games, and with Nakama's own tables if the server ever moves onto Nakama.

CREATE EXTENSION IF NOT EXISTS postgis;
CREATE SCHEMA IF NOT EXISTS td;

CREATE TYPE td.account_type AS ENUM ('player', 'founder', 'bot', 'system');
CREATE TYPE td.account_status AS ENUM ('active', 'banned', 'pending_delete', 'deleted');

-- One row per person. The internal id is the key everywhere; sign-in providers are only linked
-- identities, so a provider's user id never leaks into names, files or other players' views.
CREATE TABLE td.players (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    display_name     text NOT NULL,
    type             td.account_type NOT NULL DEFAULT 'player',
    status           td.account_status NOT NULL DEFAULT 'active',
    is_admin         boolean NOT NULL DEFAULT false,
    is_mod           boolean NOT NULL DEFAULT false,
    level            integer NOT NULL DEFAULT 1 CHECK (level >= 1),
    xp               bigint NOT NULL DEFAULT 0 CHECK (xp >= 0),
    country_code     char(2) NOT NULL DEFAULT 'MV',
    photo_file       text,                 -- file name only; the API builds the full path
    last_app_version text,
    last_seen_at     timestamptz,
    delete_after     timestamptz,          -- self-deletion grace period; signing in cancels it
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    nakama_user_id   uuid UNIQUE           -- reserved for a future move onto Nakama
);

CREATE INDEX players_level_idx ON td.players (level DESC, xp DESC) WHERE status = 'active';
CREATE INDEX players_delete_after_idx ON td.players (delete_after) WHERE delete_after IS NOT NULL;

CREATE TABLE td.player_identities (
    provider   text NOT NULL,              -- 'google' now; 'apple' and 'nakama' later
    subject    text NOT NULL,              -- the provider's stable user id
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    email      text,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);

CREATE INDEX player_identities_player_idx ON td.player_identities (player_id);

-- One row per signed-in device: several phones can stay signed in at once. Only a hash of the
-- refresh token is stored, and it rotates on every refresh.
CREATE TABLE td.sessions (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id    bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    refresh_hash bytea NOT NULL UNIQUE,
    device_hash  bytea NOT NULL,
    platform     text NOT NULL,
    app_version  text,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz
);

CREATE INDEX sessions_player_live_idx ON td.sessions (player_id) WHERE revoked_at IS NULL;

-- Anti-farming: the welcome grant is given once per phone, however many accounts sign in on it.
CREATE TABLE td.devices (
    device_hash     bytea PRIMARY KEY,
    first_player_id bigint REFERENCES td.players (id) ON DELETE SET NULL,
    accounts        integer NOT NULL DEFAULT 0,
    welcome_granted boolean NOT NULL DEFAULT false,
    first_seen_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at    timestamptz NOT NULL DEFAULT now()
);

-- Every tunable number in one table, editable from the admin API without a deploy.
CREATE TABLE td.settings (
    key         text PRIMARY KEY,
    value       numeric NOT NULL,
    description text NOT NULL,
    is_public   boolean NOT NULL DEFAULT false,   -- also sent to clients
    updated_at  timestamptz NOT NULL DEFAULT now(),
    updated_by  bigint REFERENCES td.players (id) ON DELETE SET NULL
);

-- XP comes from every action; the ledger makes balancing and abuse checks possible later.
CREATE TABLE td.xp_events (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id  bigint NOT NULL REFERENCES td.players (id) ON DELETE CASCADE,
    action     text NOT NULL,
    xp         integer NOT NULL,
    ref_id     bigint,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX xp_events_player_idx ON td.xp_events (player_id, created_at DESC);

CREATE TABLE td.admin_actions (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    admin_id   bigint REFERENCES td.players (id) ON DELETE SET NULL,
    action     text NOT NULL,
    target     text,
    detail     jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
