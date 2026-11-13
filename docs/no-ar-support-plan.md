# Plan: play Tether Rise on phones without AR

Written 2026-11-11. **Approved by the owner 2026-11-11.** Owner picked option A (holo table) on 2026-11-11, and AR phones can pick it too. In the API repo docs;
rules then go into `GAME_DESIGN.md` (Field and AR).

## Why

The owner's test phone can't install Google Play Services for AR ("Your device isn't compatible").
Today the Android build marks ARCore as **Required** (`Assets/XR/Settings/ARCoreSettings.asset`,
`m_Requirement: 0`), so Google Play hides the game from every phone like it. The iOS side is already
Optional (`ARKitSettings.asset`, `m_Requirement: 1`), and every iPhone that runs the game has ARKit anyway.
If nothing changes, a large share of Android phones in the launch region can't even see the game in Play.

## The rule

**AR is never forced** (owner, 2026-11-13). The whole game can be played from the map or a 3D view: nobody has to
open the AR view to progress, build, fight, defend or join. Beyond that AR is free: it may give a gameplay
advantage and may have features only AR has (see "Does AR give an advantage?"). Until such a feature is built the
server still does not know which view was used; one that needs to know says so in its own plan.

## What already works without AR

- The map is the game: harvesting, map drops, the TN panel, missions, drones, node contests.
- Hacking a base is planned as a 2D hex-path puzzle, so it needs no camera.
- **Only TN placement needs AR today** (Build TN here opens AR placement). The server spot comes
  from GPS anyway, so building from the map calls the same server endpoint.

## Bases (phase 4), the hard part

Base design is still **Open**, which is good news: we can design bases so they never depend on AR,
instead of bolting a fallback on later.

**Design rule for bases:** a base stands on a **GPS spot** (a map pin), and its layout is a
**hex grid of slots owned by the server** (core in the middle, modules and defences in rings
around it). Layout never depends on where a real floor or table is. Every view draws the same grid.

Options for how non-AR players see and build a base:

| | Option | How it plays | Cost |
|---|---|---|---|
| **A (recommended)** | **Holo table** | Tap the base on the map → a 3D view of the base on a dark hex platform, as if projected on a table. Drag to orbit, pinch to zoom, tap a slot to build or upgrade. AR players get the same model standing on the real ground, and can **choose the holo table instead** at any time (a VIEW: AR / HOLO TABLE switch on the base screen, and the Use AR view setting as the default). | Medium. One 3D base model serves both views; the holo table is just a camera orbiting it. |
| B | Flat base plan | Tap the base → a 2D top-down hex plan, like a blueprint. Build by tapping slots. | Low, but bases feel much less alive for non-AR players. |
| C | AR-only bases | Non-AR players can join factions and fight, but can't build or manage bases. | Lowest, but splits players into two classes and blocks Wardens with older phones. Not recommended. |

The holo table can be built first for **your own TN** (a "View TN" 3D view from the map), so the
same view is ready and tested before bases arrive.

## Does AR give an advantage?

**Changed (owner, 2026-11-13): an AR advantage is fine, and so are AR-only features, as long as AR is never
forced.** Each advantage or AR-only feature is its own decision in the mobile repo's `docs/GAME_DESIGN.md`; the
first asked for is attacking a base from the AR view. What follows is the earlier decision, kept for the record.

**Was decided (owner, 2026-11-11): no gameplay advantage, looks only.** Fights, harvests and builds are equal.
AR players get the wow factor: life-size pylons and bases, and later an AR photo mode for sharing.
A small AR bonus (for example extra XP) was considered and is not recommended: it punishes players
for the phone they own, and in the launch region many phones are not on Google's list.

## Detecting AR and telling the player

1. **ARCore set to Optional** in Unity XR settings (Android). Play stops filtering the game.
2. At startup the game runs `ARSession.CheckAvailability()` once and caches the result:
   - **Supported and installed:** AR toggle shows on the field screen.
   - **Supported but needs install:** the toggle shows; the first tap offers Google's install prompt.
     Never at startup.
   - **Unsupported:** the toggle is hidden. Build TN here builds straight from the map. One calm
     line, once, in Settings: "AR view isn't available on this phone. Everything else works on the map."
3. Replace today's message ("Install or update Google Play Services for AR") which is wrong for
   unsupported phones and sends players to a store page that says "not compatible".
4. **Settings → Use AR view (on/off)** for AR phones too, for battery or comfort. Off means the
   phone plays exactly like a non-AR phone.
5. The first-TN walkthrough skips the "place it in AR" step on non-AR phones.
6. Store listing: say AR is optional ("AR view on supported phones"). Keep it country-neutral.

## Milestones (build and test on the owner's phone after each)

1. **ARCore Optional + availability check.** Hide the AR toggle on unsupported phones, fix the message,
   add the Use AR view setting. Check Play no longer lists the phone as incompatible.
2. **Build TN from the map.** Build TN here on a non-AR phone (or with AR off) goes straight to the
   confirm card with the cost and builds at the player's GPS spot, with the same server checks.
3. **Walkthrough branch.** The first-TN walkthrough has a map-only path.
4. **Holo table for TNs.** "View TN" opens the 3D orbit view of your TN (tier, link lines, health ring).
   This is the base view's first version.
5. **Living TNs in AR.** Your TN and other players' TNs get extra life in AR only: link lines pulse
   with packets in the resource colour flowing toward the pylon, the pylon idles (a breathing glow; no turn: the owner dropped it 2026-11-12 so it stands still to walk round),
   and a fresh collection sends a visible burst up the line. The holo table shows the same TN,
   plainer (static lines, no packets). Bases inherit all of this later.
6. **AR photo mode.** A camera button in AR hides the HUD, frames your TN (later your base) in the real
   street, and adds a small TETHER RISE mark and your callsign. SHARE uses the existing share sheet
   (`ShareService`); SAVE writes it to the gallery. No location or country text on the picture.
7. **Signal echoes + CODEX.** See *AR sees more* below. Server places echoes, the AR view shows them,
   the profile gets a CODEX.
8. **Sky battles.** Drone strikes, intercepts and shield hits near you play out life-size in AR.
   Ships with the drone strike build (`drone-strike-plan.md`); if drones are already in, it goes now.
9. **Base design (phase 4)** uses the rule above: GPS spot + server hex-slot layout, built in the
   holo table, shown life-size in AR, with the living look from milestone 5.

## AR sees more (now, owner 2026-11-11)

The owner first set this as a later phase, then asked for it now. AR stays looks-only: what only AR
shows gives **no resources, no XP and no combat information**. It is story, spectacle and collection,
never an edge in a fight or a harvest. A non-AR player misses the show, never a move.

### Living TNs (milestone 5)
- Link lines carry packets: small hexes in the resource colour moving from the node to the pylon, faster
  when the TN collects at full speed and slower as the harvest slows (no numbers shown).
- Low health: the flicker from the field AR plan, plus sparks falling from the pylon.
- Other players' TNs get the same life, in their faction colour (neutral for freelancers).

### AR photo mode (milestone 6)
- Reached from a camera button in AR, only while a TN is in view.
- Picture: the camera frame, the hologram, a thin frame line, the TETHER RISE mark and the callsign.
  Never the place name, coordinates or a country.
- SHARE (share sheet) or SAVE (gallery permission asked the first time only).

### Signal echoes (milestone 7)
- **What:** faint traces in the Undersignal. In AR they are a slow shimmer of hex static about head
  height, near nodes and TNs. On the map they are **not shown** at all.
- **Where:** the server places them like nodes, a few per area, and moves them now and then (the
  Undersignal keeps shifting). `GET /v1/echoes/nearby` returns the ones within the AR view radius.
- **Read:** within 40 m, tap it in AR. The echo plays a short line of text in the narrator's style
  (no voice at first) and fades. `POST /v1/echoes/{id}/read` stores it. The client sends that the AR view
  was open; nothing is gained, so there is nothing to cheat for.
- **CODEX:** a new section on your own profile listing echoes found, in sets of five. A finished set
  gives a profile badge. Locked lines show as static. No XP, no resources.
- **Lore rules:** follow `LORE.md` and the screen rules (never link the six powers, no "AI", no power
  says its name, country-neutral). **Never a hint of a seventh voice**: no "someone walked here before
  you", no "stay free", nothing about unbound freelancers.
- **First set (approved by the owner 2026-11-11):**
  1. "Before the first map, this street was already listening."
  2. "Something counted your steps. It did not lose count."
  3. "A signal folded here and never unfolded."
  4. "Every stake driven into the lattice is felt."
  5. "What decays is not lost. It is taken back."

### Sky battles (milestone 8)
- When a drone strike, an intercept or a shield hit happens near you while AR is open, it plays above
  you life-size: the drone's trail in, the intercept flash, the shield ring lighting up.
- It shows only what the map already shows (an icon and the result). No hidden attackers, no early warning.
- Effects follow the effects restyle plan (`effects-restyle-plan.md`): clean, not childish.

## Built (2026-11-12, not committed yet)

Milestones 1 to 8 are built and on the owner's phone (builds 143 to the last one of the run); the rules are in
`GAME_DESIGN.md` (Field and AR, "Phones without AR"). Notes from building:
- Milestone 3: no first-TN walkthrough existed yet, so it was built from `field-ar-plan.md` milestone 8, with the
  map-only path.
- Milestone 4: VIEW TN on the TN panel opens the default view (AR when it is possible here, else the holo table).
- Milestone 5: AR didn't show other players' TNs yet; it does now (nearest 4 within 300 m), in the owner's faction
  colour, from a new `factionColor` on TN views.
- Milestone 7: echoes are 3 per 400 m cell beside nodes and TNs, seen within 150 m, moved after about a day (all
  admin settings, group `echoes`). The CODEX shows on AR phones, or once a line is found.
- Milestone 9 (bases) is phase 4 and not part of this run.

## Open (owner's choice)

- ~~Bases on non-AR phones~~: **decided A, holo table**, available on AR phones too (owner, 2026-11-11).
- ~~AR advantage~~: **allowed, and AR-only features too, never forced** (owner, 2026-11-13; was looks only, no gameplay bonus, 2026-11-11).
- ~~AR sees more~~: **now**, milestones 5 to 8 (owner, 2026-11-11). First echo set approved.
- ~~Use AR view setting~~: **decided yes**, AR phones can pick the holo table (owner, 2026-11-11).
