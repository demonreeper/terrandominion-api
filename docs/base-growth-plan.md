# Bases that grow: plan v2 (approved 2026-11-13)

Thread: "Make bases grow with each upgrade" (2026-11-13). Owner asked: bases are still too small and don't grow in
steps; a tower in the middle that grows, with more towers added round it at every tier and upgrade. Owner's reference:
eight castle levels at the same scale (keep grows, ring walls added outward, 2 → 36 towers); "walls or anything you
are free to build, it should be sci-fi, and match the theme". 

Renders in `docs/art-reference/base-v2/`: `tier1..5.jpg`, `growth-strip.jpg`, `lineup.jpg` (same scale),
`modules.jpg` (module towers Mk I to IV), `map-close.png`, `map-icons.png`.
Source: `source/growth.py` (renders), `anim.py` (beam video), `exp.py` (Unity export) (same Blender style and materials as the v3 base renders), `source/icons.py`.

## 1. How a base grows

| What happens | What you see |
|---|---|
| **Tier up** | The **main tower adds a storey** and a bigger crown (the lower storeys stay the same, so it is the same tower getting taller). **A new hex wall ring** goes up outside the last one on a new lower terrace, with **wall towers on its corners**. **Pylon towers** join round the main tower. Older walls and towers grow a storey. One more pair of banners. |
| **A module is built** | A **small tower** stands up on its slot in the courtyard (no more floating cube on a beam). Each type has its own top: crystal (Energy Lattice), dome (Shield Matrix), armoured block (Vault), dish mast (Uplink), three-prong claw (Collector), tilted panel (Sensor Net), landing deck (Drone Hangar), twin stacks (Fabricator), floating crystal needle (Command Relay, Capital only). |
| **A module is upgraded** | Its tower **gains a storey** (Mk I = 1 storey ... Mk IV = 4) and its top gets bigger. |

**Walls, sci-fi:** a dark armoured wall with glowing seams, and on top a see-through **energy field** in the faction
colour strung between emitter posts. Each ring has a **gate** facing the same way, so there is a straight path from
the street to the main tower. Wall towers are hexagonal, with glow bands and a dark spike on top; the Capital's outer
corners are **bastions with turrets**, and EVERY wall tower (all rings, all tiers) sends a **light beam into the sky** in the faction colour that **fades out toward the top**; beams slowly sweep in small circles with light pulses running up (owner, 2026-11-13; plain beams, no rings round them). Video: `capital-beams.mp4`.

### Tier table (real metres, as in the mock-ups)

| Tier | Main tower | Walls | Big towers | Module towers | Total | Footprint radius |
|---|---|---|---|---|---|---|
| T1 Spire | about 21 m, 4 storeys | ring 1 (3 corner towers) | 4 | 4 | 8 | 15.6 m |
| T2 Outpost | about 28 m, 5 storeys | ring 1 (6 corner towers) | 9 (+2 pylons) | 8 | 17 | 15.6 m |
| T3 Stronghold | about 35 m, 6 storeys | + ring 2 | 16 | 12 | 28 | 21.6 m |
| T4 Citadel | about 42 m, 7 storeys | + ring 3 | 23 | 18 | 41 | 28.6 m |
| T5 Capital | about 48 m, 8 storeys | + ring 4 (bastions), mid-wall towers on ring 3 | 36 | 24 | 60 | 34.6 m |

Big towers = main tower + pylons + wall towers (the owner's reference goes 2 / 6 / 9 / 13 / 17 / 22 / 30 / 36).
Today's models: platform 4 to 10 m radius, tower about 7 to 22 m, so this is roughly 2 to 3 times bigger. Every
footprint still sits well inside the base's zone (25 m to 100 m). Slot counts are the plan's (4/8/12/18/24): ring 1
inside wall 1, ring 2 between walls 1 and 2, ring 3 between walls 2 and 3.

**Size dial:** one admin setting (`BaseModelScale`, default 1.0) scales the whole base in AR, so if a 48 m Capital is
too much for a street it can be turned down without new models.

## 2. Map and holo table (AR is never forced)

- **Map, close zoom:** the base is drawn to scale from the server's layout: wall rings in the faction colour with
  their gate, towers as pale hexes, each module as a lit hex in its slot (**bigger for a higher Mk**). A player who
  never opens AR still sees it grow.
- **Map, far zoom:** the hex icon grows with tier and shows one outline per wall ring with dots for corner towers, so
  a Spire and a Capital are easy to tell apart at a glance.
- **Holo table:** the same models as AR, scaled to fit.

## 3. Models to export (after the owner OKs the look)

Unity-ready FBX, metres, pivot at ground centre, same child roles as today (Hull, Glow, GlowSoft, Beam, Glass,
PlateGrid, Banner_n), glow tinted in the faction colour in Unity:

- Main tower: `core_plinth_t1..t5`, `core_storey_1..8` (stacked), `core_crown_t1..t5`.
- `terrace_ring_1..4`, `wall_segment` (1 m long, scaled), `wall_field` (energy field, scaled), `wall_gate`.
- `tower_storey`, `tower_cap` (pylons and wall towers are stacks of these), `bridge` (scaled), `bastion_turret`.
- `mod_storey` + one top per module type (`mod_top_lattice`, `_shield`, `_vault`, `_uplink`, `_collector`, `_sensor`,
  `_hangar`, `_fabricator`, `_relay`), `mod_pad`, `power_line`.

Code stacks storeys by tier and Mk (the server already sends tier, slots and each module's Mk), so **no new API data**
is needed. The old whole-tower models (`base_tower_tN`, `base_module`, `base_arm`) retire. Tap-to-aim in AR keeps
working: modules and the core are still separate objects. Triangle budget to check at export (a Capital is about 60
towers; storeys are shared meshes so Unity can instance them).

## 4. Decided (owner, 2026-11-13)

- Look approved ("ok looks"): main tower, pylons, sci-fi walls with energy fields and gates, module towers.
- Every wall tower and bastion sends a light beam into the sky that fades out toward the top; the beams move, each in
  its own random way (owner: "make it more random").
- Size as drawn (about 21 m to 48 m tall); a client constant `BaseModelScale` can turn it down later. Five tiers stay.
- Models exported in pieces (section 3, final names `base_*` in mobile `Assets/Art/Models/Base/v2/`); build prompt:
  API `docs/base-growth-build-prompt.md` (also carries the credit Mint and 2.5x nodes).
