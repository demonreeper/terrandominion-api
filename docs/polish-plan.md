# Plan: polish pass over the current build

Approved 2026-11-09. Covers what is built through the drone strike (mobile `6125d70`,
API `f2819a8`): main menu, Field Ops (map, list, AR), Tether Node panels, Inventory (Bag, Craft),
alerts, Settings, the AR build flow, and the API that feeds them. Where this plan and
`GAME_DESIGN.md` (in the mobile repo's docs/) disagree, `GAME_DESIGN.md` wins.

**How the audit was done:** the HUD captures in `References/temp/hud/` (drawn by the same UI code as
the phone), the `Ui/`, `Audio/`, `Holo/` and `Net/` code, and the API's `apperr`, notices, settings and
harvest code. Nothing was run on the phone, so feel and timing items need a phone check.

**Stays clear of sign-in/push (S0–S3):** this plan does not touch the sign-in screen, Settings
"// MORE" (it becomes Account), push code or `ServerSession`. Milestone 3 adds a main menu row, so if S1 is still open, rebase on it first. The style kit in milestone 0 is ready for the
sign-in screen to use, but nothing waits on it.

## Decisions (owner, 2026-11-09)

1. **Font: bundle one open-licence font.** Chosen: **Exo 2** (SIL Open Font License, Google Fonts).
   It is squared and techy enough for the look but stays readable at small sizes, unlike condensed
   fonts such as Rajdhani or display fonts such as Orbitron. It also covers the accented Latin
   letters in place names (MALÉ). Use static TTFs, because Unity's legacy `Text` can't use variable fonts: Regular,
   Medium, SemiBold and Bold.
2. **Dev text stays on while connected to the dev server.** The server host, `MAP WIDTH … // RING …`,
   the GPS source and the AR `DIAG` button show whenever the app talks to a dev server, admin or not,
   and are hidden on a production server.
3. **Build MY TETHER NODES in this pass**, with its menu row, map button and related details (milestone 3).
4. **Breaking API changes are fine** while the owner is the only player (milestone 7).

## Findings

### Text readability

| # | Where | Finding |
|---|---|---|
| T1 | All screens | About 25 labels are 13–19 px on the 1080×1920 reference (TN panel uses 16–19 px for most of its text; `UiBuilder.cs` has 13, 14, 15 px). On a phone at arm's length, and in sunlight outdoors, these are hard to read. |
| T2 | Field Ops top bar | Resource names under the numbers (CRYSTAL, ALLOY…) are tiny and in the resource colour at low alpha. STORED values are small and colour-only with no labels. |
| T3 | TN panel | Hex action buttons shrink text to fit (DISMANTLE is visibly smaller than UPGRADE). "Move within 40 m to see what it collects" is small grey text. |
| T4 | Map | Map attribution and the scale line sit at ~15 px over the map; the "-" zoom glyph is a thin hyphen. |
| T5 | Dim colour | `Dim` (0.78/0.86/0.92 at 85 %) on navy is fine; the darker greys used for subtitles and the Settings "MORE" text are below comfortable contrast. |
| T6 | Copy style | Messages mix styles: "No GPS fix yet." vs "Too close to a Tether Node, move 10 m away" (no full stop), "Crafted." vs "Built and equipped.", ALL CAPS labels next to sentence-case toasts in the same line. |
| T7 | Stale copy | Settings: IN-GAME MUSIC says "While playing in AR" (it plays on the whole field screen); GAME SOUNDS says "Beacon, modules, alerts" (Beacon and modules were removed). AR pause menu subtitle "Your Tether Node stays anchored while you are here" reads like the old Beacon Lab. |
| T8 | Names | The same thing has different names: ENERGY CRYSTAL / CRYSTAL, DATA CACHE / DATA; BAG (Field Ops chip) / INVENTORY (menu and screen); STORED (top bar) / uploaded (design) / UPLOAD (TN button). |

### Layout consistency

| # | Where | Finding |
|---|---|---|
| L1 | AR build HUD vs Field Ops | The AR build screen (`hud-tn-confirm`) has its own resource strip: different order (crystal, **credit**, alloy, data vs crystal, alloy, data, credit), different numbers (spendable = carried + stored, vs carried on Field Ops), different size and position. |
| L2 | TN panel map vs AR | Your TN panel looks different in the two views: map shows "TIER 1" with a LINKS row, AR shows "TIER 1 // 1 LINKED" with no links row; HELD moves; the button row changes count and spacing. One builder should draw both. |
| L3 | Tabs | Field Ops puts AR / BAG / LIST / MAP chips under the stats card; Inventory puts BAG / CRAFT in the header beside the title. BAG on Field Ops looks like a view tab but opens a different screen. |
| L4 | Button shapes | Hexagon buttons (BACK, ALERTS, TAKE, TN actions, zoom), rectangles (SCAN AREA, BUILD/CANCEL, craft rows, pause menu) and toggle chips are mixed with no rule. Settings has two more styles (hex ON toggles on tracks, and a rectangular ON/OFF for haptics). |
| L5 | Disabled state | Disabled craft buttons are just text with no plate, so UPGRADE (needs level 5) and CRAFT (can't afford) look like labels, with no reason given. Disabled hex buttons only dim their label. |
| L6 | Build confirm | BUILD stays enabled while the panel says "A Tether Node stands too close to this spot." (`RenderTetherConfirm` only disables it while busy.) |
| L7 | Headers | ALERTS is an amber hex top-right on Field Ops only; Inventory and Settings have nothing there. The kicker line differs per screen (`// FREELANCE NETWORK <host>`, `// FIELD KIT`, `// SYSTEM CONFIG`). |
| L8 | Empty space | Main menu, Inventory and Craft leave the bottom 35–45 % empty; the TN panel on the map has a large gap between LINKS and HELD. |
| L9 | Main menu | FACTIONS shows "LOCKED" in the build capture but the code writes "LVL x / 20"; the design wants "Unlocks at level 20" with progress. Menu numbering (01–05) will shift when MY TETHER NODES arrives. |
| L10 | Status line | The one-line status at the very bottom of Field Ops (below the admin row) is easy to miss and is covered by the thumb. |

### Feedback cues

| # | Finding |
|---|---|
| C1 | **One toast slot, no queue.** `FieldOpsController.Toast` overwrites the text and shows it for a fixed 4 s, so two quick results (harvest then level-up, or a reject then a notice) lose the first. Success, warning and error look the same. |
| C2 | No busy cue: SCAN AREA, TAKE, COLLECT, UPLOAD and crafting ignore taps while waiting (`m_Busy`) but nothing on screen changes. |
| C3 | Map shows nothing while tiles load (blank panel in `hud-field-ops`). |
| C4 | Harvest gives a toast, a short haptic and the generic "Online" chime, the same sound as a default alert. Numbers in the top bar jump with no count-up. AR has a harvest effect; the map has none. |
| C5 | Level-up is only "LEVEL n!" appended to the toast plus the Upgrade arpeggio (also used for gear upgrades). |
| C6 | Cooldowns are guessed: after a harvest the client assumes 300 s (`CooldownGuess`) until the next scan. |
| C7 | Weapon buttons on another player's TN show "PULSE 0" with no hint that you need to craft it. |
| C8 | Alerts: a contest challenge (`contest`) and `contest_won` fall to the default chime with no haptic; `spike_result`/`drone_result` always play Boom, even when the target survived. |

### Audio

| # | Finding |
|---|---|
| A1 | 15 synthesised effects; several events share one: "Online" covers harvest, collect, store, pick-up and generic alerts; "Upgrade" covers level-up and gear/TN upgrades; "Reject" covers every error. |
| A2 | No sound for opening panels, switching map/list/AR, a cooldown finishing, entering harvest range, or a contest timer. |
| A3 | Gains are set per clip by ear (0.28–0.8); no shared loudness target, and nothing ducks the music under Boom or alarms. |
| A4 | No limit on repeats: several alerts arriving together each play a sound. |

### Visual effects

| # | Finding |
|---|---|
| V1 | Panels and screens appear instantly with no transition; buttons use Unity's default colour tint (weak press feedback on the dark navy). |
| V2 | Map markers: only the under-attack pulse animates. In-range nodes, the selected node and depleted nodes differ only by colour. |
| V3 | No resource "fly to bar" or count-up on gains; no flash on damage to your TN panel's health bar. |

### API (and its match with the app)

| # | Finding |
|---|---|
| API1 | Harvest response has no recharge time, so the app guesses 300 s (C6). Return `readyInSec` (and `nodeReserve` is already there). |
| API2 | Vague errors: `TetherCost` 1602 "Not enough resources to do that." doesn't say what is short; `OutOfRange` 1712 "Move closer." has no distance; `NoCells` 1710 says "an energy cell" though cells per use is a setting. |
| API3 | Notice kinds are a free string the app maps by hand; `contest` and `contest_won` are missing from the app's map (C8). Add a `severity` (info / warning / danger) to notices so the app picks sound and haptic without a list, and list the kinds in `docs/API.md`. |
| API4 | Notice text: results say "Your spike went off." / "Your drone hit its target." without the damage done; attack notices don't say who or with what. |
| API5 | Copy style differs from the app (see T6); faction errors say "leader" while the factions discussion uses Commander (leave until that is decided). |
| API6 | New players get `Freelancer-1234` names while the menu card's look (E7 avatar, "ECHO-7" in captures) suggests call signs. Not a bug; note for the sign-in phase. |
| API7 | The app holds its own copy of some numbers (`DefaultHarvestRangeM` 40, `m_TnMinSpacingM` 10, upgrade cost text) as fallbacks until `/v1/settings` loads; fine, but the "Tier 2: four links" toast is hard-coded copy. |

## Milestones

Each one is buildable and testable on its own, in order. Milestone 0 first because the rest use it.

0. **Style kit.** One `UiStyle` (in `Ui/`) with the type scale, colours and button kinds, used by every
   `UiBuilder.*`:
   - Type scale on the 1080×1920 reference: Title 72, Heading 36, Body 26, Label 22, Small 20.
     **Nothing under 20 px.** Numbers in the top bar 40.
   - Colours as named tokens (Accent, Amber, Danger, Success, Dim, Muted) with Muted raised to pass
     4.5:1 on the panel colour. Resource colours in one place.
   - Three button kinds: **Primary** (filled rectangle: SCAN AREA, BUILD, CRAFT), **Secondary**
     (outlined rectangle: CANCEL, pause menu), **Hex action** (round actions on the map and panels).
     Each has pressed (scale 0.96 + brighter plate) and disabled (dark plate, 40 % label, reason text)
     states. Toggle chips are one style everywhere (Settings haptics and track toggles use it too).
   - Font: Exo 2 TTFs in `Assets/Resources/Fonts/` with `OFL.txt` beside them, loaded by
     `UiBuilder.ResolveFont` (one per weight: titles SemiBold, numbers Medium, body Regular, primary
     buttons Bold). This is the one asset exception to "everything built in code", like the music;
     add it to the mobile `CLAUDE.md`. Settings gets a small "Font: Exo 2, SIL OFL" credit line.
   - `HudLayoutCapture` fails the build step if any active Text is under 20 px or overflows its rect
     (text width > rect width), so regressions show before install.
1. **Text and names.** Apply the scale to every screen (T1–T5). Fix stale copy (T7). One name per thing
   (T8): CRYSTAL, ALLOY, DATA, CREDIT everywhere; INVENTORY not BAG; STORED for uploaded resources, with
   UPLOAD as the action. Copy rule: labels and buttons in caps without full stops; messages are
   sentences with full stops. Dev text (host, map scale, GPS source, DIAG) shows only on a dev
   server: the API's `/v1/settings` returns `environment` ("dev" or "prod") and the app reads it
   (before the first answer, a host starting `td-dev` counts as dev).
2. **Layout consistency.**
   - One shared top bar component used by Field Ops and the AR build HUD (L1): same order, same
     carried / stored meaning.
   - One TN panel builder for map and AR (L2); keep the panel's height fixed and fill the LINKS gap.
   - Field Ops header: BACK left; ALERTS and INVENTORY as hex buttons right and MY TNs (milestone 3). The view chips become AR / LIST / MAP only (L3, L7). Inventory keeps BAG / CRAFT as chips
     under its header, in the same spot and style as Field Ops' chips.
   - Same header on every screen: BACK, kicker, title, right-side actions.
   - BUILD disabled with the reason when the spot is blocked (L6).
   - Status line moves above SCAN AREA as the toast area (L10, see milestone 4).
   - Main menu: FACTIONS right label "UNLOCKS AT LVL 20" with progress; use the empty space for the
     operative card's stats (carry, stored, TNs) rather than stretching rows (L8, L9).
3. **My Tether Nodes** (`GAME_DESIGN.md` › Menus and screens, row 3).
   - **API:** `GET /v1/tethers/mine` returns every TN you own, wherever you are: id, name, tier,
     lat/lon, health/max, expiresAt, held resources, links (node id, resource), and state (jammedUntil,
     spike fuse end, drone inbound ETA, under attack in the last 10 min). One call, no range check,
     because it's your own TN. `/v1/me` keeps `tethersOwned` for the menu row.
   - **Main menu:** new row 03 MY TETHER NODES, "Your Tether Nodes: health, time left and what they
     hold." Right label "2 OF 3", or amber "UNDER ATTACK" / "JAMMED" when one is. FACTIONS becomes 04,
     SETTINGS 05, QUIT 06.
   - **Screen** (same header as the others: BACK, `// NETWORK`, MY TETHER NODES): one card per TN
     with name and tier, a health bar, time left (amber under 6 h, red under 1 h), held resources with
     icons, links as resource dots, distance and direction from you, and a state badge (UNDER ATTACK,
     JAMMED 12 MIN, SPIKE 4:30, DRONE INBOUND 0:18). Cards are sorted by urgency (under attack, then
     spike or drone, then jammed, then time left). Buttons: **FIND ON MAP** (opens Field Ops on the
     map, centred on the TN, its panel open; this is the pan from milestone 0 of crafting/combat),
     and **VIEW IN AR** when within 40 m. COLLECT, UPLOAD and DISMANTLE stay in the TN panel, which
     checks range. Empty slots show "Build a Tether Node next to a resource node in the field." with an
     ENTER FIELD button.
   - **Field Ops header:** a MY TNs hex button beside ALERTS and INVENTORY, with a red dot when one
     of yours is in trouble.
   - **Related details:** tapping an alert about a TN opens FIND ON MAP for it. The screen refreshes
     every 15 s while open, and the countdowns tick locally. A TN that expires or is destroyed leaves
     a greyed card for 1 h saying what happened, then drops off. On the map, your own TN markers get
     the same state ring as the cards (red attack pulse, static for jam, a fuse arc for spike).
   - Capture: add `hud-my-tethers.png` (three TNs in different states) and `hud-my-tethers-empty.png`
     to `HudLayoutCapture`.
4. **Feedback cues and the message queue.**
   - `ToastService` with a queue: up to 2 visible, newest on top, 2.5 s for info and 4 s for errors,
     duplicates merged ("×3"), kinds Success / Info / Warning / Error with colour and a small icon.
     Used by Field Ops, Inventory and Settings (replaces the three `Toast`/`MenuToast` copies).
   - Busy state on every server button: label changes (SCANNING…, TAKING…) and a thin progress sweep.
   - Map loading shimmer until the first tiles arrive; "Map offline" note if they fail.
   - Cooldown ring on node markers and a countdown on TAKE using the server time (needs API1).
   - Disabled craft and weapon buttons say why ("NEED 90 CREDIT", "CRAFT IN INVENTORY") (L5, C7).
5. **Audio pass.**
   - New effects in `SoundSynth`: Harvest (one per resource, same family, pitch per resource), Collect /
     Store (coin-like), LevelUp (longer fanfare), PanelOpen / PanelClose (soft), ViewSwitch, InRange
     (quiet tick when a node comes into range), CooldownReady, ContestStart, Alarm (danger alerts).
   - Event to sound table in one place (`AudioCues`), so Upgrade, Online and Reject stop doubling up.
   - Loudness: normalise every clip to one peak target, then set per-cue gain in the table.
   - Duck music by ~6 dB for 1.5 s under Boom, Alarm and LevelUp. Same cue can't repeat within 150 ms;
     several alerts at once play the most severe one only.
   - Fix the alert mapping (C8) using the API `severity` (API3).
6. **Visual effects.**
   - Panels slide/fade in over 150 ms; screens cross-fade 120 ms. Off when the phone's "remove
     animations" setting is on.
   - Gains: "+20" floats from the node marker to its top bar number, which counts up and flashes in the
     resource colour. Level-up: the menu card's level bar fills, the top bar flashes, LevelUp sound and a
     medium haptic.
   - Markers: in-range nodes breathe slowly, the selected one has a rotating ring, recharging ones show
     the cooldown ring, depleted ones are hollow.
   - TN panel health bar flashes red and shakes on damage; jam shows a static overlay on the panel.
7. **API polish** (`terrandominion-api`, can run in parallel with 1–6; its `/v1/tethers/mine` is part of milestone 3). Breaking changes are fine
   while the owner is the only player (decided 2026-11-09); app and API ship together, with no
   compatibility shims or old fields kept.
   - Harvest returns `readyInSec` (API1); the app drops `CooldownGuess`. Other actions that start a
     timer (jam, spike fuse, drone flight, contest) return their end time the same way.
   - Errors carry data, not only text (API2): `{code, message, data}` where `data` holds what the app
     needs to say it its own way, e.g. `{"short": {"alloy": 30}}` for costs, `{"distanceM": 62,
     "rangeM": 40}` for range errors, `{"need": 2, "have": 1}` for cells. Codes may be renumbered
     into tidy groups if useful; `ApiModels.ApiError` follows.
   - Notices become `{id, kind, severity, tnId, actor, amount, body, createdAt}` (API3, API4): severity
     info / warning / danger picks the sound and haptic; actor and amount let results say
     "Aishath's spike hit your Tether Node for 40". `docs/API.md` lists every kind.
   - One copy style for every message (API5): sentences, full stops, "Tether Node" spelled out, numbers
     with units.
   - Rename fields to match the app's names where they differ (for example uploaded vs stored), so
     one word means one thing end to end (T8).
8. **Phone check and tuning.** Outdoors in daylight: read every screen at arm's length; tune sizes,
   contrast and volumes; record before/after captures in `References/temp/hud/`.

## Not in this plan

Sign-in screen, Account settings and push (their own plan); new game mechanics; localisation (text
stays in code, but milestone 1 gathers player-facing strings into one `Copy` class so it is easier later);
replacing the synthesised sounds with recorded ones.
