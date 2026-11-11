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

**AR is a view, never a gate.** Every action in the game can be done from the map or a 3D view.
AR shows the same things life-size, in the camera. The server never knows or cares which view
was used, so there is nothing to cheat with and nothing to balance.

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

**Decided (owner, 2026-11-11): no gameplay advantage, looks only.** Fights, harvests and builds are equal.
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
5. **Base design (phase 4)** uses the rule above: GPS spot + server hex-slot layout, built in the
   holo table, shown life-size in AR.
6. **Later: AR sees more** (section below), after bases.

## Later phase: AR sees more

Owner, 2026-11-11: AR stays looks-only, but later AR players get to **see more** than the map shows.
The rule that keeps it fair: what only AR shows gives **no resources, no XP and no combat information**.
It is story, spectacle and collection, never an edge in a fight or a harvest.

Ideas to pick from when this phase comes:

1. **Signal echoes.** Faint traces left in the Undersignal, seen only through the camera near nodes
   and TNs. Tapping one plays a short lore fragment and adds it to a CODEX on your profile, with a badge
   for each set. No rewards beyond the codex and badges. (Never a hint of a seventh power.)
2. **The sky in a fight.** Drone strikes, intercepts and shield hits play out above you, life-size,
   when they happen near you. The map shows the same result as an icon.
3. **Living bases.** Faction bases and TNs show extra life in AR: link lines pulsing with what flows
   through them, faction colours on the structures, idle animations. The holo table shows the same base,
   only plainer.
4. **AR photo mode.** Frame your TN or base in the real street and share it as a card (uses the
   existing share sheet).

Everything here is extra to look at: a non-AR player misses the show, never a move.

## Open (owner's choice)

- ~~Bases on non-AR phones~~: **decided A, holo table**, available on AR phones too (owner, 2026-11-11).
- ~~AR advantage~~: **decided, looks only, no gameplay bonus** (owner, 2026-11-11).
- **AR sees more:** planned below as a later phase (owner, 2026-11-11).
- ~~Use AR view setting~~: **decided yes**, AR phones can pick the holo table (owner, 2026-11-11).
