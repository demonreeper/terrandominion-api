# Walk together (signal link): plan

Status: **approved 2026-11-11** (owner). The owner's local Claude Code builds it; the prompt is
`docs/walk-together-build-prompt.md`. GAME_DESIGN.md in the mobile repo has a short section.
On the owner's go this goes to the API repo as `docs/walk-together-plan.md`, with a short GAME_DESIGN.md entry.

## The ask (owner, 2026-11-11)

> If friends in game walk together they get a bonus on scan range and collection amount, or more,
> even if different factions.

It grows out of the teaser line "Walk with friends. Or race them to it."

## The idea in one paragraph

Two to four **accepted friends** who are physically together tap **WALK TOGETHER** and form a
**signal link**. While the link holds and the group is **actually walking**, every member gets a
bigger **scan range** and a bigger **harvest and drop amount**, scaled by group size. Faction does
not matter: freelancers, members of the same power and members of rival powers can all link. The
server checks that the phones are really side by side (fresh, accurate, nearly identical fixes from
different devices, moving together), and every link check runs on top of the anti-spoof rules.

Lore line (in-game tip, no power named): *"Signals that walk together carry further."*

## Rules

| Rule | Detail |
|---|---|
| Who can link | Accepted friends only (FR1). Any faction mix, freelancers included. Not while signal-locked, suspended or banned. |
| How a link forms | A taps WALK TOGETHER, then a nearby friend's LINK button. B gets an in-app prompt `<CALLSIGN> wants to walk together.` with ACCEPT. The server links them only if both fixes pass the proximity check below. Asking expires after 2 minutes. |
| Group size | 2 to 4 (`WalkLinkMaxSize`). A new member must be friends with the member who invites them; they don't have to be friends with everyone. Non-friends in a group see each other's callsign only, never the friend name. |
| Range | Formed within `WalkLinkFormM` (30 m). Holds while every member stays within `WalkLinkHoldM` (60 m) of at least one other member. |
| Breaking | A member drifts out of range, or misses heartbeats, for `WalkLinkGraceSec` (45 s); taps LEAVE; closes or backgrounds the app; gets signal-locked; or is signal-locked, or attacks a linked member (their Tether Node, them, or their faction's base). A group that drops to one player ends. |
| Must be walking | The link stays up while standing still, but the **bonus is paused** until the group has moved `WalkLinkMoveM` (50 m) within the last `WalkLinkMoveWindowSec` (180 s). The HUD then says `KEEP WALKING`. This makes it a walking bonus, not a sit-on-a-bench bonus. |
| One link at a time | A player is in one group at most. |
| Foreground only | Like the rest of the game, the link only works with the app open. |

## Bonuses

All numbers are starting values in admin SETTINGS (new group "Walk together"), so they can be tuned
without an app update. Per the video rule, videos and store text say "walk with friends to scan
further and gather more", never the numbers.

| Group | Scan range | Harvest and map drop amount | XP |
|---|---|---|---|
| 2 | +25% | +15% | +10% |
| 3 | +40% | +25% | +20% |
| 4 | +50% | +30% | +30% |

- **Applies to:** scan radius (including how far rare roaming nodes show), node harvests and map
  drop pickups. Server-side, at action time: the action reads the player's live link.
- **Does not apply to:** Tether Node collects (owner, 2026-11-11; a group could just stand at a TN), mission rewards,
  drone strikes, damage and shields. See Open 2.
- **Stacks with faction perks by adding**, never multiplying: Sēma's scan x2 (+100%) plus a 2-link
  (+25%) is +125% on base. A cross-faction pair each gets their own perks plus the same link bonus.
- **Carry capacity still applies**, so a bigger harvest fills the rig sooner.
- **Daily cap:** linked bonus counts for at most `WalkLinkDailyMinutes` (120) of bonus-active time
  per player per day. After that the link stays up for company but the bonus stops, with the chip
  `LINK BONUS SPENT TODAY`. This caps what a two-phone farmer can gain.
- In play the existing tag style shows it: `+15% LINKED` on a harvest result, a wider dashed ring on
  the scan, next to any faction perk tag.

## Proximity check (server decides)

On forming and on every heartbeat (every `WalkLinkHeartbeatSec`, 15 s, while linked) each phone sends
its current fix with `mock` and `fixAt` (fields added by G1). The server accepts a member as "with
the group" only if all of these hold:

1. Fix age ≤ 20 s (`fixAt`), accuracy ≤ 25 m (`WalkLinkMaxAccuracyM`), `mock = false`.
2. Distance to another member's latest fix ≤ range, after subtracting both accuracies (capped so
   that two poor fixes can't "cover" 100 m).
3. The two fixes were taken within 20 s of each other.
4. Different device ids (the device binding from the pledge plan). Two accounts on one phone can never link.
5. Each fix passes the normal G0 checks (teleport, travel speed, stale fix).

The friend's exact distance is never shown, only "nearby" in the LINK list. The LINK list shows only
friends who have WALK TOGETHER open **and** are within 30 m, so it never tells anyone where a friend
is beyond what they can see with their own eyes.

## Anti-spoofing

The link is the easiest thing in the game to farm with two phones and a fake GPS app, so it gets its
own checks on top of Part C of `social-signal-plan.md`.

| # | Check | Kind | What happens |
|---|---|---|---|
| L1 | Fake location flag on any member | Hard | Normal spoof incident (lock + report) for that member; the link ends for everyone. |
| L2 | Any member signal-locked | Hard | Can't form or stay in a link. |
| L3 | **Twin fixes**: two members send the same coordinates (to ~1 m) and the same accuracy several times in a row | Soft | Real phones side by side always differ a little. Bonus stops for that group, report "Signal watch: twin fixes". |
| L4 | **Moves in lockstep**: the members' paths match too perfectly over the last minutes (same jitter, same speed changes) | Soft | Report only (built with G2's pattern checks). |
| L5 | **Same pair, every day, for the full cap**, or one player linking with many "friends" made that week | Soft | Report only, so a mod can look at possible alt accounts. |
| L6 | Same device, or same sign-in on two devices | Hard | Refused (`walk_same_device`). |
| L8 | **GPS together, Bluetooth never**: the GPS check passes for minutes but no member ever hears another | Soft | Bonus already stopped; report "Signal watch: no Bluetooth contact". |
| L7 | Phone integrity (G3, later) | Hard | Once Play Integrity / App Attest is in, a link requires a passing token from every member. Strongest defence against rooted spoofers. |

Spoof-exempt test accounts (players 2 and 4 on dev) skip L1 to L5 so the owner and Shinan can test
the flow with a fake location. They do **not** skip Bluetooth, so the BLE check gets tested for real.

**Bluetooth proof (in the first build, owner 2026-11-11):** GPS alone can't catch two people in
different places faking the same path in sync. So every linked phone also does a Bluetooth LE check:

- The server gives each member a **rotating token** (new one every 60 s) in each heartbeat reply.
- The phone **advertises** its token over BLE and **listens** for the other members' tokens.
- Each heartbeat reports the tokens it heard in the last 30 s (`bleHeard`).
- A member is "Bluetooth-confirmed" if at least one other member heard their current or previous
  token, or they heard one. Phones a few metres apart hear each other; phones in different places don't.
- **The bonus needs Bluetooth** (`WalkLinkRequireBle`, on). GPS still decides who is in range.
- **Permission:** asked the first time a player taps WALK TOGETHER, with the line
  `Lets linked phones confirm they are together.` If they say no, they can still link for company,
  but without the bonus (chip: `BLUETOOTH OFF: NO BONUS`), since a spoofer would otherwise just decline.
- Missing Bluetooth for `WalkLinkBleGraceSec` (60 s) pauses the bonus (chip `CONFIRMING...`); it
  doesn't end the link, because BLE in a pocket can drop for a moment.
- Members who keep linking with GPS that says "together" but never hear each other make a soft
  report (L8 below).

## Where it shows in the game

- **Friends screen** and the map HUD: a `WALK TOGETHER` button. It opens a small panel: friends
  nearby with WALK TOGETHER open, each with `LINK`; incoming asks with `ACCEPT`.
- **First time:** a one-line consent box: `Linked friends see you on their map while you walk
  together.`
- **While linked:** a HUD chip `LINKED x2` (hexagon link icon in neutral cyan, not a power colour)
  with the friends' callsigns on tap; the bonus icons (scan, gather, XP); `KEEP WALKING` when paused.
  Linked members show as small markers on each other's map, **only while linked**. This is the one
  place a friend's location is ever shown, and only with both players' tap.
- **Link sound** on forming and a softer one on breaking; light haptic on both.
- **Breaking toast:** `Link lost: <CALLSIGN> fell behind.` / `Link ended.`
- **Profile stat** (private, your own profile): `WALKED TOGETHER: 12.4 KM`.

## Server (API)

**Data** (migration, next free number, e.g. `00xx_walk_links.sql`)

- `td.walk_links`: `id`, `created_at`, `ended_at`, `end_reason`.
- `td.walk_link_members`: `link_id`, `player_id`, `device_id`, `joined_at`, `left_at`,
  `last_fix_at`, `last_lat`, `last_lng`, `last_acc`, `bonus_seconds_today`.
- `td.walk_link_asks`: `from_player`, `to_player`, `created_at` (2-minute expiry).
- `td.players.walked_together_m` (the profile stat).
- `td.walk_link_members` also keeps `ble_on`, `ble_last_confirmed_at`; tokens are random, kept in
  memory or a short-lived table, and never tied to a fixed device id over the air.
- **Privacy:** latest fix only, overwritten each heartbeat, and wiped when the member leaves. Kept
  for 90 days: who linked with whom, when, how long, distance, end reason (for L5 and moderation).
  Mods see no coordinates (same rule as Part C).
- Settings: `WalkLinkEnabled`, `WalkLinkMaxSize`, `WalkLinkFormM`, `WalkLinkHoldM`, `WalkLinkGraceSec`,
  `WalkLinkHeartbeatSec`, `WalkLinkMaxAccuracyM`, `WalkLinkMoveM`, `WalkLinkMoveWindowSec`,
  `WalkLinkDailyMinutes`, `WalkLinkRequireBle`, `WalkLinkBleGraceSec`, `WalkLinkBleTokenSec`, and `WalkLinkScanPct2/3/4`, `WalkLinkYieldPct2/3/4`, `WalkLinkXpPct2/3/4`.

**Endpoints** (all signed in, all carry a fix)

| Endpoint | What it does |
|---|---|
| `POST /v1/walk/open {fix}` | "I have WALK TOGETHER open." Returns friends nearby who also have it open (PlayerRef only) and incoming asks. |
| `POST /v1/walk/ask {code, fix}` | Ask a friend. Errors: `walk_not_friends`, `walk_too_far`, `walk_group_full`, `walk_locked`, `walk_same_device`, `walk_disabled`. |
| `POST /v1/walk/ask/{code}/accept {fix}` | Join (or form) the asker's group, after the proximity check. |
| `POST /v1/walk/heartbeat {fix, bleOn, bleHeard[]}` | Every 15 s while linked. Returns members, your next `bleToken`, the members' tokens to listen for, `bonusActive`, `pausedReason` (`keep_walking`, `cap_spent`, `ble_off`, `ble_confirming`), bonus numbers, and members' positions. |
| `DELETE /v1/walk` | Leave. |

**Changes to existing code**

- One `game.WalkLinkBonus(settings, playerID, now)` beside `game.FactionPerks`, read by harvest, drop
  pickup, scan radius and XP. Returns 0 unless the player's link is live, moving and under the cap.
- Harvest, pickup and scan responses add `linkBonus` so the app can show the `+15% LINKED` tag.
- `/v1/me` adds `walkLink` (current group or null).
- Any attack on a linked member's TN, on the member, or (later) on a base of their faction ends the link (`end_reason = attack`); one check in the shared attack path.
- A job ends links whose members missed heartbeats.
- L3 and L5 write `player_reports` as the `system` "Signal watch" reporter, `kind = walk_link`.

## Mobile (Unity)

- `WalkTogetherController`: the panel, asks, accept, heartbeat loop (foreground only; pauses and
  leaves on app background), break handling.
- HUD chip, bonus icons, `KEEP WALKING`, linked friend markers on the map, wider scan ring.
- `+X% LINKED` tag on harvest and pickup results, next to faction perk tags.
- Link sounds and haptics (P2 haptics helper on iOS).
- Profile: `WALKED TOGETHER` stat.
- **BLE plugin `TDProximity`** (in W2): Android `androidlib` (BluetoothLeAdvertiser + scanner,
  Android 12+ `BLUETOOTH_ADVERTISE` / `BLUETOOTH_SCAN` with `neverForLocation`, older versions use
  the location permission they already have), iOS `TDProximity.mm` (CoreBluetooth peripheral +
  central, `NSBluetoothAlwaysUsageDescription`). Foreground only. Advertises a custom service UUID
  with the 8-byte token; scans only for that UUID.
- Permission ask on first WALK TOGETHER, and the `BLUETOOTH OFF: NO BONUS` / `CONFIRMING...` chips.

## Phases

Needs friends (FR1), the spoof checks with `mock`/`fixAt` and the signal lock (G0, G1), faction
perks (`internal/game/perks.go`), drops and seed-on-scan. Checked in the API repo on 2026-11-11:
all of these are built, so this can start after the current build.

| Phase | What | Where |
|---|---|---|
| **W1** | Data, settings, endpoints, proximity check, bonus in harvest/pickup/scan/XP, L1/L2/L6, daily cap, tests | API |
| **W2** | Panel, consent box, HUD chip, markers, tags, sounds, profile stat; **Bluetooth proof** (`TDProximity` plugin, tokens, permission, L8); L3 twin-fix report | Mobile + API |
| **W2b** | Extras X1 shared scan (+ shared scan perk), X2 group nodes, X3 link missions, X5 milestones | API + mobile |
| **W3** | L4 lockstep check (with G2), L7 integrity once G3 exists | API + mobile, before public launch |

**Tests:** forms only between friends, within range, on different devices; cross-faction pairs get
the bonus; bonus pauses when standing still and returns after walking; daily cap; break on distance,
missed heartbeats, background and lock; attacking a linked member's TN (or base, later) ends the link; mock flag on one member ends the link and locks only
that member; twin fixes make a report; no bonus without Bluetooth, bonus back once members hear each other, L8 report when GPS says together but BLE never hears; exempt accounts skip spoof checks; bonus adds to perks, not
multiplies; a non-friend in a group never sees the other's friend name. **On the phones:** you and Shinan walk a
few hundred metres in Malé, linked, and compare harvests and scan rings against walking alone, then one of you turns Bluetooth off to see the bonus stop.

## Extras (owner picked all five, 2026-11-11)

### X1 Shared scan
- When any linked member scans, the nodes, drops and rare roaming nodes it finds show on **every
  member's map** until the link ends (then each member keeps only what their own scans found).
- **Faction scan advantage is shared too** (owner): while linked, every member's scan uses the
  **best scan perk in the group**. A freelancer linked with a Sēma member scans with Sēma's x2 range
  (plus the link bonus), and gets Sēma's hidden-node reveal once that is built. Only the scan perk is
  shared; crystal, cost, drone and damage perks stay with their own faction.
- Server: scan results are written per link as well as per player; `heartbeat` returns `sharedFinds`.
  The perk tag reads `x2 SĒMA (LINKED)`.

### X2 Group nodes (duo, trio, quad)
- Rare nodes that only open for a linked group of a set size: **duo** (2+), **trio** (3+), **quad** (4).
  Any faction mix. Alone, or in a too-small group, the node shows locked with its size badge
  (`DUO`, `TRIO`, `QUAD`), no other details.
- Harvest needs the required number of linked members **within 40 m of the node and Bluetooth-
  confirmed**; each harvests it once and each gets the full reward (no splitting). Bigger nodes give
  more: starting yield x3 / x5 / x8 of a normal node, and a better chance of rare drops.
- Seeded by the node seeding job (drone-strike plan M0) at a low rate, more duo than quad
  (`GroupNodeRatePerCell`, `GroupNodeMix`), last 48 h like other drops, reclaimed by the Undersignal.
- Settings: `GroupNodeYieldX2/3/4`, rate and mix. Videos may show a locked DUO node, never yields.

### X3 Walk-together missions
- Sometimes one of the daily missions is a link mission: `Walk 1 km linked with a friend`, or
  `Harvest a duo node`. It comes from the power that sends your missions (freelancers: one of the six),
  and the line names no power. Distance counts only bonus-active (moving, Bluetooth-confirmed) metres.
- Setting `LinkMissionChancePct` (how often one of the day's missions is a link mission).

### X4 No truce (owner, 2026-11-11)
- Friends get **no truce**, linked or not. Whether to attack a friend is the players' own choice
  (for example on a faction mission). Attacking a linked member **ends the link** (`end_reason = attack`),
  confirmed by the owner 2026-11-11. That covers attacking their **Tether Node**, **the player
  themself** (any attack aimed at a player), or **a base** of their faction (once bases exist).
  Nobody keeps a shared scan with someone they just hit.

### X5 Milestones
- Profile badges for distance walked together (bonus-active metres only): **5 km, 25 km, 100 km,
  500 km**, plus a first-link badge and a first group-node badge. Shown on your profile and on
  others' profiles, like any badge. Distances can be tuned in settings; videos never show them.

## Open (owner to decide)

1. **Numbers:** bonus table **confirmed by owner 2026-11-11** (XP 10/20/30). Still my starting values: 30 m range, 4-player cap, 120-minute daily cap.
2. **Tether Node collects:** **decided 2026-11-11: no bonus** (owner).
3. **Extras:** **decided 2026-11-11:** X1, X2, X3, X5 yes; no truce (X4) (owner).
4. **Bluetooth:** **decided 2026-11-11: in the first build (W2)**, required for the bonus (owner).
5. **Truce:** **decided 2026-11-11: none**, linked or not (owner).
6. **Group node yields** x3 / x5 / x8 and the 5/25/100/500 km milestones are my starting values.
