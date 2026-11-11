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
| POST | `/auth/apple` | `identityToken, authorizationCode, nonce, name, deviceId, platform, appVersion` | Sign in with Apple (iOS). Same result and welcome grant as `/auth/google`; an Apple sign-in is its own account (no linking to a Google one). `nonce` (required) is the raw nonce; the app sent its hex SHA-256 to Apple and the token must carry that. `name` is only sent on the first sign-in and names a new account. `authorizationCode` is accepted but not used yet (kept for revoking on account deletion, before App Store) |
| POST | `/auth/dev` | `name, deviceId, platform, appVersion` | Dev only (`TD_ENV=dev` and `TD_DEV_LOGIN=true`) |
| POST | `/auth/refresh` | `refreshToken, appVersion` | Returns a new pair; the old refresh token stops working |
| POST | `/auth/logout` | – | Signs this device out |

Sign-in returns `accessToken` (15 minutes by default), `accessExpiresAt`, `refreshToken` (60 days,
sliding), `newAccount`, and the player (same shape as `/me`). Refresh shortly before the access
token expires, or on E1001.

## Player
| Method | Path | Notes |
|---|---|---|
| GET | `/me` | `code` (the 8-character share code), `name` (what other players see: the callsign, or `OPERATIVE-<code>` until one is picked), `callsign` (null until picked), `callsignChangeAt` (when it may next be changed; absent when it can be changed now) and `shareUrl` (`<TD_PUBLIC_URL>/p/<code>`). The Google or Apple account's real name is never sent. Level and XP progress, carried resources (`carried`), stored inventory (`stored`, no carry limit), Tether Nodes (`tethersOwned`, `tetherMax`, `tetherCost`), carry capacity, scan range, gear, faction (null = freelancer), and `linked`: the sign-ins that open this account (`apple`, `google`; `dev` on dev) |
| POST | `/me/link/google` | Body `idToken, nonce` (as `/auth/google`; nonce required). Adds that Google sign-in to the signed-in account, so it opens this account from then on. Already on this account: no change. Opens another account: E1106 (accounts are never merged). A different Google already linked: E1108. Returns `{linked}` |
| POST | `/me/link/apple` | Body `identityToken, authorizationCode, nonce` (as `/auth/apple`). Same rules as `/me/link/google` |
| DELETE | `/me/link/{provider}` | `google` or `apple`: removes that sign-in. Refused with E1107 when it is the account's only one, E1003 when it isn't linked. Returns `{linked}` |
| GET | `/me/callsign/options` | Callsigns are picked, not typed. `options`: six made-up callsigns nobody has (`Night-Heron-42`: two words from the game's lists and a two-digit number), new ones on every call; `changeAt` when the player can't change yet |
| PUT | `/me/callsign` | Body `callsign`: one of the offered callsigns. Anything not of that make is refused (1109), so a name can't be typed even by calling the API by hand; a callsign typed before this rule stays until its owner changes it. Unique; not a blocked word (`td.callsign_blocked`). The first pick is free; after that one change every `CallsignChangeDays` (30). Errors carry `data.error`: 1109 `callsign_invalid`, 1110 `callsign_taken` (also said for a blocked or reserved name), 1111 `callsign_too_soon` (`changeAt, days`). Returns the new `/me` |
| POST | `/me/delete` | Starts the deletion grace period (7 days by default) and signs every device out. Signing in cancels it |
| GET | `/settings` | Unauthenticated. Public tunable values (levels, ranges, cooldowns) for the client to display, and which kind of server this is: `{"data": [{"key", "value", "description", "isPublic", "updatedAt"}], "environment": "dev"}`. `environment` sits beside `data` (so `data` stays the list) and is `"dev"` on a `TD_ENV=dev` server, otherwise `"prod"` |
| PUT | `/me/push` | Body `attacks, results` (booleans): the player's notification switches, also in `/me` as `pushAttacks`, `pushResults` |

## Players (PlayerRef)

Another player is always named by a **PlayerRef**: `{ "code": "K7QX3M9P", "name": "ECHO-7" }`. `code` is
the share code that opens their profile; `name` is their callsign, or `OPERATIVE-<code>` until they pick
one. A deleted or banned player is `{ "code": "", "name": "Deleted player" }`: show the name, don't link
it. The counting player id and the account's real name are never sent to another player. PlayerRefs:
`owner` on a Tether Node, `leader` on a faction, `actor` on a notice (null for system notices) and each
branch member (`code`, `name` inline). Plan: `docs/profile-plan.md`.

### Profile
`GET /players/{code}` (signed in): the public profile behind a share code. 1003 for an unknown, deleted or
banned player, or text that is not a code.

```
{ "code": "K7QX3M9P", "name": "ECHO-7", "level": 12,
  "faction": { "code": "lumen", "name": "Lumen Collective", "color": "#4fd8ff", "rank": 50, "rankTitle": "Warden" },
  "badges": [ { "badge": "founder", "factionCode": "lumen", "factionName": "Lumen Collective", "region": "MV" } ],
  "record": { "activeSince": "2026-11-01T00:00:00Z", "nodesHarvested": 214, "tethersBuilt": 9, "tethersActive": 2,
              "tethersDestroyed": 3, "dronesIntercepted": 1, "missionsCompleted": 17 },
  "isMe": false, "shareUrl": "https://td-dev.demonreeper.xyz/p/K7QX3M9P" }
```

`faction` is null for a freelancer. That is the whole public list: no real name, email, sign-ins, resources,
gear, items, Tether Node places or last-seen time. Your own profile adds XP and resources from `/me` on the phone.
`record` is counted from the action tables; `tethersDestroyed` is Tether Nodes this player destroyed
(`tether_nodes.destroyed_by`), `dronesIntercepted` drones stopped at their own Tether Nodes, `activeSince` the
first day of the month the account was made.

### Profile web page (share links)
`GET /p/{code}` (public, no sign-in, not under `/v1`): the web page behind a profile share link, HTML from
`web/profile.html` embedded in `tdapi`. It shows the same public fields as `GET /v1/players/{code}`, with Open
Graph and Twitter tags (`og:title`, `og:description`, `og:url`) so chat apps draw a preview card, an
"Open in Tether Rise" link (the share link itself: it opens the game once App Links are set up, P3) and a
store button for each of `TD_PLAY_URL` and `TD_APPSTORE_URL` that is set. An unknown, deleted or banned player,
or text that is not a code, gets the same page saying "This operative is no longer active." with 404.
`Cache-Control: public, max-age=300`; rate limited per IP like every request. `/me` and a profile carry the
link as `shareUrl` (`TD_PUBLIC_URL` + `/p/` + code; the dev address until the release domain is bought).

### Opening a share link in the game
Android and iOS each fetch a file from the link's domain to check that the app may open its links. Both are
bare JSON (no `data` envelope), public, `Cache-Control: public, max-age=300`, and must never be redirected.

| Path | What |
|---|---|
| `GET /.well-known/assetlinks.json` | Android App Links: `TD_ANDROID_PACKAGE` (default `com.hisstudios.terrandominion`) with `TD_ANDROID_CERT_SHA256`, the SHA-256 fingerprint of every key an installed build is signed with (comma-separated: this PC's debug key, the upload key, later the Play App Signing key). An empty list `[]` when none is set |
| `GET /.well-known/apple-app-site-association` | iOS Universal Links: `TD_APPLE_APP_IDS` (`<team id>.<bundle id>`, default `J2P3QBTV3J.com.hisstudios.terrandominion`) for the path `/p/*` only |

The app declares the same host (`AndroidManifest.xml` intent filter with `autoVerify`; the Associated Domains
entitlement `applinks:<host>` from `IosPostBuild`), so a new share domain needs a new app build as well as these
two variables. Check Android's verdict with
`adb shell pm get-app-links com.hisstudios.terrandominion` (`verified` for the host).

### Reports and moderation
A player reports another for griefing; moderators (`is_mod`) and admins act on it. A ban or a suspension
only stops sign-in: nothing the player owns is removed, and their Tether Nodes decay as normal.

| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/players/{code}/report` | `detail, note?` | Any signed-in player. `detail` is what happened: `repeated_attacks`, `blocking_node`, `following` or `other`; `note` up to 300 characters. The server attaches what each of the two did to the other in the last `ReportContextDays` (7): weapon hits, drone strikes, intercepts and contests. 1113 yourself, 1114 bad detail or note, 1115 you already have an open report on them, 1116 over `ReportsPerDay` (5), 1003 unknown player |
| GET | `/mod/reports` | – | Mods and admins. `players`: open reports grouped by reported player, most reporters first: `player {code, name}`, `level`, `faction`, `rankTitle`, `reporters` (how many different players), `reports [{id, reason, detail, note, context, createdAt}]`, `sanctions` (their history), `suggested {kind, days?}` (the next step up, shown not forced) and `canAct`. Also `suspendDays` (the three lengths) and `details`. No real name, email, device or location, and not who reported |
| POST | `/mod/reports/{id}/dismiss` | `reason, all?` | Closes the report with no action (`all`: every open report on that player). Each reporter gets a `report_reviewed` notice |
| POST | `/mod/players/{code}/sanction` | `kind, days?, reason, reportIds?` | `warn` (a written message from Moderation with the reason), `suspend` (`days` is one of `SuspendDays1/2/3`: 1, 3, 7) or `ban`. A reason of 3 to 500 characters is required (1117). A suspension or ban signs the player out everywhere at once. The reports named (all open ones on that player when left out) close as actioned and their reporters are told it was reviewed, never what was done. A mod can't act on a mod, an admin or themselves; an admin can't act on another admin or themselves (1118). Returns the sanction |
| GET | `/admin/sanctions` | – | Admins. `sanctions` (active first: `{id, player, kind, reason, until, by, createdAt, liftedAt, liftedBy, liftReason, active}`) and `dismissed` (reports mods dismissed) |
| POST | `/admin/sanctions/{id}/lift` | `reason` | Ends a running suspension or ban early; the player can sign in again at once unless another sanction still holds. Both entries stay in the history. 1119 when it is not running |
| POST | `/admin/reports/{id}/reopen` | – | Puts a dismissed report back in the queue (1120 otherwise) |

Sign-in and refresh answer 1112 with `until` for a suspended account ("Suspended until …") and 1102 for a banned
one. A suspension that has run out returns the account to `active` (on the next sign-in, and by the tick). To
other players a banned account reads `Deleted player` with no code; staff still see the callsign. Every action
is written to `admin_actions`. `/me` carries `isMod`.

### Admin players and stats
Staff look a player up; admins see the private side and change things. Every change is written to
`admin_actions` with the player's id as its target.

| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/mod/players?q=` | – | Mods and admins. At least 2 characters: a piece of a callsign or share code; for admins also the player's id or a piece of an email. `players [{player {code, name}, level, status, faction, isAdmin, isMod, lastSeen}]`, last seen first, at most 30. Banned accounts are found too |
| GET | `/mod/players/{code}` | – | Mods and admins. `player`, `level`, `xp`, `status`, `suspendedUntil`, `isAdmin`, `isMod`, `faction`, `rankTitle`, `tethersActive`, `registered`, `lastSeen`, `reportsAgainst`, `reportsBy`, `sanctions`, `canAct` (may warn, suspend or ban them), `suspendDays`. Admins also get `private`: `id`, `realName`, `signIns [{provider, email}]`, `platform`, `appVersion`, `carried`, `stored`, `sharedPhone` (other accounts signed in on the same phone), `infringementKinds [{kind, count, last}]`, `infringements` (the last 20: `kind, detail, path, at`), `actions` (the last 20 admin actions on them: `action, by, at, detail`). A moderator never gets `private` |
| POST | `/admin/players/{code}/adjust` | `xp?, carried? {resource: amount}, stored? {resource: amount}, reason` | Adds, or with a minus takes away. Reason of 3 to 500 characters. The level follows the new XP. The carry limit is not applied. 1121 when nothing changes, an amount is over 1,000,000, the resource is unknown or it would go below zero. Returns the player info |
| GET | `/admin/marks/{appealCode}` | – | A device-record mark by its appeal code: `kind` (`device` or `signin`), `factionCode`, `factionName`, `banned`, `appealCode`, `createdAt`. For an appeal from someone who can't sign in at all |
| POST | `/admin/marks/{appealCode}/clear` | `reason` | Removes the whole mark after an appeal (proof checked by the admin, then deleted; it is never stored here). Logged as `mark.clear` |
| POST | `/admin/players/{code}/release-pledge` | `reason` | RELEASE PLEDGE: returns a member to freelancer, for a mistaken pledge (players can't undo one). A Regional Commander's seat is left empty. Logged as `player.release_pledge`. 1121 when they have not pledged |
| POST | `/admin/players/{code}/reset-callsign` | `reason` | Removes the callsign (for an offensive one). The player is `OPERATIVE-<code>` until they pick again, which they may do at once, and gets a written message from Moderation. 1121 when they have none |
| PUT | `/admin/players/{code}/mod` | `isMod` | Makes or removes a moderator; it applies on their next call |
| GET | `/admin/stats` | – | Counts only, no names, nothing with a place: `playersTotal`, `newWeek`, `active24h`, `active7d`, `active30d` (by last seen), `harvestsMonth` (this calendar month, Maldives time), `tethersActive`, `fightsWeek` (weapon hits, drone strikes and contests in 7 days), `factions [{name, color, members}]`, `retained` of `retainedOf` (players who joined 7 to 14 days ago, and those seen a week or more after joining) with `retentionPct`, `days` (14 days, oldest first: `{day, active}`, a player being active on a day they earned XP), `asOf` |

Suspend, ban and lift from a player's info use the moderation endpoints above. Failed location checks
(E1201 GPS too weak, E1205 travel speed) are stored in `td.infringements` when the server refuses the
request: at most one of each kind per player in `InfringementGapMinutes` (10), with the numbers behind
it and the path, never a position.

### The seventh voice
Langis (freelancer-path-plan.md phase 4, LORE.md). Nothing in any answer names it before its first call is due:
its account (id 8, `signal_unbound`) and lines stay on the server until then.

- **Calls** (`GET /v1/calls`, same rules as the powers' calls): `langis_l18` halfway through `LangisCallLevel` (18) and
  `langis_l19` `LangisCallDelayHours` (24) after its message. They carry `persona: "Unknown caller"`, no faction,
  `voice: "langis"`, `audio` (`level18-call`, `level19-call`), `style: "static"` (no silhouette, no eyes) and
  `reveal: "LANGIS"` (the caller id resolves to it in the call's last seconds). An unfetched call sends the push
  "Missed call from Unknown caller".
- **Messages**: `langis_l19_msg` `LangisMessageDelayHours` (2) after the power's level 19 day-3 message, from sender
  `LANGIS`, colour `#C8D2DC`, `style: "glitch"`; after the faction level, rare messages at most once in
  `LangisRareEveryDays` (7), in turn. All of its messages carry `style: "glitch"`.
- **`/me`** carries `contact` (its first call was heard or turned down), `jobsUnlocked` (the same, for a freelancer)
  and, at the faction level, `note: {text: "Stay free.", sign: "// L."}` for FACTIONS. A member never has them.
- It never contacts a member: a pledge drops whatever of it has not reached the player, and reaching the faction
  level drops its level 19 message and call if they had not come due.

### JOBS board
F6 (phase 6): faction players at Warden and above post paid jobs; only freelancers who have had the seventh voice's
level 18 call (`/me` `jobsUnlocked`) take them. Members can't take jobs.

| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/jobs` | – | A freelancer with the board: `board` (open jobs with free slots, soonest deadline first, their own takes first, with `myStatus` and `guardSeconds`), `canTake: true`. A Warden or above: `posted` (their faction's jobs lately), `canPost: true`. Also `kinds`, `slotsMax`, `hoursMin`, `hoursMax`, `rewardMin`, `rewardMax`, `guardMinutes` |
| POST | `/jobs` | `kind, tetherId?, nodeId?, lat?, lon?, reward {resource: amount}, slots, hours` | `hit` (`tetherId`: not your own nor your faction's), `scout` (`lat, lon`: an area of `JobScoutRadiusM` around it) or `guard` (`nodeId`). `reward` is per slot, all resources together from `JobRewardMin` to `JobRewardMax`; the reward for every slot leaves your storage now (escrow, 1774 when short). Limits: `JobSlotsMax`, `JobHoursMin`..`JobHoursMax`, `JobsOpenPerFaction`, `JobsPostedPerDay` (1771) |
| POST | `/jobs/{id}/take` | – | A freelancer takes a slot. Refused (1773) against their own Tether Node, for a poster who signs in on the same phone, and for a hit job when they already hit that owner `XpAttackPerTargetDay` times today; 1771 past `JobsTakenMax`; 1772 no slot or ended |

A job is a **job** (`id, kind, factionCode, factionName, factionColor, target, tetherId, nodeId, lat, lon, radiusM, reward,
slots, taken, done, status, deadline, createdAt`); the poster is shown by faction only. **The server checks completion:**
a weapon hit or a landed drone on the target Tether Node finishes a hit job; a scan (`GET /nodes/nearby` with
`accuracy`) inside the area finishes a scout job; scans near the node add up time for a guard job (`JobGuardRadiusM`, each
gap counted up to `JobGuardGapSeconds`) until `JobGuardMinutes`. A finished job pays the taker into storage at once
(notice `job_paid`; the poster gets `job_done`). Past the deadline unfinished takes fail and the reward for every unpaid
slot goes back to the poster (`job_ended`). Errors 1770 to 1774.

### Regions from GPS (social-signal-plan.md Part B, R1)

A region is a whole country, found from the player's fix: inside a Natural Earth admin-0 shape, else the nearest
country within `CountryAtMaxKm` (reefs, lagoons, the sea). `POST /factions/{code}/lead` and `/join` take
`{lat, lon, accuracyM}` (and pass the anti-spoofing gate) and use that country's branch; no fix counts as the launch
region (old builds). Only a live country can be claimed: **1796** `RegionNotLive` (data `region`); nowhere near a
country: **1795**. `GET /factions?lat=&lon=` shows each faction's standing in that region (no fix: your own branch's
region). `GET /regions/here?lat=&lon=` is `{code, name, live, test?}`.

Admins: `GET /admin/regions` (`test`, `live`), `PUT /admin/regions/live/{code} {live}` (LiveRegions), and on dev only
`PUT /admin/regions/test/{code} {name, points: [[lat, lon], ...]}` / `DELETE` for test regions `XA`..`XZ` (a point
inside one counts as that region; never on production). The shapes are loaded once per database with
`go run ./cmd/loadcountries` (from `.env`' `TD_DATABASE_URL`).

### Viewing another region (social-signal-plan.md Part B, R2)

A member can view any **unlocked** region of their own faction (one with a Regional Commander) from anywhere, free,
as often as they like: **talk only**. The home region (`faction_members.branch_id`) never changes: rank, seat, vote,
jobs, orders and missions stay there.

| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/factions/regions` | | `{regions: [{region, name, commander, members, home, viewing}], home, viewing}`; unlocked regions only. 1798 for a freelancer |
| POST | `/factions/view-region` | `region` (a code, or `home`) | 1797 `RegionLocked` for a region without a commander (or not the faction's) |
| POST | `/admin/players/{code}/move-region` | `region, reason` | MOVE REGION: a new home for a player who really moved; they become a member there and a seat they held is left empty. Logged |

`GET /factions/branch` shows the viewed region: `region`, `regionName`, `visiting`, `home`, `visitors` (members of other
regions viewing it, each `visitor: true` with `from`), and `canAppoint` only at home. A region that goes dormant sends
its visitors home. Admin player info has `region` and `viewingRegion`; `/me` gives an admin their own `spoofExempt`.

### Anti GPS spoofing (social-signal-plan.md Part C)

Every route that acts on a place passes one check first (`httpapi/located.go`): nodes and tethers nearby, a
Tether Node's info, harvest, build, upgrade, collect, upload, attack, defuse, drone, intercept, shield, contests,
drops, posting and taking a job. Each may carry `mock` (the phone marked the fix fake) and `fixAt` (the fix's own
time, RFC 3339) beside `lat`, `lon` and `accuracyM` (`accuracy` on `/nodes/nearby`); GET routes take them as query
parameters. Old builds that send neither still work.

- During a **signal lock** every one of them returns **1790** `SignalLocked` (403, data `until`, `incident`: `first` or
  `repeat`). Sign-in and everything else still work. `/me` returns `signalLockedUntil` and `signalLockLine`.
- A fix older than `FixMaxAgeSeconds` or from the future: **1791** `FixStale`, noted (`stale_fix`), no lock.
- A hard signal (the fake flag with `SpoofCheckMockFlag`, or a jump over `SpoofTeleportKmh` and `SpoofTeleportMinKm`
  after both fixes' accuracy) refuses the request, starts a lock (`SpoofLockMinutes1/2/3`, stepping up within
  `SpoofRepeatDays`; a new incident during a lock adds no time) and files a report from Signal watch
  (reason `spoof`, context `{auto, incidents, signals}`; never a position). `SpoofLockEnabled` 0: report only,
  the request gets 1205. Spoof-exempt players are noted only, never locked.
- Movement patterns (G2, report only): the last `SpoofPatternFixes` accepted fixes are checked for the same position
  and accuracy with no jitter at all, the same accuracy on moving fixes, or a dead-straight line at a constant speed.
  A finding adds a `pattern` signal to the Signal watch report (no lock, not counted toward the step-up), at most once
  in `SpoofPatternGapHours`; `SpoofPatternEnabled` 0 turns it off.
- Mods: dismissing a `spoof` report lifts the lock and takes its incidents out of the step-up count. `/me` gives staff
  `openReports` (players with open reports). Admins: `PUT /admin/players/{code}/spoof-exempt {exempt, reason}`
  (logged); player info shows `spoofExempt`, and the private block's infringements carry `hard`, `dismissed` and
  `evidence` (the two fixes, admins only).

### Friends

| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/friends` | | `incoming`, `friends`, `outgoing` (`FriendRow {player, level, faction, friendName, status}`; `friendName` only for accepted friends) and your `settings {friendName, findableByEmail, acceptFriends}` |
| GET | `/friends/search?q=` | | Text with `@` is an email: only the whole address matches (case aside), only players with `findableByEmail`, at most `EmailSearchesPerDay` a day (1783). Otherwise a whole share code, or the first 3+ letters of a callsign (starts-with, up to 20). At most `FriendSearchPerMinute` searches a minute (1783). Returns `[{player, level, faction, status, accepts}]`; never an email or real name |
| POST | `/friends/{code}` | | Sends a request (notice `friend_request`, pushed). If they had asked you, accepts theirs. Refused (1780, the message says why): yourself, already friends or pending, they don't take requests, `FriendsMax` on either side, `FriendRequestsPerDay`, `FriendRequestsPendingMax` waiting, or they declined you inside `FriendRedeclineDays`. Unanswered requests expire after `FriendRequestExpireDays` |
| POST | `/friends/{code}/accept` | | Accepts their request (notice `friend_accepted` to them). 1781 if there is none |
| POST | `/friends/{code}/decline` | | Declines it; nobody is told, and they can't ask again for `FriendRedeclineDays` |
| PUT | `/me/privacy` | `emailSearchable` | LET PLAYERS FIND ME BY EMAIL (on by default) |
| DELETE | `/friends/{code}` | | Ends a friendship or takes back a request; nobody is told |
| PUT | `/me/friends` | `friendName, findableByEmail, acceptFriends` | The name friends see: typed, optional, letters (any script), spaces, `'.-`, up to 32, blocked words refused (1782). The Google/Apple name is never sent |

`GET /players/{code}` adds `friend` ("", `friend`, `incoming`, `outgoing`), `acceptsFriends`, and `friendName` for a friend.
Branch members (`/factions/branch`) carry `friendState`; `/me` has `emailSearchable` and `friendRequestsWaiting`. Friend
pushes carry `kind` and `actor` (the other player's share code). Banned players drop out of friend lists (a lifted ban
brings them back); suspended players can still be found and added.

### Unbound titles
F2 (phase 5): past the faction level a freelancer who has heard from the seventh voice is given titles, each a badge
on their profile (`badges [{badge, factionCode: "", factionName: "", region: ""}]`) and a message from `LANGIS`:
`drifter` at `TitleDrifterLevel` (22) "Drifter. You've earned it.", `wayfarer` at `TitleWayfarerLevel` (26)
"Wayfarer. They can't place you now.", `unbound` at `TitleUnboundLevel` (30) "Unbound. Like me." Given at the level-up
that reaches them, or when the day's missions are handed out for a level reached earlier. A member earns no more; titles
already earned stay.

### Freelancer standing and missions
- **Neutral standing** (F5): a faction player can't hit a freelancer's Tether Node (1611 with the reason), at any level,
  unless that freelancer attacked a faction Tether Node in the last `ProtectionLossDays` (3). Freelancer against
  freelancer keeps the level band. A freelancer's Tether Node still takes uploads from every faction.
- **Missions** (F3): a freelancer gets `MissionsPerDay` (6), one from each power, at every level; a member gets
  `MemberMissionsPerDay` (1) from their own power, and a Warden or above a command order besides.

### Device record
A pledge and a ban stick to the phone and to the Google/Apple sign-in (freelancer-path-plan.md phase 2): the server
keeps salted one-way hashes in `td.device_marks`, never tied to a player row, so deleting an account undoes neither.

- **Sign-up and sign-in** check the phone's hash (`deviceId`: Android's id, iPhone a Keychain id) and the sign-in's.
  A ban on either refuses entry with 1122, new account or old. A member's pledge is marked on every phone and sign-in
  they use; an account that hasn't pledged, on a marked phone or sign-in, is bound to that faction.
- **Bound accounts:** `/me` carries `pledgeBound` (the faction code), `appealCode` and `appealUrl`; `GET /factions`
  marks that faction `bound`. Pledging anywhere else answers 1306. If that branch has no commander in their country,
  they may claim its seat.
- **A ban** marks every phone and sign-in the player used; **lifting** it clears those marks. **RELEASE PLEDGE**
  clears the faction from the player's marks.
- `appealUrl` is a `mailto:` to support with the appeal code filled in. A player's info for admins lists `marks`.

### Reserved accounts
Player ids 1 to 10 are never given to a new account (migration 0043, `game/reserved.go`):

| Id | Who |
|---|---|
| 1 | The owner, the **primary admin**. A player like any other (counted in stats, plays the game) but always a freelancer: `POST /factions/{code}/lead` and `/join` answer 1305 |
| 2 to 7 | The six powers, one `system` account each (faction id + 1): 2 Signum, 3 Shingō, 4 Ishāra, 5 Sēma, 6 Tohu, 7 Merki. No sign-in; never found by share code, search, report or sanction; not counted |
| 8 to 10 | Held for later |
| 11+ | Everyone else. Moderators and other admins are ordinary accounts |

On a database with no player 1 (after the launch wipe), the account that first signs in with Google or Apple
under `TD_PRIMARY_ADMIN_EMAIL` (verified email, kept in `.env` only) is made id 1 and an admin. Access tokens
issued before the setting `TokensValidAfter` (a unix time, locked; set when ids were moved) are refused with 1001,
and every session from before was revoked, so each phone signs in once more.

**Looking inside a faction (admins).** An admin can read a faction without joining it: they are not a member, are
not listed, and nobody is told. Each look is written to `admin_actions` (`faction.inspect`).
Decided 2026-11-11: a ghost visit sees everything a member sees and can move between the faction's regions, and
no player can ever see the admin there. Today that is the branches, commander and members below. Faction chat
and commanders' orders do not exist yet; when they are built, their endpoints must also answer an admin who is not
a member (read-only), and must never list the admin as present, as a reader or as a member.

| Method | Path | Notes |
|---|---|---|
| GET | `/admin/factions` | `factions`: the six, as `GET /factions` gives them (name, colour, status, commander, members) |
| GET | `/admin/factions/{code}` | `code, name, color, status, members` and `branches [{region, name, commander {code, name}, members [{code, name, level, rank, rankTitle}]}]` |

### Faction perksMembers of a faction get its perks; they stop the moment the player leaves. The server applies them
(`game.FactionPerks`), and every number is a setting (`Perk…Pct`, group `factions`; 0 switches a perk off):

| Faction (power) | Perk | Settings |
|---|---|---|
| Lumen Collective (Signum) | More crystal from each harvest and from what their Tether Nodes gather | `PerkSignumCrystalPct` 25 |
| Forge Meridian (Shingō) | Tether Node upgrades cost less | `PerkShingoUpgradePct` 25 |
| Veil Cipher (Sēma) | Longer scan range; rare roaming nodes seen from further | `PerkSemaScanPct` 100, `PerkSemaRarePct` 100 |
| Verdant Exchange (Ishāra) | Tether Node builds cost less; more credit from missions | `PerkIsharaBuildPct` 20, `PerkIsharaMissionCreditPct` 25 |
| Slipstream Vanguard (Tohu) | Faster drones; more carry capacity | `PerkTohuDroneSpeedPct` 30, `PerkTohuCarryPct` 25 |
| Bastion Order (Merki) | Their Tether Nodes take less damage; intercepts stop more | `PerkMerkiDamagePct` 15, `PerkMerkiInterceptPct` 10 |

`/me` carries `activePerks [{key, line, tag}]`: a member's faction perks, or for a freelancer today's **daily favour**
(freelancer-path-plan.md F4: each power whose freelancer mission they finished today gives `FavourPerkPct`, 40%, of its
perk until the mission day ends; tags like `+10% SIGNUM`), and `favourOpen` for a freelancer; its `carryCapacity`, `scanRangeM`,
`tetherCost` and `tetherUpgradeCost` already include them. `key` is one of `crystal`, `upgrade_cost`, `scan`,
`rare`, `build_cost`, `mission_credit`, `drone_speed`, `carry`, `damage`, `intercept`. A harvest and a mission
claim answer with `perkTag` when a perk raised the amount. A profile's `faction.perks` lists the same lines,
so a perk is visible on anyone in that faction.

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
| GET | `/tethers/nearby` | `lat, lon, radius?` | Active TNs nearest first, each with `owner {code, name}` (a PlayerRef, see Players), within the scanner's range (or the Drone Bay's, whichever is larger), with their links (`nodeId, resource, lat, lon`), `shieldTier` (0 = none), `factionColor` (the owner's faction colour `#rrggbb`, left out for a freelancer; looks only, for living TNs in AR) and, on your own, `drones`: each drone flying at it `{id, mk, launchedAt, arrivesAt, from {lat, lon}, intercepted}` (where it came from, not who sent it) |
| GET | `/tethers/mine` | – | Every TN the player owns, wherever it is (no range check), plus ones that ended in the last `TnEndedShowMinutes` (60). Most urgent first; see My Tether Nodes below |
| POST | `/tethers` | `lat, lon, accuracyM` | Builds a tier 1 TN at that fix. Checks GPS, the per-player cap and the cost (carried first, then stored), and links free nodes in range. Returns `tether, carried, stored, xp, progress, levelUp` |
| GET | `/tethers/{id}` | `lat, lon, accuracyM?` | One TN. `links` (name, resource, amount gathered) is only filled within `TnInfoRangeM`; `inRange` says which |
| POST | `/tethers/{id}/upgrade` | `lat, lon, accuracyM` | Owner only, in range. To tier 2, linking more free nodes. Refused if none is free. Returns `tether, carried, stored, xp, progress, levelUp` |
| POST | `/tethers/{id}/dismantle` | – | Owner only. Refunds part of what was spent, plus everything it held, to stored, and frees its nodes. Returns `refund, held, stored` |
| POST | `/tethers/{id}/collect` | `lat, lon, accuracyM` | Owner only, in range. Moves the held half (`stock` in nearby, owner only) to stored, and brings the TN back to full gathering speed (see Slowdown below). Returns `moved, stored` |
| POST | `/tethers/{id}/attack` | `weapon, lat, lon, accuracyM` | `em_pulse`, `signal_jammer` or `overload_spike`. Checks GPS, `WeaponRangeM` and the attack rules; uses the item and its energy cells. A shield cuts the damage (and jam time) by `ShieldShare` (weapons count as Mk I), and no single hit takes a shielded node below 1 health. Returns `health`, `damage` (EM Pulse), `destroyed`, `jammedUntil` (jammer only), `spikeFiresAt` (spike only), and `xp, progress, levelUp` (see Fighting XP below) |
| POST | `/tethers/{id}/drone` | `lat, lon, accuracyM` | Needs a working Drone Bay; range is the bay's `stats.range_m` (Mk I 150 m, Mk II 500 m, Mk III 1 km, Mk IV 5 km) plus the fix's accuracy (at most doubling it), else 1712 with `distanceM, rangeM`. Attack rules apply. Uses the bay's `stats.cells` energy cells (Mk IV 3) and `DroneLaunchWear` (5) durability; at most `stats.drones` in flight. Flies at `DroneSpeedMps`, at least `DroneMinFlightSec`. Returns `id, tnId, mk, launchedAt, arrivesAt, distanceM, from {lat, lon}, to {lat, lon}, damage` (what lands without an intercept, after the shield) |
| POST | `/tethers/{id}/intercept` | `lat, lon, accuracyM` | Owner only, within `WeaponRangeM`, `CellsIntercept` energy cells. Intercepts the soonest-landing drone not yet intercepted; it stops `InterceptShareMk1..4` of its damage by the drone's Mk (Mk I 100%: the drone is void at once). 1732 when no drone is inbound, 1733 when all are already intercepted. Returns `droneId, share, shotDown, xp, progress, levelUp` |
| POST | `/tethers/{id}/shield` | `item, lat, lon, accuracyM` | Owner only, within `WeaponRangeM`. Fits a crafted `tn_shield_mk1`..`mk4` (uses the item); lasts as long as the node; 1734 unless it is above the fitted one. Returns `shieldTier` |
| GET | `/drones` | – | The player's own strikes in flight and those ended in the last `DroneResultsMinutes` (10): `id, tnId, mk, status (flying, hit, void), launchedAt, arrivesAt, from, to, intercepted, damage` (`damage` once landed) |
| POST | `/tethers/{id}/defuse` | `lat, lon, accuracyM` | Owner only, in range, before the fuse ends |
| POST | `/tethers/{id}/upload` | `lat, lon, accuracyM` | In range, faction rules apply. Moves everything carried to the player's stored inventory, less the charge paid to the owner. Returns `moved` (what went into storage this time), `fee`, `carried`, `stored` (new totals) |

**Fighting XP.** A hit on another player's Tether Node pays the attacker `XpAttack` (25), plus
`XpDestroy` (50) when it destroys the node: an EM Pulse or jammer when used (in the attack's
response), a spike when it fires and a drone when it lands (in the tick; the next `/me` shows it). A
drone shot down pays nothing. Intercepting a drone pays the defender `XpIntercept` (20). Only
`XpAttackPerTargetDay` (3) attacks pay per attacker and target player each day (midnight at
`MissionDayOffsetMinutes`, Maldives time), and the same number of intercepts per defender and
attacker; after that they still work but `xp` is 0 and `progress` is left out.

### Tether Node slowdown
A Tether Node gathers at full speed for `TnFullSpeedHours` (2) after it is built or after its owner last
collected at it (`POST /tethers/{id}/collect`), then `TnSlowPctPerHour` (10) percent slower for each further hour,
never below `TnSlowFloorPct` (20) percent of full speed. Slower means each link waits longer before its next
harvest: `HarvestCooldownSec` divided by the speed (5 minutes at full speed, 25 minutes at 20%). A collect also
cuts every link's wait back to the usual cooldown. All three are public settings on the admin settings screen
(group `tether`); `TnSlowPctPerHour` 0 switches the slowdown off. The owner's own Tether Nodes carry `speedPct`
(100 = full speed) in `/tethers/nearby`, `/tethers/{id}`, the build and upgrade answers and `/tethers/mine`;
nobody else is told.

### My Tether Nodes
`GET /tethers/mine` returns an array of:

```
{ "id": 12, "name": "Crystal Seam K-27", "tier": 1, "lat": 4.17, "lon": 73.50,
  "health": 72.5, "maxHealth": 100, "expiresAt": "...", "status": "active", "endedAt": null,
  "stock": {"crystal": 14},
  "links": [{"nodeId": 301, "resource": "crystal", "name": "Crystal Seam K-27"}],
  "jammedUntil": null, "spikeFiresAt": null, "droneArrivesAt": null, "attackedAt": "...",
  "underAttack": true, "shieldTier": 2, "speedPct": 60,
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
{ "id": 88, "kind": "attacked", "severity": "danger", "tnId": 12,
  "actor": { "code": "K7QX3M9P", "name": "ECHO-7" }, "amount": 25,
  "body": "ECHO-7's EM Pulse hit your Tether Node near Crystal Seam K-27 for 25 damage.",
  "createdAt": "..." }
```

`push` is true for the kinds sent to the phone as notifications (the app lists those in ALERTS). `severity` (`info`, `warning`, `danger`) picks the sound and haptic; unknown future kinds are
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
| `invitation` | info | freelancer | At `LeadFactionLevel`: "You were invited. The choice is yours. Open FACTIONS." Plain game text, no power speaking; the inviting faction is `invited` in `GET /factions` | null | null |
| `call` | info | freelancer | "Incoming call: <power>": a power's call the app did not fetch within `CallPushDelaySeconds`. Always pushed, whatever the two push switches say | null | null |
| `warden` | info | member | Their Regional Commander made them a Warden | commander | null |
| `defused` | info | planter | The owner defused your spike | TN owner | null |

Contest notices have `tnId` null.

## Map drops
| Method | Path | Body / query | Notes |
|---|---|---|---|
| GET | `/drops/nearby` | `lat, lon` | What ended Tether Nodes left on the map, within the scanner's range, each with `expiresAt`: a drop lasts `DropLifetimeHours` (48, the same as a Tether Node's life), then the Undersignal reclaims it and the Tether Node's owner gets a `drop_reclaimed` notice (info, in the game only) |
| POST | `/drops/{id}/pick` | `lat, lon, accuracyM` | In range. Moves what fits into carried; the rest stays on the map |

## Signal echoes (no-ar-support-plan.md milestone 7)
Faint traces in the Undersignal, seen and read only in the AR view. Looks and story only: no resources, no XP, no combat information.
| Method | Path | Body / query | Notes |
|---|---|---|---|
| GET | `/echoes/nearby` | `lat, lon` | Echoes within `EchoViewRadiusM` (150), nearest first: `{id, lat, lon, distanceM, known}` (`known`: the line is already in your CODEX). The line itself is not sent. The cells around are filled first: `EchoesPerCell` (3) per `EchoCellM` (400 m) cell, each placed `EchoOffsetMinM`..`EchoOffsetMaxM` (6–18 m) beside an active node or Tether Node, carrying a random line; after about `EchoMoveHours` (24, spread ±25%) it moves (removed, and the cell gets a new one) |
| POST | `/echoes/{id}/read` | `lat, lon, accuracyM, arView` | `arView` must be true (1615). Within `HarvestRangeM` (40 m) with the node accuracy and travel checks (1614); a moved echo is 1613. Returns `{set, line, text, new, found, total, badge?}`: the line goes into the CODEX; a finished set gives the badge `codex_set_<n>` (a title with no faction) |

`/me` has `codex`: every set `{set, found, total, complete, lines: [{line, text?}]}`; a locked line has no `text`.
The lines live in `game.EchoSets`; only lines the owner approved go there (first set approved 2026-11-11).

## Walk together (docs/walk-together-plan.md)
Two to four accepted friends standing together form a signal link (any faction mix). Every call carries a fix
(`lat, lon, accuracyM`, and `mock`, `fixAt` or the G1 headers) and goes through the spoof checks. Settings group
`walk` ("Walk together").
| Method | Path | Body | Notes |
|---|---|---|---|
| POST | `/walk/open` | fix | "I have WALK TOGETHER open." Returns `{nearby, asks, link}`: `nearby` = friends with theirs open (within `WalkLinkOpenSec`), within `WalkLinkFormM` after accuracy, on another phone, not linked (PlayerRefs only, never a distance); `asks` = who asked you (within `WalkLinkAskSec`); `link` your link or null |
| POST | `/walk/ask` | `code` + fix | Ask a friend: 1811 not friends, 1812 not within reach or panel closed, 1813 your group is full, 1814 either side signal-locked, 1815 same phone (L6), 1816 your fix too weak, 1818 they are in a link, 1810 off |
| POST | `/walk/ask/{code}/accept` | fix | Join the asker's group (or form one) after the proximity check; 1817 when the ask expired. Returns the panel |
| POST | `/walk/heartbeat` | fix | Every `heartbeatSec` while linked. Returns the panel; `link {size, members [{player, me, lat?, lon?, outOfRange}], bonusActive, pausedReason (keep_walking, cap_spent, out_of_range, weak_fix), bonus {scanPct, yieldPct, xpPct}, heartbeatSec, bonusSecondsLeft, walkedM}`; positions are the other members' only. `ended` (and `link` null) when this beat ended your place: `fell_behind`, `alone`, `disabled`. 1819 when not linked |
| DELETE | `/walk` | – | Leave (also on going to the background); a link left with one player ends |

Proximity, on forming and every beat: fix taken within `WalkLinkFixAgeSec` (20 s), accuracy ≤ `WalkLinkMaxAccuracyM` (25),
not fake (spoof-exempt testers skip this); distance less both accuracies (together at most `WalkLinkMaxAccuracyM`) ≤
`WalkLinkFormM` (30) to form, ≤ `WalkLinkHoldM` (60) to at least one other member to hold; the two fixes within 20 s of each
other; different devices. Out of range or silent for `WalkLinkGraceSec` (45) leaves the link. The bonus pauses until the
player has moved `WalkLinkMoveM` (50) within `WalkLinkMoveWindowSec` (180), and stops for the day after
`WalkLinkDailyMinutes` (120) of bonus time. While on: scan range, node harvests and map drop pickups (`linkBonus`
on both outcomes, the percent; harvests also `linkXpBonus`) and harvest XP get `WalkLinkScanPct/YieldPct/XpPct` 2/3/4
(25/40/50, 15/25/30, 10/20/30), added to any faction perk, never multiplied. Not on Tether Node collects, missions,
drones, damage or shields. A signal lock or fake flag on any member (L1, L2) ends the link for everyone; an attack on a
linked member's Tether Node ends it (`attack`). `/me` has `walkLink` (as above, or null) and `walkedTogetherM`.

## Orders and daily missions
| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/missions` | – | Today's missions (the Maldives date; `MissionsPerDay` 6, refreshing at local midnight, `MissionDayOffsetMinutes` 300), given on first ask: freelancers get one from each of the six lore powers, at any level; after joining, every mission comes from the player's own faction's power, and Regional Commanders and Wardens get one command order among theirs, also from their own power. Returns `day, refreshesAt, missions`, and `invitation`, which is retired and always null (see `invited` in `GET /factions`) |
| POST | `/missions/{id}/claim` | – | Pays a done mission: XP, credits into storage, its item if any. Returns `reward, xp, progress, levelUp` |

Each mission: `{id, persona, factionCode, color, audience, goal, goalKey, count, progress, status (active, done, claimed),
reward {xp, credits, item, itemCount}, line, title}`. `persona` is the lore power that gave it (Signum, Shingō,
Sēma, Ishāra, Tohu, Merki); `line` is its message; `title` says what to do ("Harvest crystal 3 times"; harvest missions count harvests, not amounts). Progress counts
on the server inside each action (harvest, build, upgrade and collect at a Tether Node, upload, craft, gear upgrade and
repair, attack, drone, intercept, shield, appoint a Warden). The same actions feed the player's playstyle score (below).
Rewards grow with level (`MissionXpBase`, `MissionXpPerLevel`, `MissionCreditBase`, `MissionCreditPerLevel`),
`MissionItemChance` of a low-level item, command orders `MissionCommandMult` times.

## Factions
| Method | Path | Notes |
|---|---|---|
| GET | `/factions` | All factions in the player's region (a country; `MV` at launch): `status` (`locked` until any region is claimed), `leader {code, name}` (the Regional Commander here, null when nobody holds it), `perks [{key, line, tag}]` (what members get, with the numbers: `line` for the FACTIONS screen, `tag` for beside a number it changed, e.g. `+25% SIGNUM`), `mine` (the asking player's own faction: the perks are working on them now), `members` (here), `region`, and `invited`: true on the one faction that invited this freelancer at `LeadFactionLevel` (never for a player already in a faction) |
| POST | `/factions/{code}/lead` | **Pledge** to a faction with no Regional Commander in your region, claiming its seat (one per faction per country; first to claim wins). Rank 90. Returns the membership with `powerName` and `powerLine` (the chosen power and its motto, shown once). A pledge is final: there is no leave (freelancer-path-plan.md) |
| GET | `/factions/branch` | Your branch: `factionCode, factionName, region, members [{code, name, level, rank, rankTitle}], canAppoint` (Regional Commander). 1763 when you're in no branch |
| POST | `/factions/branch/wardens` | `code` (the member's share code; 1003 if unknown): the Regional Commander makes a member of their branch a Warden (rank 50). 1762 if you aren't, 1763 if they aren't in your branch, 1764 if already Warden or above |
| POST | `/factions/{code}/join` | **Pledge** to the faction's branch in your region, once it has a Regional Commander. Rank 1. Returns the membership with `powerName` and `powerLine`. Final, like `lead`. Players never see the word "join": the app says PLEDGE |

### The powers' calls and messages

One power speaks to a player: the one that matches what they have mostly done (`docs/lore-correction-plan.md`,
"Who calls"). The server keeps a **playstyle score** per power, added to inside each action's transaction: harvests
(crystal most) for Signum; building and upgrading Tether Nodes, crafting, upgrading and repairing gear for Shingō;
data harvests and jammers and spikes (crafted or used) for Sēma; uploads, collects and credit harvests for Ishāra;
drones, EM pulses and ground covered between harvests (1 point per 100 m, at most 5) for Tohu; shields, intercepts and
defused spikes for Merki. The highest score wins; a tie goes to the power whose daily missions the player finished
most. The same power makes the level 19 offer and is the faction marked `invited` at `LeadFactionLevel`.

| Method | Path | Notes |
|---|---|---|
| GET | `/calls` | `calls`: the calls that have come due and were neither played nor rejected, oldest first (they ring); `callLog`: every call the player answered or rejected, latest first (30 at most), with `rejected`, `played` and `canPlay` (a rejected call plays once, an answered call can be heard once more): ALERTS lists it; `missedCalls`: the rejected calls not yet played (`missed: true`), kept for older builds; `messages`: the written messages that have arrived, newest first; `unread`. Fetching a call delivers it, so no push follows |
| POST | `/calls/{id}/played` | The call was heard (answered, a rejected call played, or an answered call heard again): it never rings again; a rejected call is spent after one play, an answered call after one replay. 1765 if it has no play left |
| POST | `/calls/{id}/reject` | REJECT on a ringing call (or it rang out in the app): it stops ringing and moves to `missedCalls`. The power's repeat call and message still come. 1765 if it isn't ringing |
| POST | `/messages/{id}/read` | Clears a message's unread mark; messages stay. 1766 |

Each call: `{id, kind (level10, level19, level19_repeat), persona, factionCode, color, line, dueAt, voice, audio,
missed}`. `voice` and `audio` name the app's sound file (`Audio/Voices/<voice>/<audio>`); `line` is the same words, for
the screen. `missed` is true once the call is older than `CallRingSeconds` (60): the app then offers to play it once
instead of ringing. Each message: `{id, persona, factionCode, color, body, at, read}`.

Freelancers only. **Level 10:** when an XP gain carries the player past half of `CallPathLevel` (10), one call about
the path they are on; no repeat. **Level 19:** past half of the level before `LeadFactionLevel`, the offer (a call),
a repeat call `CallRepeatHours` (24) later and a written message `CallMessageHours` (48) later. Reaching
`LeadFactionLevel` first drops whatever has not come due. A call the app has not fetched `CallPushDelaySeconds` (45; the app polls every 20 s)
after it came due is pushed as "Incoming call: <power>" through the notices outbox. No power says its own name, and
nothing here links the six.

Memberships (here and `faction` on `/me`) carry `factionId, factionCode, factionName, rank, branchId,
region, rankTitle`. `/me` also lists `badges`: `[{badge, factionCode, factionName, region}]`; `founder`
goes to the first player ever to command a faction's branch in a country, and is permanent. Ranks: 1 Member, 50 Warden, 90 Regional Commander; 100 is the shared lore AI
above every faction (internal name `signal`; players see a separate commander per faction), never a player.

## Admin (admin accounts only)
| Method | Path | Body | Notes |
|---|---|---|---|
| GET | `/admin/settings` | – | The settings screen: `groups [{key, title}]` in display order, `settings [{key, label, description, group, value, min, max, step, locked, isPublic, updatedAt}]` (`step` 1 = whole numbers; `locked` ones are shown and never changed), `changes` (the last 30: `{key, label, old, new, by, at}`, `by` is the admin's callsign) and `announcement {text, kind, endsAt}` or null |
| PUT | `/admin/settings/{key}` | `value` | Changes one setting inside its `min` to `max` (whole numbers when `step` is 1). 1007 when locked, 1008 outside the range (`min, max, step`), 1003 for an unknown key. The old and new values are logged in `admin_actions`; live within a minute. Returns the setting |
| PUT | `/admin/announcement` | `text, kind, hours` | Posts or replaces the one-line banner on everyone's main menu: up to 140 characters, `kind` `info`, `event` or `warning`, shown for 1 to 720 hours (1009 otherwise). Empty `text` removes it. Players get it as `announcement` in `/me` while it runs. Country-neutral wording is the admin's job. Returns `{announcement}` |
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
| 1007 | Setting is locked | |
| 1008 | Setting value outside its range, or a fraction where whole numbers are wanted | `min, max, step` |
| 1009 | Announcement text, kind or hours not allowed | |
| 1100 | Google sign-in could not be verified | |
| 1101 | Session expired: sign in again | |
| 1102 | Account suspended | |
| 1103 | Account deleted | |
| 1104 | Device id missing | |
| 1105 | Apple sign-in could not be verified | |
| 1106 | That Google or Apple sign-in already opens another game account (no merging) | |
| 1107 | Can't remove the account's only sign-in | |
| 1108 | A different account of that provider is already linked: unlink it first | |
| 1109 | Callsign has the wrong shape | `error: "callsign_invalid"` |
| 1110 | Callsign is taken, reserved or blocked | `error: "callsign_taken"` |
| 1111 | Callsign was changed too recently | `error: "callsign_too_soon", changeAt, days` |
| 1112 | Account suspended (sign-in and refresh) | `until` |
| 1113 | Can't report yourself | |
| 1114 | Report detail or note not allowed | |
| 1115 | Already an open report on that player | |
| 1116 | Daily report limit reached | `perDay` |
| 1117 | Sanction kind, days or reason not allowed | `days` |
| 1118 | Not allowed to act on that account | |
| 1119 | Sanction is not running | |
| 1120 | Report is not dismissed | |
| 1121 | Admin change to a player not allowed (the message says why) | |
| 1122 | This device is banned (the device record: the phone or the sign-in carries a ban). Sign-up and sign-in | `appealCode, appealUrl` |
| 1305 | The primary admin (player 1) stays a freelancer: can't lead or join a faction | |
| 1770 | That job can't be posted (the message says which rule) | |
| 1771 | Over a JOBS board limit | |
| 1772 | That job has no slot left, or it ended | |
| 1773 | You can't take that job (the message says why) | |
| 1774 | Not enough in storage to pay every slot | `short` |
| 1780 | Friend request refused (message says why) | |
| 1781 | No friend request from that player | |
| 1782 | The name for friends can't be used | |
| 1783 | Too many email searches today | |
| 1790 | Signal lock: location actions refused | `until`, `incident` |
| 1791 | The fix is too old or from the future | |
| 1795 | No region near where the player stands | |
| 1796 | The region where the player stands isn't live yet | `region` |
| 1797 | That region has no Regional Commander yet (can't be viewed) | |
| 1798 | Only faction members have regions | |
| 1306 | Your pledge stands: a pledge made on this phone or sign-in binds the account to one faction | `faction, appealCode, appealUrl` |
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
| 1613 | The echo has faded (moved) | |
| 1614 | Move closer to the echo | `distanceM, rangeM` |
| 1615 | Echoes are read in the AR view | |
| 1810 | Walk together is off (`walk_disabled`) | |
| 1811 | Only with a friend (`walk_not_friends`) | |
| 1812 | Stand closer together (`walk_too_far`) | |
| 1813 | That group is full (`walk_group_full`) | |
| 1814 | A signal is locked (`walk_locked`) | |
| 1815 | Two accounts on one phone (`walk_same_device`) | |
| 1816 | Position not accurate enough to link (`walk_weak_fix`) | |
| 1817 | The ask expired (`walk_ask_gone`) | |
| 1818 | Already walking with a group (`walk_busy`) | |
| 1819 | Not walking with anyone (`walk_not_linked`) | |
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
| 1765 | That call has ended (not yours, not due, or already played) | |
| 1766 | That message is gone (not yours, or not arrived) | |
| 1750 | Seeding map data not imported (admin seed calls; run `scripts/import-osm.ps1`) | |
| 1900 | Server error | |

Codes are never reused. Logs write them as `E1200 ...` with the request id and player id.
