# Build prompt: nodes float again on the street (fix)

For the owner's local Claude Code in `D:\Development\Github\ProjectAR`. Mobile main 69d80b8 (build 213). Branch
`fix/ar-floating`. Don't commit to main or push without the owner's go.

## What the owner saw (14 Nov, Malé street, build 213)

Nodes about 20 m ahead looked like they hover over the paving. Lines on screen: `GPS LOCKED ±4 m // FUSED
// FLOOR 1.4 m (5)`, `GEO VPS nd · TRACKING 4.3 m ±1.6 v · 32.4°`.

## What it is not

ARCore Geospatial was **not** placing anything: `GeoSource` takes over only within 3 m and 5 degrees, and the
reading was 4.3 m and 32 degrees. With `UseGeo` false, `ArGroundService` runs the same maths as build 207 (`Looks` is
`m_Height`, no terrain ground, `m_Walked.Step` gets NaN). Floor was found from a plane (1.4 m, no EST). Leave the
Geospatial code alone.

## The cause: far node markers are drawn far too big for where their foot is

Every GPS-placed thing stands at its real distance on the floor. But node markers (and TNs) are then **grown with
distance** (`ArGeoPlacement.StreetSize = Scale(Squeeze(d)) * FarGrow(d)`), and since build 206 nodes also get
`NodeBoost` on top (`ArNodeService` line ~187: `ScaleFor(spot, NodeBoost(spot.Real), ...)`). The owner's decision
was "2.5x bigger **near**" (within 10 m), but `NodeBoost` only starts fading at 10 m and ends at 60 m, so it
multiplies the far growth too. Drawn size against true size:

| distance | before 206 (StreetSize) | now (x NodeBoost) |
|---|---|---|
| 5 m | 1.0x | 2.5x |
| 10 m | 2.0x | 5.0x |
| 20 m | 3.0x | 7.0x |
| 40 m | 5.2x | 7.9x |

A thing drawn 7x too big reads as about 7x nearer (about 3 m instead of 20 m). But its foot sits on the floor line
of 20 m, close to the horizon. To the eye that is a near object hovering at about knee to chest height. Before 206 the
same effect was there but smaller (3x), and the far TN/node look was never checked on a street since the real-distance
change (build 201). Occlusion can't help: depth is ignored past 12 m, so a 20 m node is drawn over nearer real things.

## Owner's later field shots (14 Nov, after this prompt was first written)

- Bases (48 to 54 m, behind a railing, real size) and Tether Nodes (close, in a courtyard) look **good**: the owner
  said so. **Do not change how bases or TNs are sized.**
- In the courtyard shot a rock node far behind the TN, at the building's entrance, hangs at about door height: the
  same oversized-far-node case.
- One GEO line read `VPS ErrorNetworkConnection`, another `VPS no`: no VPS here, as expected. Not part of this fix.

## Fix (resource node markers only)

1. **`NodeBoost` only near (`Holo/ArGeoPlacement.cs`).** 2.5x within 6 m, fading to 1x by 15 m (constants
   `NodeBoostFullM` 6, `NodeBoostEndM` 15). That keeps the owner's "bigger near" and stops it multiplying the far
   growth.
2. **Node markers stop growing in the band where the ground can be read.** In `ArNodeService` (not in the shared
   `StreetSize`, which TNs use and which looks right for them): a node marker's street scale is at most
   `NodeMaxShowRatio` (1.5) times its scale at 6 m out to `NodeGrowFromM` = 40 m. Past 40 m it eases back up to
   today's `StreetSize` value by 100 m, so far nodes are still beacons on the horizon. Salvage drops keep today's
   size. Drawn size against true size for a node: about 2.5x at 5 m, 1.5x at 20 m and 40 m, today's size past 100 m.
3. **A contact shadow for node markers** (architecture.md lists it as missing). Reuse `BaseFootprint`'s soft dark
   disc, about 1.3x the model's footprint, on the marker's own ground, strength from `ArLight` (0.5 night to 0.85 day),
   street only (not up high). Leave TNs as they are.
4. **Up high must not flip on the street.** The first shot showed `FLOOR 1.4 m (5)`: GPS and barometer said 5 m above
   ground on the street, 1 m under `GroundHeight.UpHighM` (6). Crossing it for 3 s swaps every AR thing to the
   up-high scale-model view, which on a street floats. Raise `UpHighM` to 10 and `BackDownM` to 7. Log
   `[ProjectAR] Ground: up high ...` / `back on the street ...` with height and accuracy when it flips.
5. **Testers' check (ADMIN > AR TEST TOOLS only):** a NODE SIZE switch on the AR view, cycling NEW / OLD (today's
   build 213 sizes), so the owner can compare on the spot. Off (NEW) by default, kept in PlayerPrefs.

Update the edit-mode tests that pin `NodeBoost`, and add one for the node size cap (6 to 40 m within 1.5x).

## Check and report

- Edit-mode tests green; build; captures as usual (`holo-*` nodes near and far).
- Ask the owner to stand where the screenshots were taken (the street with the flags, the courtyard): nodes 10 to
  40 m away should sit on the paving, smaller than now; near (under 6 m) they stay 2.5x; far ones (100 m+) still show
  as beacons; bases and TNs unchanged. Flip NODE SIZE to compare.
- Update `docs/architecture.md` (AR ground: the size rule and the contact shadow) and the `CLAUDE.md` gotcha:
  "a node drawn bigger than its distance says reads as nearer and floating; node markers grow only past 40 m".
