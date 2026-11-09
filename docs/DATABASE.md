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
         ├─ xp_events                        XP ledger: every action that earned XP
         └─ faction_members ── factions ─┬─ faction_branches   regional branch, sub-leader
                                          └─ bases              faction bases (spacing trigger)
devices          anti-farming: welcome grant once per phone (stored as a keyed hash)
settings         every tunable number; edited with the admin API, cached for a minute
resource_types   crystal, alloy, data, credit
admin_actions    who changed what, and when
```

### players
The internal `id` is the key everywhere. Sign-in providers are linked through
`player_identities`, so a provider's user id never appears in names, file names or other players'
views. `level` is stored for fast leaderboards but always follows `xp` upward (it never drops, so
an admin may grant a level by hand). `delete_after` is the self-deletion grace period: signing in
before it cancels the deletion. `nakama_user_id` is reserved for a future move onto Nakama.

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
any interval). `source`/`source_ref` say whether a node was placed by hand or generated from
OpenStreetMap.

Every harvest stores the position and accuracy the phone reported and the computed distance. That
is the anti-spoofing record, and the source of the travel-speed check (`MaxTravelSpeedKmh`).

### gear
`gear_types` is the catalogue: slot (scanner, extractor, rig, drone_bay, shield), tier, minimum
level, stats as JSON (so new stats need no migration), craft cost as JSON, durability and the type
it upgrades to. `player_gear` is what a player owns; a partial unique index allows only one
equipped item per slot. Gear at durability 0 contributes nothing until repaired. New players get a
Mk I scanner and extractor. `player_items` holds the consumables a player has crafted (energy
cells and weapons) as a count per item; their costs are settings (`Craft*`).

### factions
Six placeholder factions are seeded, all `locked` with a lore reason. The first player at
`LeadFactionLevel` to claim one becomes its head: the claim is a conditional update, so of two
players racing, exactly one wins. `leader_player_id` is unique, so nobody heads two factions.
Locked factions cannot be joined. A player is in at most one faction (`faction_members` is keyed
by player); no row means freelancer. Ranks are numbers for now (1 member to 100 head).
Branches belong to a faction and a region (atoll or city) with a sub-leader the head appoints.

### bases
Faction bases grow in place (`tier`). The spacing rules are a trigger, so no code path can skip
them: one base per faction within `BaseSpacingSameFactionM` (E1402), and none within
`BaseSpacingOtherFactionM` of another faction's base (E1403). An advisory lock serialises base
inserts so two simultaneous builds cannot both pass.

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
