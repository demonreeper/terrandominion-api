# API v1

Base URL: `https://<host>/v1`. JSON in and out. Authenticated calls send
`Authorization: Bearer <accessToken>`.

Every response is either `{"data": ...}` or `{"error": {"code": 1712, "message": "...", "data": {...}}}`.
Clients switch on the numeric code. The message is a sentence ready to show ("Move within 40 m. You
are 62 m away."); the error's `data` holds the numbers behind it (see Error codes), so the app can
say it its own way. `data` is left out when there is nothing to add. Lists are always arrays, never
null. Request bodies are strict: unknown fields are rejected (E1000), so client typos show up at once.

The app sends `X-App-Build: <build number>` (the Android version code) and `X-App-Platform: android|ios`
on every call. A build below the `MinAppBuild` setting (`MinAppBuildIOS` on iPhones) gets E1006 on any
`/v1` call, so raising the setting locks a handed-out build out (`PUT /v1/admin/settings/MinAppBuild`).
Calls without `X-App-Build` (scripts, curl) are not checked.

Words: **stored** is the player's stored inventory (no carry limit; the `player_uploaded` table).
**Upload** is the action that moves carried resources into it at a Tether Node.

Copy style for every message and notice body: sentences with full stops, "Tether Node" spelled out,
numbers with units ("40 m", "4 min 10 s").

## Auth
| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/auth/google` | `idToken, nonce, deviceId, platform, appVersion` | Signs in or creates the account. `platform`: android, ios, editor, pc. `nonce` (optional) must match the one in the token. A new account gets the welcome grant once per phone: 800 crystal, 600 alloy, 200 data stored, and 5 energy cells |
| POST | `/auth/dev` | `name, deviceId, platform, appVersion` | Dev only (`TD_ENV=dev` and `TD_DEV_LOGIN=true`) |
| POST | `/auth/refresh` | `refreshToken, appVersion` | Returns a new pair; the old refresh token stops working |
| POST | `/auth/logout` | – | Signs this device out |

Sign-in returns `accessToken` (15 minutes by default), `accessExpiresAt`, `refreshToken` (60 days,
sliding), `newAccount`, and the player (same shape as `/me`). Refresh shortly before the access
token expires, or on E1001.

## Player
| Method | Path | Notes |
|---|---|---|
| GET | `/me` | Level and XP progress, carried resources (`carried`), stored inventory (`stored`, no carry limit), Tether Nodes (`tethersOwned`, `tetherMax`, `tetherCost`), carry capacity, scan range, gear, faction (null = freelancer) |
| POST | `/me/delete` | Starts the deletion grace period (7 days by default) and signs every device out. Signing in cancels it |
| GET | `/settings` | Unauthenticated. Public tunable values (levels, ranges, cooldowns) for the client to display, and which kind of server this is: `{"data": [{"key", "value", "description", "isPublic", "updatedAt"}], "environment": "dev"}`. `environment` sits beside `data` (so `data` stays the list) and is `"dev"` on a `TD_ENV=dev` server, otherwise `"prod"` |
| PUT | `/me/push` | Body `attacks, results` (booleans): the player's notification switches, also in `/me` as `pushAttacks`, `pushResults` |

## Push notifications
| Method | Path | Body | Notes |
|---|---|---|---|
| PUT | `/push/token` | `token, platform (android, ios)` | Registers this phone's FCM token against the signed-in session |
| DELETE | `/push/token` | – | Forgets this session's phones. Signing out does the same |

Data sent with each push: `kind`, `noticeId`, and `tnId` when it concerns a Tether Node.

## Crafting
| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/craft` | `recipe` | Makes one consumable from `recipes` in `/me` (`items` holds the counts). Paid from carried, then stored. A recipe with `minLevel` (the TN Shields: Mk I 5, II 10, III 15, IV 20) needs that level, else 1301 |
| POST | `/gear/craft` | `type` | Makes a piece of gear from `craftableGear` in `/me`, equipped in its empty slot. Needs its level |
| POST | `/gear/{id}/upgrade` | – | To the next tier in the same slot, at full durability. Needs the tier's level; cost is `upgradeCost` on the gear, and `upgradeStats` are the next tier's stats (Drone Bay Mk I → II → III → IV: `range_m, damage, drones, cells`) |
| POST | `/gear/{id}/repair` | – | To full durability for alloy (`repairCost` on the gear). No XP |

## World
| Method | Path | Body / query | Notes |
|---|---|---|---|
| GET | `/nodes/nearby` | `lat, lon, radius?` | Active nodes nearest first, within the scanner's range. `readyInS` = seconds until this player can harvest it again. `rare: true` and `expiresAt` on rare roaming nodes. Seeds the cells the scan touches for the first time (nearest first, at most `SeedCellsPerScan`), so the next scan may show more |
| POST | `/nodes/{id}/harvest` | `lat, lon, accuracyM` | Server checks GPS accuracy, distance, travel speed, cooldown, reserve and carry room. Returns `resource, amount, nodeReserve, carried, xp, progress, levelUp, readyInSec` (seconds until this player may harvest this node again) |

## Tether Nodes
| Method | Path | Body / query | Notes |
|---|---|---|---|
| GET | `/tethers/nearby` | `lat, lon, radius?` | Active TNs nearest first, within the scanner's range (or the Drone Bay's, whichever is larger), with their links (`nodeId, resource, lat, lon`), `shieldTier` (0 = none) and, on your own, `drones`: each drone flying at it `{id, mk, launchedAt, arrivesAt, from {lat, lon}, intercepted}` (where it came from, not who sent it) |
| GET | `/tethers/mine` | – | Every TN the player owns, wherever it is (no range check), plus ones that ended in the last `TnEndedShowMinutes` (60). Most urgent first; see My Tether Nodes below |
| POST | `/tethers` | `lat, lon, accuracyM` | Builds a tier 1 TN at that fix. Checks GPS, the per-player cap and the cost (carried first, then stored), and links free nodes in range. Returns `tether, carried, stored, xp, progress, levelUp` |
| GET | `/tethers/{id}` | `lat, lon, accuracyM?` | One TN. `links` (name, resource, amount gathered) is only filled within `TnInfoRangeM`; `inRange` says which |
| POST | `/tethers/{id}/upgrade` | `lat, lon, accuracyM` | Owner only, in range. To tier 2, linking more free nodes. Refused if none is free. Returns `tether, carried, stored, xp, progress, levelUp` |
| POST | `/tethers/{id}/dismantle` | – | Owner only. Refunds part of what was spent, plus everything it held, to stored, and frees its nodes. Returns `refund, held, stored` |
| POST | `/tethers/{id}/collect` | `lat, lon, accuracyM` | Owner only, in range. Moves the held half (`stock` in nearby, owner only) to stored. Returns `moved, stored` |
| POST | `/tethers/{id}/attack` | `weapon, lat, lon, accuracyM` | `em_pulse`, `signal_jammer` or `overload_spike`. Checks GPS, `WeaponRangeM` and the attack rules; uses the item and its energy cells. A shield cuts the damage (and jam time) by `ShieldShare` (weapons count as Mk I), and no single hit takes a shielded node below 1 health. Returns `health`, `damage` (EM Pulse), `destroyed`, `jammedUntil` (jammer only), `spikeFiresAt` (spike only) |
| POST | `/tethers/{id}/drone` | `lat, lon, accuracyM` | Needs a working Drone Bay; range is the bay's `stats.range_m` (Mk I 150 m, Mk II 500 m, Mk III 1 km, Mk IV 5 km) plus the fix's accuracy (at most doubling it), else 1712 with `distanceM, rangeM`. Attack rules apply. Uses the bay's `stats.cells` energy cells (Mk IV 3) and `DroneLaunchWear` (5) durability; at most `stats.drones` in flight. Flies at `DroneSpeedMps`, at least `DroneMinFlightSec`. Returns `id, tnId, mk, launchedAt, arrivesAt, distanceM, from {lat, lon}, to {lat, lon}, damage` (what lands without an intercept, after the shield) |
| POST | `/tethers/{id}/intercept` | `lat, lon, accuracyM` | Owner only, within `WeaponRangeM`, `CellsIntercept` energy cells. Intercepts the soonest-landing drone not yet intercepted; it stops `InterceptShareMk1..4` of its damage by the drone's Mk (Mk I 100%: the drone is void at once). 1732 when no drone is inbound, 1733 when all are already intercepted. Returns `droneId, share, shotDown` |
| POST | `/tethers/{id}/shield` | `item, lat, lon, accuracyM` | Owner only, within `WeaponRangeM`. Fits a crafted `tn_shield_mk1`..`mk4` (uses the item); lasts as long as the node; 1734 unless it is above the fitted one. Returns `shieldTier` |
| GET | `/drones` | – | The player's own strikes in flight and those ended in the last `DroneResultsMinutes` (10): `id, tnId, mk, status (flying, hit, void), launchedAt, arrivesAt, from, to, intercepted, damage` (`damage` once landed) |
| POST | `/tethers/{id}/defuse` | `lat, lon, accuracyM` | Owner only, in range, before the fuse ends |
| POST | `/tethers/{id}/upload` | `lat, lon, accuracyM` | In range, faction rules apply. Moves everything carried to the player's stored inventory, less the charge paid to the owner. Returns `moved` (what went into storage this time), `fee`, `carried`, `stored` (new totals) |

### My Tether Nodes
`GET /tethers/mine` returns an array of:

```
{ "id": 12, "name": "Crystal Seam K-27", "tier": 1, "lat": 4.17, "lon": 73.50,
  "health": 72.5, "maxHealth": 100, "expiresAt": "...", "status": "active", "endedAt": null,
  "stock": {"crystal": 14},
  "links": [{"nodeId": 301, "resource": "crystal", "name": "Crystal Seam K-27"}],
  "jammedUntil": null, "spikeFiresAt": null, "droneArrivesAt": null, "attackedAt": "...",
  "underAttack": true, "shieldTier": 2,
  "drones": [{"id": 7, "mk": 2, "launchedAt": "...", "arrivesAt": "...", "from": {"lat": 4.18, "lon": 73.51}, "intercepted": false}] }
```

- `name`: the node the TN stands by (its first link, or for an ended one the nearest node within
  `TnLinkRadiusM`); `""` when there is none. Notices use the same name.
- `status`: `active`, `destroyed`, `decayed` (out of health or past its lifetime) or `dismantled`.
  `endedAt` is null while active. An ended TN has no links, no stock and no timers.
- `stock`: the held half, resource to amount (`{}` when empty).
- `jammedUntil` only while the jam runs; `spikeFiresAt` while a spike is armed; `droneArrivesAt` the
  first drone still flying at it; null otherwise. `attackedAt` is the last hit (null if never).
- `underAttack`: active and hit within the last `TnUnderAttackMinutes` (10).
- Order: under attack, then spike or drone, then jammed, then soonest `expiresAt`; ended ones last,
  most recently ended first.

## Node contests and notices
| Method | Path | Body / query | Notes |
|---|---|---|---|
| POST | `/nodes/{id}/contest` | `lat, lon, accuracyM` | In range, `CellsContest` energy cells. Needs another player's recent harvest or hold (`canContest` in `/nodes/nearby`). Returns `id, endsAt` |
| POST | `/contests/{id}/join` | `lat, lon, accuracyM` | In range: the defender, or a faction ally of either side |
| GET | `/notices` | `after?` | What happened to the player after that notice id, newest first (50 at most) |

`/nodes/nearby` also gives `contestId, contestEndsAt, contestRole, canContest, heldByMe, heldByOther`. `/tethers/*` gives `jammedUntil, attackedAt, spikeFiresAt, droneArrivesAt`.

Actions that start a timer say when it ends (times are ISO 8601): harvest `readyInSec`, jammer
attack `jammedUntil`, spike attack `spikeFiresAt`, drone `arrivesAt`, contest start `endsAt`.

### Notices
`GET /notices` returns an array of:

```
{ "id": 88, "kind": "attacked", "severity": "danger", "tnId": 12, "actor": "Aishath", "amount": 25,
  "body": "Aishath's EM Pulse hit your Tether Node near Crystal Seam K-27 for 25 damage.",
  "createdAt": "..." }
```

`severity` (`info`, `warning`, `danger`) picks the sound and haptic; unknown future kinds are
`info`. `tnId`, `actor` (the other player's current display name) and `amount` (whole number) are
null when they don't apply. `body` is a finished sentence. Every kind:

| Kind | Severity | To | Meaning | `actor` | `amount` |
|---|---|---|---|---|---|
| `attacked` | danger | TN owner | An EM Pulse or a drone hit your TN and it survived (with the health left) | attacker | damage, after intercept and shield |
| `destroyed` | danger | TN owner | An EM Pulse, spike or drone destroyed your TN | attacker | damage of the last hit |
| `spike` | danger | TN owner | A spike was planted on your TN; defuse it before it fires | planter | damage it will deal |
| `spike_fired` | danger | TN owner | A spike went off on your TN and it survived | planter | damage |
| `drone_inbound` | danger | TN owner | A drone is flying at your TN | launcher | damage it will deal |
| `jammed` | warning | TN owner | Your TN was jammed and collects nothing for a while | jammer | null |
| `contest` | warning | defender | Someone is contesting a node you harvested or hold; be within range before it ends | challenger | null |
| `contest_lost` | warning | loser | You lost a node contest | winner | null |
| `contest_won` | info | winner | You won a node contest and hold its next harvests | loser | null |
| `spike_result` | info | planter | Your spike went off: it hit or destroyed the TN | TN owner | damage |
| `drone_result` | info | launcher | Your drone landed: it hit or destroyed the TN (with the health left) | TN owner | damage |
| `drone_launched` | info | launcher | Your drone is away, with its flight time and damage | TN owner | damage it will deal |
| `drone_intercepted` | warning | both | The owner intercepted a drone: the share stopped, or shot down | the other player | null |
| `shield_fitted` | info | TN owner | A shield was fitted to your TN | null | null |
| `mission_done` | info | player | A mission is done; claim it in ORDERS (from its lore power) | null | null |
| `invitation` | info | freelancer | At level 20, the lore power they followed most invites them to its faction | null | null |
| `warden` | info | member | Their Regional Commander made them a Warden | commander | null |
| `defused` | info | planter | The owner defused your spike | TN owner | null |

Contest notices have `tnId` null.

## Map drops
| Method | Path | Body / query | Notes |
|---|---|---|---|
| GET | `/drops/nearby` | `lat, lon` | What ended Tether Nodes left on the map, within the scanner's range |
| POST | `/drops/{id}/pick` | `lat, lon, accuracyM` | In range. Moves what fits into carried; the rest stays on the map |

## Orders and daily missions
| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/missions` | – | Today's missions (the Maldives date; `MissionsPerDay` 4, refreshing at local midnight, `MissionDayOffsetMinutes` 300), given on first ask: freelancers get one from each of 4 different lore powers, members from their own faction's, Regional Commanders and Wardens one command order among theirs. Returns `day, refreshesAt, missions, invitation` |
| POST | `/missions/{id}/claim` | – | Pays a done mission: XP, credits into storage, its item if any. Returns `reward, xp, progress, levelUp` |

Each mission: `{id, persona, factionCode, color, audience, goal, goalKey, count, progress, status (active, done, claimed),
reward {xp, credits, item, itemCount}, line, title}`. `persona` is the lore power that gave it (Signum, Shingō,
Sēma, Ishaaraath, Tohu, Merki); `line` is its message; `title` says what to do ("Harvest crystal 3 times"; harvest missions count harvests, not amounts). Progress counts
on the server inside each action (harvest, build, upgrade and collect at a Tether Node, upload, craft, gear upgrade and
repair, attack, drone, intercept, shield, appoint a Warden). `invitation` (`factionCode, factionName, persona, color,
line`) is set once a freelancer reaches `LeadFactionLevel`: the lore power whose missions they finished most invites
them. Rewards grow with level (`MissionXpBase`, `MissionXpPerLevel`, `MissionCreditBase`, `MissionCreditPerLevel`),
`MissionItemChance` of a low-level item, command orders `MissionCommandMult` times.

## Factions
| Method | Path | Notes |
|---|---|---|
| GET | `/factions` | All factions in the player's region (a country; `MV` at launch): `status` (`locked` until any region is claimed), `leaderName` (the Regional Commander here, empty when nobody holds it), `members` (here), `region` |
| POST | `/factions/{code}/lead` | Claim regional command of the faction in your region (one Regional Commander per faction per country; first to claim wins). Rank 90. Returns the membership |
| GET | `/factions/branch` | Your branch: `factionCode, factionName, region, members [{playerId, name, level, rank, rankTitle}], canAppoint` (Regional Commander). 1763 when you're in no branch |
| POST | `/factions/branch/wardens` | `playerId`: the Regional Commander makes a member of their branch a Warden (rank 50). 1762 if you aren't, 1763 if they aren't in your branch, 1764 if already Warden or above |
| POST | `/factions/{code}/join` | Join the faction's branch in your region, once it has a Regional Commander. Rank 1. Returns the membership |

Memberships (here and `faction` on `/me`) carry `factionId, factionCode, factionName, rank, branchId,
region, rankTitle`. `/me` also lists `badges`: `[{badge, factionCode, factionName, region}]`; `founder`
goes to the first player ever to command a faction's branch in a country, and is permanent. Ranks: 1 Member, 50 Warden, 90 Regional Commander; 100 is the shared lore AI
above every faction (internal name `signal`; players see a separate commander per faction), never a player.

## Admin (admin accounts only)
| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/admin/settings` | – | Every setting with its description |
| PUT | `/admin/settings/{key}` | `value` | Changes one setting; logged in `admin_actions`; live within a minute |
| POST | `/admin/nodes` | `name, resource, lat, lon, regionCode, yield, regenPerHour` | Places a node by hand |
| POST | `/admin/nodes/scatter` | `lat, lon, count (1-40), radiusM (30-3000)` | Test nodes at random around a point (source `scatter`, fast regrowth) |
| POST | `/admin/nodes/clear-scatter` | – | Retires every scattered test node |
| POST | `/admin/push/test` | – | Sends "Test push" to your own phones; returns `sent` |
| GET | `/admin/seed` | `lat, lon` (query) | Seeding readout: `cell` `{osmLoaded, cellId, seeded, target, seedNodes, walkM, totalCells, totalNodes}` for the cell at that point, and `areas` `[{id, name}]` |
| POST | `/admin/seed` | `areaId` | Pre-seeds every cell of an area in `td.seed_areas` (`male`, `hulhumale`, `villimale`) in the background, one transaction per cell; cells already seeded are left alone. Returns `started`. Command line: `tdapi seed <area or all>` |
| POST | `/admin/seed/cell` | `lat, lon` | Retires the seed nodes in the cell at that point and seeds it again. Returns `cellId, target, created, retired` |

## Error codes
`data` values are whole numbers unless noted. Distances are metres (`distanceM` rounded up, so it
never reads as inside the range).

| Code | Meaning | `data` |
|---|---|---|
| 1000 | Bad request (malformed or unknown fields) | |
| 1001 | Not signed in / token invalid: refresh or sign in | |
| 1002 | Not allowed | |
| 1003 | Not found | |
| 1004 | Rate limited | |
| 1005 | Request too large | |
| 1006 | App build locked out (HTTP 426): below `MinAppBuild` / `MinAppBuildIOS` | `build`, `minBuild` |
| 1100 | Google sign-in could not be verified | |
| 1101 | Session expired: sign in again | |
| 1102 | Account suspended | |
| 1103 | Account deleted | |
| 1104 | Device id missing | |
| 1200 | Too far from the node | `distanceM, rangeM` |
| 1201 | GPS accuracy too poor (every action that takes a fix) | `accuracyM, maxAccuracyM` |
| 1202 | Node recharging for this player | `readyInSec` |
| 1203 | Node depleted | |
| 1204 | Carry capacity full | |
| 1205 | Location implausible (travel speed) | |
| 1206 | Node not available | |
| 1300 | No Regional Commander for this faction in your region yet (join) | |
| 1301 | Level too low (factions, gear craft and upgrade) | `needLevel, level` |
| 1302 | Already in a faction | |
| 1303 | Faction already has a Regional Commander in your region | |
| 1304 | (unused since regional command) Faction already active | |
| 1305 | (log only) account cleanup skipped a faction head | |
| 1402 | Base too close to the same faction's base | |
| 1403 | Base too close to another faction's base | |
| 1600 | Tether Node limit reached | `max` |
| 1601 | No free resource node in range of the build | |
| 1602 | Not enough resources (TN build and upgrade, crafting, gear craft, upgrade and repair) | `short`: `{resource: amount still missing}` |
| 1603 | Tether Node gone | |
| 1604 | Only the Tether Node's owner can do that | |
| 1605 | Nothing to collect or upload | |
| 1606 | Too far from the Tether Node (collect, upload, upgrade) | `distanceM, rangeM` |
| 1607 | Tether Node belongs to another faction | |
| 1608 | Tether Node already fully upgraded | |
| 1609 | Map drop gone | |
| 1610 | Too far from the map drop | `distanceM, rangeM` |
| 1611 | Not allowed to attack this Tether Node | |
| 1612 | Too close to another Tether Node | `spacingM` |
| 1700 | Unknown recipe | |
| 1701 | Gear not owned | |
| 1702 | Gear already at its top tier | |
| 1703 | Gear doesn't need repair | |
| 1710 | Not enough energy cells | `need, have` |
| 1711 | Item missing (the weapon used) | `item` (string id) |
| 1712 | Out of range (weapons, defuse, drone, contest start and join) | `distanceM, rangeM` |
| 1713 | Tether Node already jammed | `jammedUntil` (ISO time) |
| 1714 | Spike already planted | |
| 1715 | No spike to defuse | |
| 1716 | Node contested: another player holds its next harvests | |
| 1717 | Contest already running | |
| 1718 | Nobody to contest the node with | |
| 1719 | Contest over | |
| 1720 | Can't join this contest | |
| 1730 | No working Drone Bay | |
| 1731 | Drone already in flight (as many as the bay's `stats.drones`) | |
| 1732 | No drone flying at this Tether Node (intercept) | |
| 1733 | Every inbound drone is already intercepted | |
| 1734 | Shield not higher than the fitted one | `shieldTier` |
| 1740 | Push token not valid | |
| 1741 | Push not set up on this server | |
| 1742 | No phone registered for push | |
| 1743 | Push could not be sent | |
| 1760 | Mission gone or already claimed | |
| 1761 | Mission not done yet | `progress, count` |
| 1762 | Only the Regional Commander can do that | |
| 1763 | Not in your faction's branch | |
| 1764 | Already a Warden or above | |
| 1750 | Seeding map data not imported (admin seed calls; run `scripts/import-osm.ps1`) | |
| 1900 | Server error | |

Codes are never reused. Logs write them as `E1200 ...` with the request id and player id.
