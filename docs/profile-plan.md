# Player profile, share links, admin and moderation: plan

Status: **approved 2026-11-11** (callsign, ban, perks, links and build order decided). Nothing built yet.
GAME_DESIGN.md (mobile repo) has the matching "Player profile, sharing and moderation" section.

## Goal

1. Every player has a **profile page** in the game.
2. Tapping the **operative card** on the main menu opens your own profile. Tapping **any other
   player's name** in the game opens theirs.
3. Every profile has a **share link**. Sent over WhatsApp, Viber and so on, it opens the profile in
   the game if it is installed, or a small web page with the store buttons if it isn't.

## What is in the game today (checked in the code)

- `td.players.display_name` is set from the **Google or Apple account's real name** at first sign-in
  (`defaultDisplayName` in `service/accounts.go`). There is no way to change it. The card shows
  `OPERATIVE: <REAL NAME>` and the hexagon shows initials.
- `photo_file` and `country_code` columns exist but nothing uses them yet.
- The internal player id is a small counting number (you are 2, Shinan is 4). It already appears
  in one response (`BranchMember.playerId`).
- Other players' names reach the phone as plain strings with no id: `TetherView.ownerName`,
  `FactionView.leaderName`, `Notice.actor` (the actor id is stored but not sent), and names inside
  notice and outcome text.
- No leaderboards, no deep links, no share sheet, no web pages served by `tdapi`.
- Android and iOS app id: `com.hisstudios.terrandominion`.

## Decisions this plan needs (Open until you choose)

| # | Question | Recommended |
|---|---|---|
| D1 | **Which name do other players see?** **DECIDED 2026-11-11: player-chosen callsign.** | Players pick a **callsign** (3 to 16 letters, digits, `-` or `_`, unique, case-insensitive). The real name from Google or Apple is never shown to anyone. Until a callsign is picked, show `OPERATIVE-` plus the share code. Asked once, right after sign-in, and editable from your profile (once every 30 days). |
| D2 | **What goes in the share link?** | A random **8-character share code** (`td.players.public_code`, e.g. `K7QX3M9P`), never the counting id (anyone could walk 1, 2, 3...) and never the callsign (it can change and links would break). |
| D3 | **Which web address?** **DECIDED 2026-11-11: dev links for now; the owner gets a new domain before launch.** | Dev: `https://td-dev.demonreeper.xyz/p/<code>`. Release: a short neutral domain you own (Open: which one). App Links need the domain fixed before release, because the app build carries it. |
| D4 | **Avatar** | v1 keeps the hexagon with callsign initials. Later: pick from a set of game emblems (no cross shapes, no photos, so nothing to moderate). The unused `photo_file` stays unused. |
| D5 | **Show the region?** | Not in v1. Player-facing text stays country-neutral, and the faction line already shows rank. |

## What a profile shows

**Everyone sees** (another player's profile, the web page, and your own):

- Hexagon avatar with initials, `OPERATIVE: <CALLSIGN>`, `LVL nn` (no XP bar for other players).
- Standing line, same as the card: `FREELANCER // UNAFFILIATED`, or `<FACTION> // <RANK TITLE>`
  (Member, Warden, Regional Commander) in the faction colour.
- Badges (Founder of a branch; later the early-tester badge).
- Service record, all counted by the server:
  - Active since (month and year)
  - Nodes harvested
  - Tether Nodes built / active now
  - Tether Nodes destroyed (attacks won)
  - Drones intercepted
  - Missions completed
- `SHARE PROFILE` button.

**Only you see** on your own profile: the XP bar and XP to next level, carry and stored resources
(today's card line), `EDIT CALLSIGN`, and `COPY LINK`.

**Never shown to others:** real name, email, sign-in providers, resources, gear, items, Tether Node
locations, exact last-seen time, anything with a position in it.

**Deleted or banned players:** the name shows as `Deleted player` and is not tappable; their share
link shows "This operative is no longer active." Banned accounts look the same as deleted ones to
others.

## Where tapping opens a profile

| Place | Today | Change |
|---|---|---|
| Main menu operative card (screenshot) | Not tappable | Whole card is a button: your own profile. |
| Tether Node panel title, `<NAME>'S TETHER NODE` | Text | Name is a link (underline + accent colour). `TetherView.owner` becomes `{code, name}`. |
| Factions list, `LED BY <NAME>` | Text | Name opens the Regional Commander's profile. `FactionView.leader` becomes `{code, name}`. |
| Faction branch member list | Row with APPOINT | Tapping the name opens the profile; APPOINT stays a separate button. `BranchMember` swaps `playerId` for `code`. |
| ALERTS (attacks, drone strikes, intercepts, contests, defused spikes, Warden appointments) | Name inside text | Alert row with a player gets a small `PROFILE` action. `Notice.actor` becomes `{code, name}` (null for system notices). |
| Combat outcome toasts and popups that name a player | Text | Same `{code, name}` object where a name is shown, tappable where it is a panel (toasts stay plain). |
| Future leaderboards, faction rosters, chat | n/a | Every new list that shows a player uses the same `PlayerRef` and the same tap rule. |

Rule for later work: **any player name on screen comes with a `PlayerRef` and opens the profile.**
One shared UI helper (`UiBuilder.PlayerName(...)`) draws the tappable name, so this stays consistent.

## API

Breaking changes are fine now, so no compatibility fields.

**Data (migration 0036_profiles.sql)**

- `td.players.public_code text UNIQUE NOT NULL`: 8 characters of Crockford base32 (no I, L, O, U),
  filled for existing players in the migration and made on sign-up.
- `td.players.callsign text`, `callsign_changed_at timestamptz`, and a unique index on
  `lower(callsign)`.
- `display_name` keeps the provider name for support and admin use only. Every player-facing query
  switches to `COALESCE(callsign, 'OPERATIVE-' || public_code)` through one SQL helper.
- A small blocked-words list (setting `CallsignBlocked`) and reserved words (admin, support,
  moderator, the six power names, faction names).

**Shared type**

```json
"owner": { "code": "K7QX3M9P", "name": "ECHO-7" }
```

`PlayerRef {code, name}` replaces `ownerName`, `leaderName`, `actor` and `BranchMember.playerId`.
`/v1/me` gains `code`, `callsign` (null until picked) and `shareUrl`.

**Endpoints**

| Endpoint | What it does |
|---|---|
| `GET /v1/players/{code}` (signed in) | Public profile: ref, level, standing, badges, service record, `isMe`. 404 for unknown, deleted or banned. |
| `PUT /v1/me/callsign` `{callsign}` | Checks format, blocked words, uniqueness and the 30-day limit. Errors: `callsign_taken`, `callsign_invalid`, `callsign_too_soon`. The first pick is free of the limit. |
| `GET /p/{code}` (public, no sign-in) | Web profile page (HTML). Same public fields, the game's look, "Open in Terran Dominion" and the store buttons. Open Graph tags so chat apps show a preview card. Cached by Cloudflare for 5 minutes, rate limited. |
| `GET /.well-known/assetlinks.json` | Android App Links file (app id + signing SHA-256 fingerprints). |
| `GET /.well-known/apple-app-site-association` | iOS Universal Links file (team id + app id, path `/p/*`). |
| Later: `GET /p/{code}/card.png` | A rendered preview image for link previews (phase 4). |

The service record comes from tables that already exist (`harvests`, `tether_nodes`, `xp_events`,
`drone_strikes`, `player_missions`), one query with counts. If it gets slow, keep a
`player_stats` row updated by the actions instead; not needed at this size.

The web page lives in the API repo (`web/profile.html` template, embedded in `tdapi` like the
migrations), per the rule that web pages go in the API repo.

## Mobile (Unity)

- **ProfileController + UiBuilder.Profile**: a full screen in the menu style (same card on top, the
  service record as rows below, badges as chips). Opens over whatever screen called it; BACK
  returns there. Loads `GET /v1/players/{code}`; your own profile uses `/me` for the private parts.
- **Callsign prompt**: after sign-in, if `callsign` is null, a one-field screen "CHOOSE YOUR
  CALLSIGN" with a live availability check on submit. SKIP is allowed (you keep `OPERATIVE-<code>`
  and are asked again next launch, at most once a day).
- **Share**: Android uses an `Intent.ACTION_SEND` chooser through `AndroidJavaObject` (no plugin).
  iOS needs a tiny native `UIActivityViewController` bridge (`Plugins/iOS/TDShare.mm`), or the free
  open-source NativeShare package for both. Recommended: our own small bridge, like TDAuth/TDPush.
  Share text: `Find me in Terran Dominion: <link>` (country-neutral).
- **Opening links in the game**:
  - Android: an App Links intent filter (`autoVerify`, `https`, host, path prefix `/p/`) on the
    existing `MessagingUnityPlayerActivity` in `Plugins/Android/AndroidManifest.xml`.
  - iOS: the Associated Domains entitlement `applinks:<host>`, added by the post-build step that
    already edits the Xcode project, so Codemagic signing picks it up.
  - Unity: read `Application.absoluteURL` at start and listen to `Application.deepLinkActivated`.
    A `/p/<code>` link opens that profile. If the player isn't signed in yet, it is kept and opened
    right after sign-in. A link to your own code opens your own profile.
- **Tap targets** from the table above, through the shared `PlayerName` helper.

## Phases

| Phase | What | Repos |
|---|---|---|
| **P0 Callsign and share code** | Migration, callsign rules and endpoint, every view switched to `PlayerRef`, callsign prompt, card shows the callsign | API + mobile |
| **P1 Profile screen** | `GET /v1/players/{code}`, profile screen, own profile from the operative card, all tap targets | API + mobile |
| **P2 Share links** | `/p/{code}` web page with OG tags, share button (Android + iOS bridge), copy link | API + mobile |
| **P3 Open in the game** | `.well-known` files, App Links intent filter, Associated Domains, deep link routing | API + mobile + your steps |
| **P4 Nicer previews** (optional) | Rendered `card.png` preview image; emblem avatars | API + mobile |
| **A1 Admin settings** | MonoCity-style settings screen (groups, ranges, locks, search, confirm, history, announcement) | API + mobile |
| **P1b Faction perks** | Perks from Side fix 2; numbers tuned from A1 | API + mobile |
| **M1 Reports and moderation** | Report a player, mod queue, warn / suspend / ban, admin lifts | API + mobile |
| **A2 Admin players and stats** | Player search and info, adjust, roles, callsign reset, game stats + share card, player share cards | API + mobile |

**Build order (decided 2026-11-11):** P0, P1, A1, P1b, P2, M1 (needs profiles to report from), A2, P3, P4. A1 comes
before the perks so their numbers can be tuned on the phone. P0 to P2, A1, M1 and A2 need nothing
from you beyond reviewing. P3 needs the steps below.

## Your steps (P3 only)

1. Get the release domain for share links before launch (D3). Dev uses `td-dev.demonreeper.xyz` until then; P3 can be built and tested on dev.
2. Android: send the SHA-256 of the upload key (and later the Play App Signing key, the same place
   as the SHA-1 step already on the pre-Play-Store list) for `assetlinks.json`.
3. iOS: in the Apple developer portal, turn on **Associated Domains** for
   `com.hisstudios.terrandominion`, then let Codemagic fetch a fresh provisioning profile.
4. Cloudflare: leave `/.well-known/*` uncached or short-cached, and make sure no redirect touches it
   (both platforms refuse redirected verification files).

## Tests

- API: callsign rules (format, taken, case, blocked, 30 days), public code uniqueness, profile hides
  private fields and deleted/banned players, `/p/{code}` 404 and HTML escaping, every view now
  returns `PlayerRef`.
- Mobile: edit-mode tests for link parsing (`/p/<code>`, bad codes, own code) and the pending-link-
  until-signed-in path. HUD capture adds the profile screen and the callsign prompt.
- On the phone: share to WhatsApp, tap the link with the game installed and not installed, on
  Android and iOS.

## Side fix: "credit" in daily missions (asked 2026-11-11)

**What it is.** Credit is the fourth map resource (crystal, alloy, data, credit), the rarest one
(nodes come 4 : 3 : 2 : 1, so about 1 node in 10). It pays for Tether Node builds and upgrades and
drone bay upgrades. A finished mission pays `MissionCreditBase + MissionCreditPerLevel x level`
credits (5 + 1 per level now), and claiming puts them **straight into storage**, not carry
(`ClaimMission`, `service/missions.go`).

**Why it reads as unclear.** ORDERS shows `+7 CREDIT` with nothing tying it to the resource; the
operative card shows only one `STORED` total, so the credit vanishes into a number; and two Ishāra
missions ask you to *harvest* credit, which reads like a currency rather than a node type.

**Fix (goes with P1, mobile only):**
- ORDERS reward line uses the resource's colour and icon and says where it went:
  `+7 CREDIT (STORED)`. The claim toast says `7 credit added to storage.`
- Harvest missions name the node: `Harvest 3 credit nodes` in the progress line (the power's spoken
  line stays as written).
- Your own profile's private section shows stored resources **per type** (crystal, alloy, data,
  credit) instead of one total, so mission credits can be seen arriving.
- First time a mission pays credit, a one-line tip: `Credit is the rarest resource. It builds and
  upgrades Tether Nodes.`

Open: whether credit should later become a separate currency (for bounties and trading, per the
combat table) instead of a harvested resource. Not changed here.

## Side fix 2: faction perks (asked 2026-11-11)

**Checked: faction perks are not built.** GAME_DESIGN.md lists one per faction in the concept table
("+25% crystal", "scanner range doubled", and so on), but nothing in the API reads a player's
faction for harvests, scan range, carry, costs, drones or damage. Today, joining a faction only
changes who sends your missions, tags your Tether Nodes with the faction, and gives you a rank. The
Factions screen shows the name, motto and commander, and nothing about perks.

**Plan: build the perks that fit systems we already have** (numbers are settings, so they can be
tuned without an app update; numbers **confirmed 2026-11-11**, tuned later from admin settings if needed). Perks apply only while you're a member and
stop the moment you leave.

| Power (faction) | Concept perk | v1 perk (built now) | Later, when the system exists |
|---|---|---|---|
| Signum (Lumen Collective) | +25% crystal from every node | **+25% crystal** from each harvest and each Tether Node collect | — |
| Shingō (Forge Meridian) | Outposts upgrade twice as fast | **Tether Node upgrades cost 25% less** | Base upgrades twice as fast |
| Sēma (Veil Cipher) | Scanner range doubled, hidden nodes revealed | **Scan range x2**; rare roaming nodes show from twice as far | Hidden nodes |
| Ishāra (Verdant Exchange) | Trade bonuses, cheaper builds | **Tether Node builds cost 20% less**, **+25% mission credit** | Trade bonuses, lower bounty fees |
| Tohu (Slipstream Vanguard) | Faster drones, bigger carry | **Drones fly 30% faster**, **carry +25%** | — |
| Merki (Bastion Order) | Claims resist raids, shields recharge fast | **Your Tether Nodes take 15% less damage**, **intercepts stop 10% more** | Base shields recharge fast |

**API:** one `game.FactionPerks(settings, factionCode)` used by harvest, collect, scan range, carry,
build/upgrade cost, drone flight time, damage and intercept. `/v1/factions` returns each faction's
`perks` as short lines; `/v1/me` returns `activePerks`. Settings keys `Perk<Name>` (e.g.
`PerkSignumCrystalPct = 25`). Tests: each perk applies to members only and stops after leaving.

**Faction screen (clearer):**
- Every faction card gets a **PERKS** block under the motto, with the numbers:
  `+25% CRYSTAL FROM HARVESTS AND TETHER NODES`.
- A freelancer sees under it: `Members get these perks. Joining is open at level 20.`
- Your own faction's card shows the block in the faction colour with `ACTIVE ON YOU`.
- The join confirmation repeats the perks, and that leaving removes them.
- Wherever a perk changes a number the player sees (harvest result, scan ring, carry cap, build and
  upgrade cost), a small tag shows it, e.g. `+25% SIGNUM`, so the effect is visible in play.

**Profile:** the standing line stays `<FACTION> // <RANK>`; your own profile and other players'
profiles show the faction's perk chips under it, so a perk is visible on anyone in that faction.

Goes in as its own phase (**P1b Faction perks**, API + mobile), after P1, since it doesn't depend on
callsigns. GAME_DESIGN.md's concept table gets a "v1 perk" column once the numbers are confirmed.

## Admin area and moderation (asked 2026-11-11)

Modelled on MonoCity's admin screens (`lib/screens/admin_settings_screen.dart`,
`admin_players_screen.dart`, `mod_*`, `core/stats_card.dart`, `core/brag_card.dart`), in the game's
own look. Today's in-game ADMIN panel only has the minimum app build and the sound board.

**Roles.** `is_mod` and `is_admin` already exist on players. Admins can do everything mods can.
The main menu's ADMIN button shows for both; each sees only the tabs their role allows.

| Tab | Who | What |
|---|---|---|
| REPORTS | Mods, admins | The report queue and sanctions |
| PLAYERS | Admins (mods: read-only, no private fields) | Search, player info, actions |
| SETTINGS | Admins | Every tunable number, MonoCity-style |
| STATS | Admins | Game totals and the share-stats card |
| FACTIONS | Admins | Look inside a faction without joining it (added 2026-11-11): everything a member sees (branches, commander, members now; chat and commanders' orders when they exist), moving between its regions. Never visible to players, logged |
| TOOLS | Admins | Today's panel: minimum app build, sound board, node seeding |

### A1 Settings (MonoCity-style)

- `td.settings` gains `grp` (harvest, tether, combat, drones, missions, factions/perks, profile,
  moderation, app), `label`, `min_value`, `max_value` and `locked`. A migration fills them for all
  existing keys; anything not grouped lands in "other".
- The screen: a search box (matches name, description, key or group, every word, like MonoCity's
  `adminSettingMatch`), groups that fold open, each row with name, description, value and its
  `min to max` range. Locked rows are greyed and can't be tapped (things that would break saves,
  e.g. resource ratios mid-season).
- Tap a row: number field with minus and plus, then a confirm "Change <name> from A to B?".
  The API refuses values outside the range or on locked keys.
- **Recent changes** at the bottom: who, when, old to new (from `admin_actions`, which already
  logs every change; the old value is added to its `detail`).
- **Announcement** row, as in MonoCity: a one-line banner on the main menu with kind (info, event,
  warning) and hours to show. Country-neutral wording is the admin's job; the screen reminds them.
- API: `GET /v1/admin/settings` returns groups, rows and the last 30 changes; `PUT
  /v1/admin/settings/{key}` checks the range and lock; `PUT /v1/admin/announcement`.

### M1 Reports and moderation

**Reporting (players).** On another player's profile: `REPORT` → one reason for now, **Griefing**,
with a short list to pick what happened (repeated attacks on my Tether Nodes, blocking a node,
following me, other) and an optional note (300 characters). The server attaches what it knows about
the two players: their attacks, drone strikes, intercepts and contests on each other in the last 7
days (from `notices` and `tether_attacks`). Limits: one open report per reporter per player, 5
reports a day per reporter, and you can't report yourself. Reporters see "Thanks. A moderator will
look at it." and later a notice "Your report was reviewed." (never what was done).

**Mod queue.** Reports grouped by reported player, most reports first: callsign, level, faction,
how many reporters, every note, the attached interactions, and that player's past sanctions. Mods
see no real name, email, device or location.

**Actions** (a reason is required for every one; all are logged):

| Action | Effect |
|---|---|
| Dismiss | No action; reports closed |
| Warn | The player gets an in-game message from "Moderation" with the reason |
| Suspend | 1, 3 or 7 days (settings `SuspendDays1/2/3`). Sign-in and refresh refused with "Suspended until <date>"; sessions revoked at once. Their Tether Nodes keep decaying as normal |
| Ban | Permanent. Sessions revoked; shows as "Deleted player" to others. **Decided 2026-11-11: nothing is removed.** Their Tether Nodes and everything else stand; they just can't sign in, and their Tether Nodes decay away on their own within 48 hours |

Mods can't act on admins, other mods or themselves. A second suspension inside 30 days suggests the
next step up (shown, not forced).

**Admin reversal.** A SANCTIONS list (active and past) with `LIFT` on each suspension or ban:
reason required, the player can sign in again at once, and the history keeps both entries. Admins
can also reopen a dismissed report and remove a mod's role.

**Data:** `td.player_reports` (reporter, target, reason, detail, note, context jsonb, status open /
dismissed / actioned, handled_by, handled_at, sanction_id) and `td.player_sanctions` (player, kind
warn / suspend / ban, reason, until, by, created_at, lifted_at, lifted_by, lift_reason).
`account_status` gains `suspended`; a job returns expired suspensions to `active`.

**API:** `POST /v1/players/{code}/report`; mod: `GET /v1/mod/reports`, `POST
/v1/mod/reports/{id}/dismiss`, `POST /v1/mod/players/{code}/sanction {kind, days, reason,
reportIds}`; admin: `GET /v1/admin/sanctions`, `POST /v1/admin/sanctions/{id}/lift`. A new `mod()`
middleware lets mods and admins through.

### A2 Admin players and stats

**Player search** by callsign, share code, id or email. **Player info** (admins), like MonoCity's
`adminPlayerDetailText`: callsign and real name, email and sign-ins, level and XP, faction and
rank, carried and stored resources, Tether Nodes, registered, last seen, app build and platform,
other accounts on the same phone (from `devices` and `sessions`), cheating records (failed GPS or
travel-speed checks, which start being stored in `td.infringements`), reports by and against,
sanctions, and past adjustments.

**Actions:** open their profile; **adjust** resources or XP (plus or minus, reason required,
logged); **reset callsign** (they must pick again; for offensive names); make or remove mod;
suspend, ban or lift (same as M1). MonoCity's "play as" is left out for now.

**Player stats and sharing:**
- Each player's stats are the profile's service record (P1).
- **Share card** (like MonoCity's brag cards): next to SHARE PROFILE, players can share a picture
  of their card (callsign, level, faction, badges, three top stats) with their link, made on the
  phone. Also offered at level-ups and badges.
- **Game stats** (admins, top of STATS, like MonoCity's "today's numbers"): players total and new
  this week; active 24 h, 7 d, 30 d; harvests this month; Tether Nodes active; fights this week;
  faction members per faction; 7-day retention; a 14-day bar chart of active players.
- **Share stats card** (like MonoCity's `StatsCard`): one picture of those totals for testers and
  partners. Counts only, no names, no money, no country names. `GET /v1/admin/stats`.

### Tests

API: settings range and lock checks, report limits and self-report, mod can't sanction mods or
admins, suspended sign-in refused until the date then allowed, lift restores access, mods never get
private fields. Mobile: tab visibility per role, the report flow and HUD captures of each tab.

## Side fix 3: map drops last 48 hours (asked 2026-11-11)

Today a map drop (what an ended Tether Node held) lasts **24 hours**: `DropLifetimeHours = 24`
(migration 0011, default in `game/settings.go`). The owner wants **48 hours**, the same as a Tether
Node's life (`TnLifetimeHours = 48`, the same for every player). Fix: a migration sets
`DropLifetimeHours` to 48, the code default changes to match, and GAME_DESIGN.md says both lifetimes
are 48 hours. Existing drops keep their old end time. Until it ships, the admin settings endpoint
can change it on dev now: `PUT /v1/admin/settings/DropLifetimeHours` with 48.

**Lore (owner 2026-11-11): the Undersignal reclaims it.** A drop nobody picks up is taken back by the
Undersignal, the same force that rejects Tether Nodes (LORE.md). Player-facing text: the drop's map
label and panel show `RECLAIMED BY THE UNDERSIGNAL IN 31H`, and the owner's notice when one ends
unpicked says `The Undersignal reclaimed what your Tether Node left behind.` LORE.md's Tether Node row
adds that unclaimed spills are reclaimed after 48 h.

## Not in this plan

Friends or follow lists, messaging, blocking, leaderboards themselves, report reasons other than griefing. The
`PlayerRef` rule makes each of these plug into the profile when they come.
