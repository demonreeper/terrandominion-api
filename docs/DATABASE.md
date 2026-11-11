# Database

PostgreSQL 16+ with PostGIS. Everything lives in the `td` schema, so the game can share a
Postgres server with other projects (and with Nakama's own tables, if the server ever moves onto
Nakama) without name clashes.

Migrations are in `internal/db/migrations/`, embedded in the binary and applied in order by
`tdapi migrate` (and on `tdapi serve` start-up). **Never edit a migration that has run on live;
add a new numbered file.** Applied files are recorded in `td.schema_migrations`.

## Tables

```
players ─┬─ player_identities   sign-in links (google now; apple, nakama later)
         ├─ sessions            one per signed-in device, refresh token stored as a hash
         ├─ player_resources    what the player carries (losing a fight costs a cut)
         ├─ player_uploaded     stored resources, no carry limit (Tether Node output and uploads)
         ├─ tether_nodes ── tether_links ── nodes   Tether Nodes and the resource node each slot holds
         │    ├─ tether_stock       the held half, waiting for the owner
         │    └─ tether_transfers   every collect and upload with the reported GPS fix
         ├─ map_drops           what an ended Tether Node held, until picked up or expired
         ├─ player_gear ── gear_types        owned gear; the catalogue of gear
         ├─ harvests ── nodes                every harvest with the reported GPS fix
         ├─ xp_events                        XP ledger: every action that earned XP (target_player_id: the other player in a fight, for the daily cap)
         └─ faction_members ── factions ─┬─ faction_branches   regional branch, sub-leader
                                          └─ bases              faction bases (spacing trigger)
devices          anti-farming: welcome grant once per phone (stored as a keyed hash)
settings         every tunable number; edited with the admin API, cached for a minute
resource_types   crystal, alloy, data, credit
osm_land, osm_walkways, osm_blocked   the node seeder's map layers (tdapi import-osm)
seed_cells       every seeding cell looked at: its target, path length, when seeded and topped up
seed_areas       areas an admin pre-seeds (Malé, Hulhumalé, Villimalé)
callsign_blocked words a callsign may not contain
admin_actions    who changed what, and when
```

### players
The internal `id` is the key everywhere. Sign-in providers are linked through
`player_identities`, so a provider's user id never appears in names, file names or other players'
views. `level` is stored for fast leaderboards but always follows `xp` upward (it never drops, so
an admin may grant a level by hand). `delete_after` is the self-deletion grace period: signing in
before it cancels the deletion. `nakama_user_id` is reserved for a future move onto Nakama.

Names (0036, `docs/profile-plan.md`): `display_name` is the Google or Apple account's real name and
is for support and admin use only. Other players see `callsign` (3 to 16 of `A-Z a-z 0-9 - _`, unique
on `lower(callsign)`, null until picked; `callsign_changed_at` drives the 30-day wait) or, until one
is picked, `OPERATIVE-` plus `public_code`: an 8-character random share code in Crockford base32
(default `td.new_public_code()`, unique), which is what share links and the API use instead of the
counting id. Every player-facing query names a player through `td.player_name(callsign, public_code,
status)` and `td.player_code(public_code, status)`: a deleted or banned account reads `Deleted player`
with no code.

Account types (`type`): `player`, `founder`, `bot`, `system`. Status: `active`, `banned`,
`pending_delete`, `deleted`. A deleted account keeps its row (so logs and faction history stay
consistent) but loses its name, photo, sign-in links and sessions.

### sessions
Several devices can be signed in at once. Only a SHA-256 of the refresh token is stored; every
refresh rotates it, and a spent token can never be used again. A ban revokes every session on the
next refresh, so it takes effect within one access-token lifetime (15 minutes by default).

### nodes and harvests
Fixed resource sites with a PostGIS `geography` point and a GiST index. `reserve` (0 to 1) drains
by `HarvestReserveCost` per harvest and regrows at `regen_per_hour` (the `node-regrow` job; safe at
any interval). `source` says where a node came from: `manual` (by hand), `scatter` (admin test
nodes), `seed` (the seeder; `source_ref` is its cell id) or `rare` (rare roaming nodes, which end
at `expires_at` and don't respawn).

**Seeding.** The world is cut into `SeedCellM` (250 m) square cells, id `<metres>:<row>:<col>`: rows
are latitude bands, each cut into columns by the cosine of its centre latitude. A cell's row in
`seed_cells` is locked `FOR UPDATE` while it is seeded, so two scans can't both fill it. Seed nodes
count toward the cell they stand in (a respawn can move one across), scatter nodes don't count. Spots
come from walkable ways in `osm_walkways` moved up to `SeedSnapM`, kept only on `osm_land`, outside
`osm_blocked` (buildings, inland water, runways, taxiways, aprons) and away from other nodes and
Tether Nodes. `scripts/import-osm.ps1` replaces the three OSM tables and clears every cell's
`seeded_at`, so each is looked at again.

Every harvest stores the position and accuracy the phone reported and the computed distance. That
is the anti-spoofing record, and the source of the travel-speed check (`MaxTravelSpeedKmh`).

### gear
`gear_types` is the catalogue: slot (scanner, extractor, rig, drone_bay, shield), tier, minimum
level, stats as JSON (so new stats need no migration), craft cost as JSON, durability and the type
it upgrades to. `player_gear` is what a player owns; a partial unique index allows only one
equipped item per slot. Gear at durability 0 contributes nothing until repaired. New players get a
Mk I scanner and extractor. `player_items` holds the consumables a player has crafted (energy
cells and weapons) as a count per item; their costs are settings (`Craft*`).

### drones and shields
`drone_strikes` holds every strike: the target, the launcher, its raw `damage`, `mk` (the bay's tier), the
launch fix (`from_lat`, `from_lon`; the defender sees the direction), `arrives_at` and `status`
(`flying`, `hit`, `void`). An intercept sets `intercepted` and `intercept_share`; a share of 1 voids it
at once. Landing and intercept both lock the row with a `status = 'flying'` check, so a strike resolves once;
`landed_damage` is what reached the node after intercept and shield. `tether_nodes.shield_tier` (0-4) is the
fitted TN Shield; it lasts as long as the node. Drone Bay tiers are `gear_types` rows `drone_bay_mk1..4`
linked by `upgrades_to`, with `range_m`, `damage`, `drones` and `cells` in `stats`.

### factions
Six placeholder factions are seeded, all `locked` with a lore reason. The first player at
`LeadFactionLevel` to claim one becomes its head: the claim is a conditional update, so of two
players racing, exactly one wins. `leader_player_id` is unique, so nobody heads two factions.
Locked factions cannot be joined. A player is in at most one faction (`faction_members` is keyed
by player); no row means freelancer. Ranks are numbers for now (1 member to 100 head).
Branches belong to a faction and a region (atoll or city) with a sub-leader the head appoints.

### the powers: missions, playstyle, calls and messages
`mission_types` is the pool the day's missions are drawn from and `player_missions` what each player was given
(6 a day for a freelancer, one per power; after joining, only their own faction's power). `player_playstyle` scores
what a player mostly does, per power, inside each action's transaction; its highest row is the one power that calls,
messages and invites them. `player_calls` holds one row per player per kind (`level10`, `level19`,
`level19_repeat`): `due_at` is when the app may see it, `delivered_at` when the app fetched it, `pushed_at` when a
push went out instead, `played_at` when it was heard and so ended. `player_messages` holds the powers' written
messages (`level19_day3` first); `read_at` only clears the unread mark. `player_invitations` is the faction that
invited a freelancer at `LeadFactionLevel`. `factions.persona_invite` is no longer read: the level 20 invitation is
plain game text. Nothing in these tables joins the six powers.

### bases
Faction bases grow in place (`tier`). The spacing rules are a trigger, so no code path can skip
them: one base per faction within `BaseSpacingSameFactionM` (E1402), and none within
`BaseSpacingOtherFactionM` of another faction's base (E1403). An advisory lock serialises base
inserts so two simultaneous builds cannot both pass.

### notices and push
`notices` is what happened to a player. `push_state` makes it an outbox: a pushable kind is written as
`pending` in the action's own transaction, and the sender loop (every 2 s) moves it to `sent`,
`skipped` or `failed`. `tn_id` is the Tether Node concerned, used to throttle repeats. `actor_id` is
the other player involved (attacker, planter, contest opponent; set null if they are deleted) and
`amount` the damage involved; the API joins the actor's display name when listing. A notice's
severity is not stored: it follows from its kind (`internal/game/notices.go`).
`push_tokens` holds one FCM token per phone, tied to the session that registered it; only tokens with
a live session receive pushes. `players.push_attacks` and `push_results` are the player's switches.

## Rules the database enforces itself
- carried resources and XP never go negative (CHECK)
- node reserve stays between 0 and 1 (CHECK)
- one equipped item per gear slot (partial unique index)
- one head per faction, and one faction per head (unique leader column, conditional claim)
- one faction per player (primary key)
- base spacing (trigger)

## Setting up a database
PostGIS is not a trusted extension, so a superuser enables it once per database:

```sql
CREATE DATABASE terrandominion_dev;
\c terrandominion_dev
CREATE EXTENSION postgis;
CREATE ROLE td_dev LOGIN PASSWORD '...';
GRANT CONNECT ON DATABASE terrandominion_dev TO td_dev;
GRANT CREATE ON DATABASE terrandominion_dev TO td_dev;  -- lets migrations create the td schema
```

Then `tdapi migrate` (as the `td_dev` role) creates everything else. Use a separate database and
role for live, and back each one up on its own schedule.
