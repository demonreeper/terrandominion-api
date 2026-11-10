# Plan: drone strike, Drone Bay tiers and node seeding

The phase after crafting and first combat (`docs/crafting-combat-plan.md` in the mobile repo) and the
polish pass (`docs/polish-plan.md`). Approved 2026-11-10. Where this plan and
`GAME_DESIGN.md` (mobile docs/) disagree, `GAME_DESIGN.md` wins; the first milestone built updates its
Combat and loss, Tether Nodes and Nodes sections to match this plan (Drone Bay tiers, intercept, TN
shield, cells per strike, node seeding) and drops "Drone charges" from its Open decisions. Every number is a starting value in `td.settings` or `td.gear_types`,
to be tuned after testing. Breaking API changes are fine while the owner is the only player.

## Where we start

A first drone strike is already built (API migration `0019_drone_strike.sql`, `service/drones.go`;
mobile `FieldOpsController.LaunchDrone`, the DRONE hex on the TN panel):

- `drone_bay_mk1` is craftable at level 5 (40 alloy, 20 crystal) and was set to **1 km, 15 damage**.
- `POST /v1/tethers/{id}/drone` checks the TN attack rules, GPS, range, one drone in flight, spends
  `CellsDrone` (1) energy cell, and stores a `drone_strikes` row that lands in the world tick.
- Flight is `max(DroneMinFlightSec 20, distance / DroneSpeedMps 15)`. The owner gets a
  `drone_inbound` notice; both sides get a result notice. The TN panel and the AR pylon count down.
- A drone bay widens how far you see other players' TNs (`NearbyTethers`), so you can target them.
- The polish pass already adds the drone ETA to `/v1/tethers/mine`, end times on timer actions,
  notice `severity`, and damage in result notices. This plan reuses those and does not repeat them.

## Decisions (owner, 2026-11-09)

- **Drone Bay Mk I range is 150 m**, damage 15. 1 km moves to Mk III.
- **Tier ladder** as in the table below (Mk II and Mk III "ok for now"; Mk IV given by the owner).
- **Mk IV does 100 damage.** It costs 5x Mk III, so the one-hit power is earned.
- **Intercept is partial, by drone level:** the higher the drone's Mk, the less an intercept stops.
- **A Tether Node shield** gives level-based defence against drones the same way, so a Mk IV can't
  destroy a shielded TN in one hit.
- **Cost per strike:** 1 energy cell for Mk I to Mk III, **3 for Mk IV**. No separate drone charge
  (removed from Open decisions in `GAME_DESIGN.md`).

## Numbers

| Gear | Level | Range | Damage | Drones in flight | Cells per strike | Craft / upgrade cost |
|---|---|---|---|---|---|---|
| Drone Bay Mk I | 5 | 150 m | 15 | 1 | 1 | 40 alloy, 20 crystal (craft) |
| Drone Bay Mk II | 10 | 500 m | 20 | 1 | 1 | 120 alloy, 60 data, 20 credit (upgrade) |
| Drone Bay Mk III | 15 | 1 km | 25 | 2 | 1 | 250 alloy, 120 data, 60 credit (upgrade) |
| Drone Bay Mk IV | 20 | 5 km | 100 | 5 | 3 | 1250 alloy, 600 data, 300 credit (upgrade, 5x Mk III) |

Flight stays 15 m/s with a 20 s minimum: Mk I always 20 s, Mk II up to ~33 s, Mk III up to ~67 s,
Mk IV up to ~5.5 min. Cells per strike live in each bay's `stats.cells`, replacing `CellsDrone`. Each
launch costs the bay **5 durability** (`DroneLaunchWear`), repaired with alloy like any gear.
Slipstream Vanguard's faster drones wait for factions v1 (a speed multiplier per faction).

### Intercept (defender)

The TN's owner, within 40 m, spends 1 energy cell (`CellsIntercept`) to intercept one inbound drone
before it lands. An intercept cuts that drone's damage by a share set by the drone's Mk; one intercept
per drone.

| Drone | Mk I | Mk II | Mk III | Mk IV |
|---|---|---|---|---|
| Damage stopped | 100% (shot down) | 75% | 50% | 25% |

### Tether Node shield

A new TN fitting. The owner installs it within 40 m of their own TN from the TN panel; it lasts as long
as the TN, and a higher Mk replaces a lower one. It comes in four Mks, at the same player levels as the
drone bays, crafted on the Craft tab as an item.

| Shield | Level | Craft cost |
|---|---|---|
| TN Shield Mk I | 5 | 30 crystal, 20 alloy |
| TN Shield Mk II | 10 | 80 alloy, 40 data |
| TN Shield Mk III | 15 | 160 alloy, 80 data, 40 credit |
| TN Shield Mk IV | 20 | 800 alloy, 400 data, 200 credit (5x Mk III) |

**It protects against every attack on the TN, partly, by level** (owner, 2026-11-09). The share it
stops depends on the shield's Mk against the attack's Mk: 30% when they match, 15% more for each Mk
the shield is above the attack, 15% less for each Mk below, never under 5% or over 75%.
EM Pulse, Signal Jammer and Overload Spike count as Mk I until they get tiers of their own.

| Shield vs attack | Mk I attack | Mk II | Mk III | Mk IV |
|---|---|---|---|---|
| Shield Mk I | 30% | 15% | 5% | 5% |
| Shield Mk II | 45% | 30% | 15% | 5% |
| Shield Mk III | 60% | 45% | 30% | 15% |
| Shield Mk IV | 75% | 60% | 45% | 30% |

- **Damage** (drone, EM Pulse, Overload Spike) is cut by the share. **Signal Jammer** time is cut by
  the share (a 30 min jam on a Mk II shield lasts 17 min).
- **No one-hit kill:** a single hit of any kind can't take a shielded TN below 1 health, even if decay
  has already lowered it. A second hit can destroy it.
- Intercept and shield stack: damage = attack damage x (1 - intercept share) x (1 - shield share),
  rounded up. Examples: Mk IV drone (100) on a Mk IV shield 70, intercepted 53; on a Mk I shield 95,
  intercepted 72. EM Pulse (25) on a Mk II shield 14. Overload Spike (40) on a Mk IV shield 10.
  Mk I drone (15) on a Mk I shield 11.

## Milestones

Each one is buildable and testable on its own, in order. Milestone 0 doesn't depend on drones and can
go first: drone strikes and the tuning in milestone 8 need a map full of TNs and nodes to aim at.

0. **Auto seeding of nodes** (owner, 2026-11-10). Today nodes exist only where an admin scattered test
   nodes from Field Ops, plus respawns of depleted ones. The server now keeps the map stocked by itself.
   - **Land and walkable paths.** A one-time import (`scripts/import-osm.ps1`, osm2pgsql or ogr2ogr)
     loads land polygons and walkable ways (footways, paths, residential and service roads, squares,
     parks) from the same `maldives.osm.pbf` used for the map tiles into `td.osm_land` and
     `td.osm_walkways`. Re-run it when the map is rebuilt. Seeded nodes go only on land, within
     **15 m of a walkable way** (`SeedSnapM`), never in water, inside buildings or on runways.
   - **Cells.** The world is split into **250 m square cells** (`SeedCellM`), each with a target of
     **4 nodes** (`SeedPerCell`, owner 2026-11-10: "4 per 250 and then we see"). Cells with too little
     walkable land get fewer (no more than one node per 40 m of path). Nodes keep at least **25 m**
     apart (`SeedMinGapM`) and **10 m** from a TN. The world tick tops a cell up when it falls under
     target (for example after respawns move nodes away).
   - **When (owner: a mix of both).** At go-live an admin command pre-seeds every cell over land in the
     test area (Malé, Hulhumalé, Villimalé; a list of areas in `td.seed_areas` so more islands can be
     added later). Everywhere else, a cell is seeded the first time a player's scan touches it.
   - **Mix.** Crystal 4 : alloy 3 : data 2 : credit 1, the same weights and yields as the scatter tool,
     regrowth at the normal rate (not the test 0.5 an hour). Names from the existing label sets
     ("Crystal Seam K-07"). `source = 'seed'`, with the cell id in `source_ref`.
   - **Rare roaming nodes** (in this phase, owner 2026-11-10; already decided in `GAME_DESIGN.md`, not built): the same seeder places
     **1 rare node per 4 km² a day** (`RarePerKm2Day` 0.25) at a random seeded spot, lasting 2–4 h, at
     **3x yield**. Veil Cipher's scanner bonus waits for factions v1.
   - **Admin.** Field Ops admin row: RESEED CELL (clears and re-seeds the cell you stand in) and a
     seed count readout. SCATTER and CLEAR test nodes stay for testing.
   - **First run:** pre-seed the test area. Existing `scatter` nodes **stay for now** (owner); they
     **don't count** toward a cell's 4 (owner, 2026-11-10); only `seed` nodes do. Seeded nodes still keep
     the 25 m gap from them. CLEAR still removes them when wanted.
   - API: a seeding pass runs inside `GET /v1/nodes/nearby` only for unseeded cells (small, one
     transaction per cell, `FOR UPDATE` on a `td.seed_cells` row so two scans don't double-seed).
     Settings above are public except the rare ones. Pre-seeding is `POST /v1/admin/seed` (area id),
     one transaction per cell.

1. **Mk I to 150 m.** API migration sets `drone_bay_mk1` stats to `{"drones": 1, "range_m": 150,
   "damage": 15}` (or the agreed damage). Range errors already carry `distanceM` and `rangeM` after the
   polish pass; the DRONE button on an out-of-range TN is disabled with "OUT OF RANGE 220 / 150 M"
   instead of failing after the tap.
2. **Drone Bay tiers.** Add `drone_bay_mk2`, `drone_bay_mk3` and `drone_bay_mk4` to `td.gear_types` with
   `upgrades_to` links, so the existing Craft tab upgrade flow covers them (no new endpoint). Add
   `DroneLaunchWear` and apply it in `LaunchDrone`. Inventory shows each tier's range, damage and drone
   count; a disabled upgrade says what it needs ("NEEDS LEVEL 10", "NEED 60 DATA").
3. **Aiming on the map.**
   - With a drone bay equipped, Field Ops map shows a faint **range ring** around you at the bay's
     range (toggle in the map controls, on by default). TNs inside it that you may attack get a small
     crosshair badge.
   - Tapping such a TN opens its panel at any distance; DRONE is the only weapon live beyond 40 m.
     The panel shows distance and flight time before you launch ("180 M // 20 S"), and LAUNCH asks for
     one confirm tap because it spends a cell.
   - Zooming out past the ring keeps it visible (it uses the pinch-zoom from crafting milestone 0).
4. **Drone in flight.**
   - API: `drone_strikes` stores `from_lat`, `from_lon` (the launch fix) and returns `launchedAt`,
     `arrivesAt`, `from` and `to` in the launch response, in `/v1/tethers/mine` and on the attacker's
     own `GET /v1/drones` (their strikes in flight and the last 10 min of results).
   - Map, attacker: a drone marker moves along a dashed line from the launch point to the target with
     an ETA, then a hit flash. The player can leave the area; the strike still lands.
   - Map, defender: the drone marker approaches your TN from its launch direction (the defender sees
     where it came from, not who sent it, until it lands).
   - AR, defender within range: a small drone hologram flies in from the launch bearing and strikes
     the pylon; the health ring drops. Attacker in AR sees it leave.
5. **Intercept.** `POST /v1/tethers/{id}/intercept` (own TN, within 40 m, a drone inbound, 1 cell).
   It sets `drone_strikes.intercepted = true`; the landing applies the drone Mk's share. A Mk I
   intercept voids the strike (`status = 'void'`). Both players get a notice ("Your drone was
   intercepted. 75% of its damage was stopped."). Button INTERCEPT on your TN panel and in AR while a
   drone is inbound, showing the share it will stop, with the same countdown as defuse.
6. **Tether Node shield.** Recipes `tn_shield_mk1`…`mk4` (items in `td.player_items`), a
   `tether_nodes.shield_tier` column, `POST /v1/tethers/{id}/shield` (own TN, within 40 m, uses the item).
   One function (`game.ShieldShare(shieldMk, attackMk)`) is used by drone landing, EM Pulse, spike
   detonation and jam time, with the no-one-hit floor for every damaging hit. TN panel, AR pylon and My Tether Nodes
   card show the shield Mk; attackers see it on the TN panel before they launch, with the damage the
   strike would do ("DRONE 100 // SHIELD MK II // 70").
7. **Feedback.** Sounds: DroneLaunch (rising whine), DroneInbound (repeating alarm, danger severity,
   replaces the borrowed SpikeArm cue), DroneHit, DroneIntercept, ShieldUp. Haptics: medium on launch, heavy when your
   TN is hit, a tick each second in the last 10 s of an inbound drone. Notices name the damage and
   health left ("Your drone hit the Tether Node for 15. 40 health left.").
8. **Phone check and tuning.** Two phones in Malé: launch at 150 m, intercept, fit a shield, upgrade
   to Mk II and strike at ~450 m; set Mk IV with an admin grant and check a shielded TN survives. Tune
   range, damage, intercept and shield shares, wear and cell costs.

## API notes

- Range errors reuse `OutOfRange` 1712 (it already carries `distanceM`, `rangeM`). New error codes
  after 1731: `NoDroneInbound`
  (intercept with nothing to intercept), `AlreadyIntercepted`, `ShieldNotHigher` (fitting a Mk
  that isn't above the current one). Add to `docs/API.md`.
- New settings: `DroneLaunchWear` 5, `CellsIntercept` 1, `InterceptShareMk1..4` 1.0 / 0.75 / 0.5 / 0.25,
  `TnShieldBase` 0.30, `TnShieldStep` 0.15,
  `TnShieldMin` 0.05, `TnShieldMax` 0.75, `DroneRangeRingDefault` true (public). `CellsDrone`
  is removed (cells move to each bay's stats).
- New notice kinds: `drone_launched` (attacker, info), `drone_intercepted` (both, warning), `shield_fitted` (owner, info). Listed in
  `docs/API.md` with their severity.
- Landing and intercept both lock the strike row (`status = 'flying'` check), so a drone that is
  intercepted in the same tick it lands is resolved once.

## Still to tune

The intercept shares (100/75/50/25%) and the shield rule (30% at equal Mk, 15% a Mk either way,
5–75%), shield costs and levels are Claude's numbers for the owner's rules, taken as starting values
(approved with the plan 2026-11-10). Tune them in milestone 8.

## Not in this plan

Faction speed bonus (factions v1), drones against faction bases and players (with bases), traps,
bounties, PC remote attacks.
