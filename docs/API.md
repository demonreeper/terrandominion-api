# API v1

Base URL: `https://<host>/v1`. JSON in and out. Authenticated calls send
`Authorization: Bearer <accessToken>`.

Every response is either `{"data": ...}` or `{"error": {"code": 1200, "message": "..."}}`. Clients
switch on the numeric code; the message is for display. Lists are always arrays, never null.
Request bodies are strict: unknown fields are rejected (E1000), so client typos show up at once.

## Auth
| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/auth/google` | `idToken, deviceId, platform, appVersion` | Signs in or creates the account. `platform`: android, ios, editor, pc |
| POST | `/auth/dev` | `name, deviceId, platform, appVersion` | Dev only (`TD_ENV=dev` and `TD_DEV_LOGIN=true`) |
| POST | `/auth/refresh` | `refreshToken, appVersion` | Returns a new pair; the old refresh token stops working |
| POST | `/auth/logout` | – | Signs this device out |

Sign-in returns `accessToken` (15 minutes by default), `accessExpiresAt`, `refreshToken` (60 days,
sliding), `newAccount`, and the player (same shape as `/me`). Refresh shortly before the access
token expires, or on E1001.

## Player
| Method | Path | Notes |
|---|---|---|
| GET | `/me` | Level and XP progress, carried resources, uploaded inventory (`uploaded`, no carry limit), Tether Nodes (`tethersOwned`, `tetherMax`, `tetherCost`), carry capacity, scan range, gear, faction (null = freelancer) |
| POST | `/me/delete` | Starts the deletion grace period (7 days by default) and signs every device out. Signing in cancels it |
| GET | `/settings` | Public tunable values (levels, ranges, cooldowns) for the client to display |

## Crafting
| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/craft` | `recipe` | Makes one consumable from `recipes` in `/me` (`items` holds the counts). Paid from carried, then uploaded |
| POST | `/gear/{id}/upgrade` | – | To the next tier in the same slot, at full durability. Needs the tier's level; cost is `upgradeCost` on the gear |
| POST | `/gear/{id}/repair` | – | To full durability for alloy (`repairCost` on the gear). No XP |

## World
| Method | Path | Body / query | Notes |
|---|---|---|---|
| GET | `/nodes/nearby` | `lat, lon, radius?` | Active nodes nearest first, within the scanner's range. `readyInS` = seconds until this player can harvest it again |
| POST | `/nodes/{id}/harvest` | `lat, lon, accuracyM` | Server checks GPS accuracy, distance, travel speed, cooldown, reserve and carry room |

## Tether Nodes
| Method | Path | Body / query | Notes |
|---|---|---|---|
| GET | `/tethers/nearby` | `lat, lon, radius?` | Active TNs nearest first, within the scanner's range, with their links (`nodeId, resource, lat, lon`) |
| POST | `/tethers` | `lat, lon, accuracyM` | Builds a tier 1 TN at that fix. Checks GPS, the per-player cap and the cost (carried first, then uploaded), and links free nodes in range |
| GET | `/tethers/{id}` | `lat, lon, accuracyM?` | One TN. `links` (name, resource, amount gathered) is only filled within `TnInfoRangeM`; `inRange` says which |
| POST | `/tethers/{id}/upgrade` | `lat, lon, accuracyM` | Owner only, in range. To tier 2, linking more free nodes. Refused if none is free |
| POST | `/tethers/{id}/dismantle` | – | Owner only. Refunds part of what was spent, plus everything it held, to the uploaded inventory, and frees its nodes |
| POST | `/tethers/{id}/collect` | `lat, lon, accuracyM` | Owner only, in range. Moves the held half (`stock` in nearby, owner only) to the uploaded inventory |
| POST | `/tethers/{id}/upload` | `lat, lon, accuracyM` | In range, faction rules apply. Moves everything carried to the player's uploaded inventory, less the charge paid to the owner |

## Map drops
| Method | Path | Body / query | Notes |
|---|---|---|---|
| GET | `/drops/nearby` | `lat, lon` | What ended Tether Nodes left on the map, within the scanner's range |
| POST | `/drops/{id}/pick` | `lat, lon, accuracyM` | In range. Moves what fits into carried; the rest stays on the map |

## Factions
| Method | Path | Notes |
|---|---|---|
| GET | `/factions` | All factions with status, leader and member count |
| POST | `/factions/{code}/lead` | Claim a dormant faction (first to claim leads) |
| POST | `/factions/{code}/join` | Join an active faction |

## Admin (admin accounts only)
| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/admin/settings` | – | Every setting with its description |
| PUT | `/admin/settings/{key}` | `value` | Changes one setting; logged in `admin_actions`; live within a minute |
| POST | `/admin/nodes` | `name, resource, lat, lon, regionCode, yield, regenPerHour` | Places a node by hand |
| POST | `/admin/nodes/scatter` | `lat, lon, count (1-40), radiusM (30-3000)` | Test nodes at random around a point (source `scatter`, fast regrowth) |
| POST | `/admin/nodes/clear-scatter` | – | Retires every scattered test node |

## Error codes
| Code | Meaning |
|---|---|
| 1000 | Bad request (malformed or unknown fields) |
| 1001 | Not signed in / token invalid: refresh or sign in |
| 1002 | Not allowed |
| 1003 | Not found |
| 1004 | Rate limited |
| 1005 | Request too large |
| 1100 | Google sign-in could not be verified |
| 1101 | Session expired: sign in again |
| 1102 | Account suspended |
| 1103 | Account deleted |
| 1104 | Device id missing |
| 1200 | Too far from the node |
| 1201 | GPS accuracy too poor |
| 1202 | Node cooling down for this player |
| 1203 | Node depleted |
| 1204 | Carry capacity full |
| 1205 | Location implausible (travel speed) |
| 1206 | Node not available |
| 1300 | Faction dormant (locked) |
| 1301 | Level too low |
| 1302 | Already in a faction |
| 1303 | Faction already has a leader |
| 1304 | Faction already active |
| 1305 | (log only) account cleanup skipped a faction head |
| 1402 | Base too close to the same faction's base |
| 1403 | Base too close to another faction's base |
| 1600 | Tether Node limit reached |
| 1601 | No free resource node in range of the build |
| 1602 | Not enough resources |
| 1603 | Tether Node gone |
| 1604 | Only the Tether Node's owner can do that |
| 1605 | Nothing to collect or upload |
| 1606 | Too far from the Tether Node |
| 1607 | Tether Node belongs to another faction |
| 1608 | Tether Node already fully upgraded |
| 1609 | Map drop gone |
| 1610 | Too far from the map drop |
| 1611 | Not allowed to attack this Tether Node |
| 1612 | Too close to another Tether Node |
| 1700 | Unknown recipe |
| 1701 | Gear not owned |
| 1702 | Gear already at its top tier |
| 1703 | Gear doesn't need repair |
| 1900 | Server error |

Codes are never reused. Logs write them as `E1200 ...` with the request id and player id.
