# Friends, region visiting and anti GPS spoofing: plan

Status: **approved 2026-11-11** (owner). Three plans written in separate threads on 2026-11-11 and combined here so
they build as one run. Builds **after the freelancer build** (`docs/freelancer-path-plan.md`), and friends needs the
profile plan's P0 and P1 (callsigns, `PlayerRef`, profile screen). The owner's local Claude Code builds it; the
prompt is `docs/social-signal-build-prompt.md`. GAME_DESIGN.md in the mobile repo has a short section for each.

Migration numbers below (e.g. `0052`) are examples: use the next free number at build time.

## Build order

| # | Phase | Part | Where |
|---|---|---|---|
| 1 | **G0** Teleport and stale-fix checks, signal lock, system reports, anti-cheat settings, exempt flag | C | API |
| 2 | **G1** Fake-location flag (Android + iOS), lock screen, narrator lines | C | Mobile (+ API fields) |
| 3 | **FR1** Friends | A | API + mobile |
| 4 | **R1** Region from GPS, live regions, admin test regions | B | API |
| 5 | **R2** View region (talk only) | B | API |
| 6 | **R3** Region switch in the app | B | Mobile |
| 7 | **G2** Movement pattern checks (report only) | C | API |
| later | **G3** Play Integrity and App Attest (needs the owner's Google Cloud and Apple setup) | C | API + mobile |

Anti-spoofing goes first because it protects everything else and needs no other plan.

## Part A. Friends

### Goal (owner, 2026-11-11)

Players can find each other by **email**, by **operator name** (callsign), or from their **faction's
roster** (same faction only), and send a friend request. When the other player accepts, both see
each other in a **friends list, with real names shown to each other**.

### How it fits the profile plan

- The profile plan says the real name from Google or Apple is **never public**. Friends is the one
  exception, and only between two players who both agreed: sending a request and accepting it is
  the consent. Everyone else still sees only the callsign.
- Every friend row and search result is a `PlayerRef {code, name}`, so it opens the profile with the
  same tap rule as everywhere else.
- The profile screen (P1) gets the friend button. The "Not in this plan" line in profile-plan.md
  (friends lists) is covered by this plan.

### Rules

| Rule | Detail |
|---|---|
| Who can be added | Any active player, freelancer or faction member, any faction. Not yourself, not deleted or banned players. Suspended players can be added and stay listed. |
| Find by email | **Exact, full email only** (case-insensitive), the email of the player's Google or Apple sign-in. No partial matches, so nobody can browse emails. The result shows only the callsign card, never the email. |
| Find by operator name | Callsign, starts-with match from 3 letters, up to 20 results. A share code (8 characters) also works. |
| Find from the faction roster | Faction members see their branch roster (the existing branch member list) with an `ADD` button on each row. Freelancers have no roster, so they use email or name. |
| Requests | Pending until accepted, declined or cancelled. Decline is silent (the sender just sees it stay pending, then it expires after 14 days). After a decline, the same sender can't ask again for 7 days. If both send a request to each other, they become friends at once. |
| Limits (settings) | `FriendsMax` 200, `FriendRequestsPerDay` 20, `FriendRequestsPendingMax` 50, `FriendSearchPerMinute` 20, `FriendRequestExpireDays` 14, `FriendRedeclineDays` 7. |
| Real name | Shown only on an accepted friendship, both ways, from `td.players.display_name`. If Apple gave no name (the player hid it), it shows `NAME NOT SHARED`. |
| Removing | Either player can remove the friend at once, with no notice to the other. The real names stop showing on both sides at once. |
| Deleted or banned | Removed from every friends list (the friendship rows are deleted with the account; a ban hides them, and a lifted ban brings them back). |
| What friends see | Callsign, real name, level, standing line, and the profile. **No** email, location, Tether Node positions, last-seen time or online status. |
| Moderation | Mods and the mod queue still never see real names (unchanged). Friends don't affect reports. |
| Region switching | Friends lists are global and don't change when a player switches region inside their faction. |

**Email search can be turned off:** a `LET PLAYERS FIND ME BY EMAIL` switch on your own profile,
**on by default**. Off means email search never finds you; name and roster still do.

### Where it shows in the game

- **Main menu:** a `FRIENDS` row, with a small count when requests are waiting.
- **Friends screen**, three tabs:
  - `FRIENDS`: rows of hexagon initials, callsign, real name below in smaller text, level, faction
    colour. Tap opens the profile. Long-press (or a `...` button): `REMOVE FRIEND`.
  - `REQUESTS`: incoming (with `ACCEPT` and `DECLINE`) and sent (with `CANCEL`). The accept box says:
    `Accepting shows your real names to each other.`
  - `FIND`: one search box (`Operator name or email`), results with `ADD`. Below it, for faction
    members, `FROM YOUR FACTION`: the branch roster with `ADD`, `REQUESTED` or `FRIENDS` on each row.
- **Profile screen** (another player's): one button by friend state: `ADD FRIEND`, `REQUESTED`,
  `ACCEPT REQUEST`, or `FRIENDS` (tap to remove). On a friend's profile, a line only you see:
  `REAL NAME: <name>`.
- **Alerts and push:** `<CALLSIGN> sent you a friend request.` and `<CALLSIGN> accepted your friend
  request.` Both have a `PROFILE` action. Push for both (setting to turn off, like the other pushes).
  Declines and removals send nothing.

### API

**Data (migration, next free number, e.g. `00xx_friends.sql`)**

- `td.friendships`: `player_lo`, `player_hi` (the two ids, smaller first, primary key on the pair),
  `requested_by`, `status` (`pending` / `accepted`), `created_at`, `accepted_at`.
- `td.friend_declines`: `from_player`, `to_player`, `declined_at` (for the 7-day rule).
- `td.players.email_searchable boolean NOT NULL DEFAULT true`, and a `lower(email)` index if there
  isn't one.
- Settings above, in the `profile` group of the A1 settings screen.
- A job drops pending requests older than `FriendRequestExpireDays`.

**Endpoints** (all signed in)

| Endpoint | What it does |
|---|---|
| `GET /v1/players/search?q=` | If `q` has `@`: exact email match (only `email_searchable` players). If it is a share code: that player. Otherwise callsign starts-with. Returns `PlayerRef` + level + standing + `friendState`. Rate limited. Never returns email. |
| `GET /v1/friends` | `{friends: [...], incoming: [...], outgoing: [...]}`. Friends carry `realName`. |
| `POST /v1/friends/requests {code}` | Send. Errors: `friend_self`, `friend_exists`, `friend_limit`, `friend_request_limit`, `friend_too_soon`, `player_not_found`. Auto-accepts if the other already asked. |
| `POST /v1/friends/requests/{code}/accept` / `/decline` | Answer an incoming request. |
| `DELETE /v1/friends/requests/{code}` | Cancel a sent request. |
| `DELETE /v1/friends/{code}` | Remove a friend. |
| `PUT /v1/me/privacy {emailSearchable}` | The email search switch. |

**Changes to existing responses**

- `GET /v1/players/{code}` adds `friendState` (`none` / `outgoing` / `incoming` / `friends`) and
  `realName` only when `friends`.
- Branch member list (faction roster) rows add `friendState`.
- `/v1/me` adds `emailSearchable` and `friendRequestsWaiting`.
- New notice kinds `friend_request`, `friend_accepted` (with `actor` as `PlayerRef`), pushed through
  the existing notices outbox.

### Mobile (Unity)

- `FriendsController` + `UiBuilder.Friends`: the three-tab screen in the menu style, opened from the
  `FRIENDS` main menu row and from your own profile.
- Profile screen: the friend button and the private `REAL NAME` line.
- Branch roster: the `ADD` / `REQUESTED` / `FRIENDS` button beside the existing `APPOINT`.
- Own profile: the email search switch.
- Alerts: the two notice kinds with the `PROFILE` action; tapping the push opens `REQUESTS` or the
  friend's profile.

### Privacy pages

The privacy policy and delete-account page (API repo `web/` / `docs/legal/`) add: "Players you
accept as friends can see the name on your Google or Apple account. Other players can find you by
your exact email address unless you turn this off. Your friends list is deleted with your account."

### Phase

**FR1 Friends** (API + mobile), one phase. Needs profile P0 and P1. Step 3 of the build order above.

### Tests

- API: email search exact only and respects the switch, never returns email; callsign search limits;
  self, duplicate, limit, rate and 7-day rules; mutual requests auto-accept; `realName` only on
  accepted friendships, both ways, and gone at once after removal; deleted and banned players drop
  out and a lifted ban brings friendships back; expiry job.
- Mobile: HUD captures of the three tabs, the profile button states and the accept box; the roster
  `ADD` button; push routing.
- On the phones: your account (id 2) and Shinan (id 4) find each other by email, by callsign and
  from the roster once in the same faction, request, accept, see real names, remove.

## Part B. Switching regions within your faction (talk only)

### The ask

Players can switch between regions **within their faction**, but only to regions that are already
**unlocked by someone in that region**, never before. The point is **social only** (owner,
2026-11-11): to talk to that region's players. Players are told they **can't do jobs** or other faction
work there. It is not a transfer of rank, seat or rights.

### How it fits what is decided

- A **region is a whole country**, and each faction has one **branch** per country with one regional
  Commander seat (GAME_DESIGN.md, Factions; `docs/regional-command-plan.md`).
- A branch is **unlocked** once a player standing in that country claims its Commander seat. Until then
  it is "dormant here: awaiting a commander". If everyone leaves, it goes dormant again (succession rule).
- **The pledge is final.** Switching region never changes faction.
- Today the server has one hard-coded region (`game.LaunchRegion = "MV"`), every member's
  `faction_members.branch_id` points at the Maldives branch, and the player's region is
  `players.country_code`. Nothing reads the GPS country yet.

### Two ideas: home region and viewed region

- **Home region** = your branch (`faction_members.branch_id`), as today. It holds your rank, your
  Commander, your Commander seat if you have one, your vote, your jobs and orders, and where your build
  rights apply. **Switching never changes it.**
- **Viewed region** = the region your FACTIONS screen shows and whose chat you are in. Switching
  changes only this. Default is your home region.

### Rules

1. **Who can switch:** any faction member. Freelancers have no branch.
2. **Where to:** only a region of **your own faction** that is **unlocked** (has a regional Commander,
   acting Commander counts). Dormant or never-claimed regions are not offered at all, so nobody sees
   "no commander yet".
3. **Who unlocks:** only a **freelancer at `LeadFactionLevel` standing in that country** who claims the
   seat (today's claim rule, unchanged). A member visiting by switching can't claim it.
4. **From anywhere** (owner, 2026-11-11): no GPS check on a switch.
5. **No cost, no cooldown:** switch back and forth freely; one tap returns you home.
6. **Talk only:** in a viewed region you see its member list and Commander, and read and post in its
   faction **chat** (when chat is built). You show as a **visitor from another region** (a small tag on
   your name), unlike the admin ghost visit, which stays invisible.
7. **No jobs or faction work there:** you can't post or see that region's jobs, get or act on its
   orders, vote, appoint, or use any rank there. All of that stays in your home region. The app says
   so on the switch list and as a line while visiting: "Visiting. You can talk here; jobs and orders
   stay in your region."
8. **Moving home for real** (someone who moves country): **Open**, not part of this. For now, admin
   MOVE REGION.

### Region from GPS (needed before a second country exists)

- Add `td.countries` (ISO code, name, simplified boundary polygon, from Natural Earth admin-0,
  public domain) and a lookup `CountryAt(lat, lng)`.
- `POST /v1/factions/claim` and `/join` use the **country you are standing in** instead of
  `game.LaunchRegion`. `players.country_code` becomes "home country" for display only.
- **Live regions** setting (`LiveRegions`, starts `MV`): a country not in the list can't be claimed,
  so nobody unlocks a branch in a country that hasn't launched.
- **Admin test regions** (dev only): an admin draws a test region on the map (e.g. Hulhumalé) with a
  reserved code (`XA`, `XB`), so the whole flow can be tested in Malé without travelling. Hidden from
  players on production.

### Server (API)

- Migration `0052_region_view.sql`: `td.countries`, `td.faction_members.viewing_branch_id` (null =
  home), setting `LiveRegions`.
- `GET /v1/factions/regions`: for a member, the faction's **unlocked** regions only: name, Commander
  (callsign), member count, which is home, which is being viewed.
- `POST /v1/factions/view-region {region}` (or `home`): checks rules 1 and 2, sets
  `viewing_branch_id`. Errors: `region_locked`, `not_member`.
- Member list and chat use the viewed region. Jobs, orders, missions, votes and officer actions always
  use the home branch, whatever is being viewed.
- Region member lists include current visitors in a VISITING section.
- Admin: PLAYERS shows home and viewed region; admin MOVE REGION (changes home, logged) for players who
  really moved. The admin ghost visit already moves between regions and stays invisible.

### App (mobile)

- FACTIONS > your faction: a **REGION** row ("Your region" or "Visiting") with SWITCH REGION, shown only
  when your faction has more than one unlocked region.
- Switch list: unlocked regions with Commander and member count, and the line "Talk only. Jobs and
  orders stay in your region." Pick one, no confirm needed. A BACK HOME button while visiting.
- While visiting, job and order buttons are hidden for that region.
- Region names come from the server as data. App copy stays country-neutral ("your region", "this
  region"), so it works in every launch country.

### Build order

1. **R1 GPS region:** `td.countries`, `CountryAt`, `LiveRegions`, claim/join use the GPS country,
   admin test regions. No visible change in the Maldives.
2. **R2 View region:** migration, endpoints, visitor tag, everything but talk pinned to home, tests
   (unlocked only, visitor can't post jobs, act or vote, freelancer can't switch).
3. **R3 App:** REGION row, switch list, BACK HOME, visitor tag, talk-only line.
4. Test with two admin test regions on dev, then the owner's phone.

### Open

- Rule 3 (only a freelancer standing there can unlock): my default reading.
- Rule 8 (a real home move): later.
- Chat itself isn't built yet; until it is, a switch shows only the member list and Commander.

## Part C. Anti GPS spoofing

### What already exists (checked in the API repo, 2026-11-11)

- `checkTravel` (`internal/game/harvest.go`) refuses a harvest, tether build or attack whose fix could not have been
  reached from the last one (`MaxTravelSpeedKmh`, 80). The error is `HarvestImplausible`.
- Refused location checks are stored in `td.infringements` (`gps_weak`, `travel_speed`), shown to admins on player
  info (A2). Nothing happens to the player beyond the refusal.
- M1 moderation is built: `player_reports`, `player_sanctions` (warn / suspend / ban, lift), the mod queue, device
  marks, and `account_status = suspended` with `suspended_until`.
- The Android app reads location through our own plugin (`TDLocation.androidlib`, `FusedLocation.java`), so it can
  read Android's "this fix is fake" flag. iOS has no location plugin yet (Unity's `Input.location`).

### How we catch it

Two kinds of signal. **Hard** signals start a lock at once. **Soft** signals only make a report for a human to look at.

| # | Signal | Kind | How |
|---|---|---|---|
| D1 | **Fake location flag** | Hard | Android: `Location.isMock()` (12+) or `isFromMockProvider()` (older), read in `FusedLocation.java`. iOS 15+: `CLLocation.sourceInformation.isSimulatedBySoftware` (needs a small `TDLocation.mm` plugin). Every request that carries a fix also sends `mock: true/false`. This catches the common fake GPS apps on unrooted phones. |
| D2 | **Teleport** | Hard | A jump faster than `SpoofTeleportKmh` (300) **and** longer than `SpoofTeleportMinKm` (5) since the player's last fix, after taking off both fixes' accuracy. Phones glitch by tens or hundreds of metres in town, not kilometres, so a 5 km floor keeps glitches out. A flight is fine: hours pass between fixes. Between 80 and 300 km/h stays as today (refused, noted, no lock), which covers fast boats and cars. |
| D3 | **Too-perfect movement** | Soft | Over the last 30 fixes: the exact same accuracy every time, no jitter at all while standing, or a dead-straight line at a constant speed (joystick apps). Noted as a report only. |
| D4 | **Old or future fix** | Soft | The phone sends the fix's own time. A fix more than 2 minutes old, or from the future, is refused like a weak fix and noted. Replay tools often send stale fixes. |
| D5 | **Phone integrity** (later) | Hard | Play Integrity (Android) and App Attest (iOS) say if the phone is rooted, jailbroken or running a modified app, which is how better spoofers hide D1. Needs a Google Cloud project and Apple setup, so it is its own phase before public launch. |

Not used: matching GPS to the IP address's country. VPNs and roaming make it wrong too often, and it adds personal
data we would have to disclose.

The client sends signals; **the server decides.** A modded app can stop sending `mock: true`, which is why D2, D3
and later D5 run on the server side.

### What happens to the player

**Decided (2026-11-11):** "suspend for X minutes" is a **signal lock**, not a sign-out. The player can still open the app,
so they hear the narrator and see the countdown, but every game action that uses location (harvest, scan, build,
collect, upload, attack, drone launch, job take) is refused until the lock ends. A mod can still give a full
suspension or a ban from the report. 

1. A hard signal refuses the action and starts a lock. Lengths are admin settings: `SpoofLockMinutes1/2/3`, default
   **30, 120 and 720 minutes**. Each incident inside `SpoofRepeatDays` (7) steps up one length; the third and later
   stay at the longest.
2. The app shows a full-screen **SIGNAL REJECTED** screen in the narrator's style (dark, no power colour, no call
   screen) with the time left, and plays the narrator line with its caption:
   - first incident in the window: `undersignal-spoof-first` ("The signal bends where you stand...")
   - any later one: `undersignal-spoof-repeat` ("Again. A false trail...")
3. When the lock ends, the next time the app opens it plays `undersignal-spoof-return` ("The Undersignal is
   listening again. Walk true.") once.
4. Their Tether Nodes and everything else keep running as normal; nothing is taken away.
5. Locks stack with nothing: a new incident during a lock does not add time, but it is still recorded.

Audio is in project files `audio/narrator/undersignal-spoof-*.ogg` (game) and `.mp3` (listening), rendered with the
narrator recipe (`audio/narrator/source/render-undersignal-spoof.py`). They are already copied to the home PC at
`TerranDominion-Mobile\Assets\Resources\Audio\Voices\narrator\` (add them in git with G1). Lines name no power and no other persona.

Player text on the lock screen (country-neutral):

> SIGNAL REJECTED
> The Undersignal has turned away from you.
> Back in 28:14

### What mods and admins get

- **Every incident makes a report** in the existing REPORTS queue, from the `system` account shown as
  **"Signal watch"**, tagged `AUTO: GPS spoofing`. Several incidents for the same player group under them, like
  player reports.
- The report shows: which signal (fake flag, teleport, pattern), when, the distance and speed of a jump, the lock
  given, and the player's past incidents and sanctions. Mods do **not** see coordinates; admins can open the two
  fixes on a map from player info.
- Mod actions, as M1: **dismiss** (false alarm: lifts the lock at once and removes that incident from the step-up
  count), warn, suspend for days, ban. Admins can lift a lock from SANCTIONS like any other.
- The ADMIN button shows a count of open reports, so new ones are seen without opening the queue.
- **Testers:** an admin can mark a player **spoof-exempt** (player info, logged). Needed for anyone who tests with a
  fake location on purpose; you and Shinan would be exempt by default on dev.

### Settings (admin SETTINGS, new group "Anti-cheat")

| Key | Default | What |
|---|---|---|
| `SpoofLockEnabled` | on | Off = report only, no lock (use if false alarms show up) |
| `SpoofCheckMockFlag` | on | Act on D1 |
| `SpoofLockMinutes1/2/3` | 30 / 120 / 720 | The X: lock lengths |
| `SpoofRepeatDays` | 7 | Window for stepping up |
| `SpoofTeleportKmh` | 300 | D2 speed |
| `SpoofTeleportMinKm` | 5 | D2 shortest jump |
| `FixMaxAgeSeconds` | 120 | D4 |

### Data and API

- `td.infringements` gains kinds `mock_location`, `teleport`, `pattern`, `stale_fix`, and a `hard` flag.
- `player_sanctions` gains kind `spoof_lock` with `until` and `by = system`. Lift works as for suspensions.
- `td.players.spoof_exempt` (bool).
- New error `SignalLocked` (403, data: `until`, `incident` = first or repeat) returned by every location action
  during a lock. `GET /v1/me` returns `signalLockedUntil` and `signalLockLine` so the app shows the screen on start.
- Fix-carrying requests gain `mock` (bool) and `fixAt` (the fix's own time). Old app builds without them still
  work; raise the minimum build once G1 ships.
- `player_reports` accepts `system` as reporter with `kind = spoof` and the incident ids.

### Phases

| Phase | What | Where |
|---|---|---|
| **G0** | D2 teleport and D4 stale fix, signal lock with step-up, `SignalLocked`, system reports, settings, exempt flag, tests | API |
| **G1** | D1 fake flag (Android in `FusedLocation.java`, iOS `TDLocation.mm`), send `mock` and `fixAt`, lock screen, three narrator lines, open-report count on ADMIN | Mobile |
| **G2** | D3 pattern checks, report only | API |
| **G3** | D5 Play Integrity and App Attest (needs your Google Cloud and Apple setup steps) | API + mobile, before public launch |

Tests: a teleport over the floor locks and reports; a 1 km glitch at good accuracy does not; a flight hours long does
not; step-up across three incidents and reset after the window; dismiss lifts the lock and the count; exempt players
are never locked; every location endpoint refuses during a lock; mods see no coordinates.

**Privacy:** `legal/privacy.html` adds one line: location fixes are checked for faking, and incidents are kept for
moderation for 90 days. Ships with G1.
