# Build prompt: bases that grow (v2) + credit Mint and bigger nodes

One prompt for local Claude Code. Owner decisions 2026-11-13: bases grow like a fortress (main tower grows a storey per
tier, hex wall rings with towers added outward, modules become small towers that grow a storey per Mk, every wall
tower sends a moving light beam into the sky), plus the credit Mint and 2.5x bigger resource nodes in AR.
Plan and renders: `docs/base-growth-plan.md`, `docs/art-reference/base-v2/` (tier1-5, lineup, modules, map-close,
map-icons, capital-beams.mp4; sources in `source/`).

**Rules:** use the exported models as they are, never rebuild them in code (CLAUDE.md, Models). Work on a branch, build,
install on the phone so the owner can check it, and don't push or merge until the owner says go. Watch the build number.

## 0. Before you start
- You have uncommitted base-anchoring work on mobile `main` (ArGroundService, GeoAnchor, LocalGround, ArLight,
  BaseFootprint, ModelDress ...). Finish it and commit it on its own branch first (or stash it), so this work starts
  clean and the two don't mix.
- `git pull` in **both** repos. Mobile main now has the credit Mint model (`Assets/Art/Models/Node/node_credit/`) and the
  new base models (`Assets/Art/Models/Base/v2/`, see 2). API main has this prompt, the plan and the art reference.
- Open the project once (or run `ModelLibraryBuilder`, "AR Game/Rebuild Model Library") so Unity imports the new FBX
  and PNG files, writes their `.meta` files and adds them to `Resources/Models/ModelLibrary.asset`. Commit those
  `.meta` files with this work. Check that every `_Normal.png` imports as a normal map and `_MetallicSmoothness` /
  `_Emission` are linear, as for the other models.

## Part A: credit Mint and bigger nodes
Follow `docs/credit-mint-build-prompt.md` steps 2 to 4 (the coin turns with `Spinner`; resource nodes 2.5x bigger near
you in AR, fading back to today's size by about 60 m; then the checks). Step 1 there is the pull above.

## Part B: bases that grow

### B1. The models (`Assets/Art/Models/Base/v2/`, one folder per model, metres, pivot at the ground centre)
Same child roles as the other models: `Hull` (baked maps), `Glow`, `GlowSoft`, `Glass` (the wall energy field, very
transparent, owner colour). Glow parts are white: tint them in the owner colour as for the old base.

| Model | What it is | Size / stacking |
|---|---|---|
| `base_core_plinth_t1..t5` | main tower plinth (widens per tier) | height 0.9 x tier |
| `base_core_storey_a/b/c` | one storey of the main tower | 4.0 m tall, 4.6 m wide; storey k uses variant `a,b,c`[k % 3], XZ scale 0.9^k |
| `base_core_crown_t1..t5` | crown on top, bigger per tier | already sized for that tier's top storey |
| `base_terrace_1..4` | hex terrace plate of wall ring k | 0.35 m thick, top at 0.35 |
| `base_wall_ring_1..4` | the whole hex wall of ring k: armoured wall, glowing seams, `Glass` energy field on emitter posts, **one gate** | corner radius 14 / 20 / 27 / 33 m |
| `base_tower_base`, `base_tower_storey_a/b`, `base_tower_cap` | pylon / wall tower pieces (hexagonal) | base 0.6 m; storeys 3.0 m each, alternate a/b, storey k XZ scale 0.93^k; cap scaled 0.93^n; whole tower XZ x (w / 2.6) |
| `base_bastion_turret` | Capital bastion turret, faces +X like `base_turret` | sits on a 3-storey tower instead of a cap |
| `base_bridge` | sky bridge, 1 m along +X | scale X to the length |
| `base_mod_pad`, `base_mod_storey` | module pad (0.25 m) and one module storey | storey 1.7 m, w 1.9; storey k XZ scale 0.92^k; Vault and Hangar x1.21 wider |
| `base_mod_top_<kind>` | top per module type: lattice, shield, vault, uplink, collector, sensor, hangar, fabricator, relay | scale uniformly by 0.8 + 0.15 x Mk |
| `base_power_line` | glow line on the deck, 1 m along +X | scale X |

Banner poles reuse `base_banner_pole` (scale Y by (6 + 1.2 x tier) / 6). Sky beams are drawn in code (B4).
`base_tower_cap` and `base_bastion_turret` have a small glow sphere where the beam starts (cap: 3.4 m above its pivot,
turret: 1.7 m).

### B2. Layout (rewrite `Holo/BaseStructure.cs`)
All positions are on the ground plane in model metres from the base origin. Angles are measured round the up axis.
`GateAngle` is the direction the walls' gates face. **Find it in the editor capture** (the gate gap of
`base_wall_ring_1` seen from above; it was at 300° in Blender, which can come out mirrored in Unity) and give every
angle below relative to it, so modules never stand in a gate.

- **Rings by tier:** `nr` = 1, 1, 2, 3, 4 wall rings for T1..T5 (rings are built at T1, T3, T4, T5).
- **Terraces:** for k = 1..nr place `base_terrace_k` at height 0.35 x (nr - k). Deck height at radius r = the top of
  the smallest terrace k whose apothem (0.866 x (R_k + 1.6)) - 0.3 is ≥ r: 0.35 x (nr - k + 1); 0 outside.
  Outer radius (`PlatformRadius`) = R_nr + 1.6 (15.6 / 15.6 / 21.6 / 28.6 / 34.6 m).
- **Main tower** at the centre on the deck: plinth_tN, then STOREYS = 4,5,6,7,8 storeys from the plinth top, then
  crown_tN. About 21 / 28 / 35 / 41 / 48 m to the tip. This is the core (tap target and aim point).
- **Pylons** round the main tower: count 0,2,3,4,6; slot order j → idx = [0,3,1,4,2,5][j]; angle = GateAngle + 60° x
  (idx + 1); radius 6.2; storeys n = tier + 1 - j/2 (integer); w = 2.4. A bridge from radius 1.84 to radius 5.2 at
  deck + 0.6 + 3.0 x n x 0.62.
- **Walls:** `base_wall_ring_k` at its terrace's deck height, turned so its gate faces GateAngle.
- **Wall towers** on each ring's corners: angle = GateAngle + 30° + 60° x c (c = 0..5), radius R_k. T1 has only three
  (c = 1, 3, 5, i.e. GateAngle + 90°, 210°, 330°). age = tier - builtTier[k] with builtTier = 1,3,4,5; storeys
  n = max(2, 1 + min(age, 2) + (k == 1 ? 1 : 0)); w = 2.6 - 0.1 x (k - 1); facing outward. Ring 4 (Capital)
  corners are **bastions**: 3 storeys + `base_bastion_turret` facing outward. The Capital also has **mid-wall towers**
  on ring 3: GateAngle + 60° x e (e = 1..5, not the gate edge), at radius 0.866 x 27, n = 2, w = 2.0.
  Tower counts (main + pylons + wall towers): 4, 9, 16, 23, 36.
- **Module slots** (server `(ring, index)`; the slot never moves when the base tiers up):
  - ring 1, radius 9.6: eight positions at GateAngle + 71.5° + 45° x p. Index i → p = 2i for i < 4, else 2(i - 4) + 1
    (so a T1 module keeps its place at T2).
  - ring 2, radius 16.0: index 0..3 at GateAngle + 60°, 120°, 240°, 300°.
  - ring 3, radius 22.6: index i < 6 at GateAngle + 60° x i + 14°; i ≥ 6 at GateAngle + 60° x (i - 6) - 14°.
  - A filled slot: `base_mod_pad` at deck height, then Mk storeys of `base_mod_storey`, then its
    `base_mod_top_<kind>` (module ids map: lattice, shield, vault, uplink, collector, sensor, hangar, fabricator,
    relay). An empty slot shows the pad only. Aim point: the module's top.
  - Ring 1 modules get a `base_power_line` on the deck from radius 3.6 to the pad (length r - 5.2).
- **Banners:** 2 x tier poles at radius PlatformRadius x 1.05, angle 360°/n x i + 180°/n + 8.6°.
- Drop the old arms, floating modules on beams and the old platform/tower models from `BaseStructure`.
- **Assembly (B2 projects):** a module or Mk storey being built shows its new piece floating above its place in the
  existing ghost style and lowering as the bar fills (as the old module did). A tier project shows the new wall ring as
  a ghost at its place.
- `PlatformRadius`, `TowerTop` and the slot aim points keep their meaning for `ArBaseService`, `BaseLife`,
  `BaseShieldDome`, `BaseFootprint`, the AR quick attack bar and the holo table.

### B3. Size in AR
Model metres stay real metres (`TrueSizeOfBase` = 1): a Capital is about 48 m tall and 69 m across. Keep the existing
far-growth and up-high clamps. Put the 1.0 in one constant (`BaseModelScale`) so the owner can turn the whole base
down later if it is too big for a street.

### B4. Sky beams (owner: "add to all bastion towers, fade out to the top, make them move, more random")
Every wall tower and every bastion (not the pylons, not the main tower) sends a beam from its glow sphere straight up.
Drawn in code, no rings round it (a flat ring round a vertical beam reads as a cross).
- Shape: 30 model metres tall (scaled with the base); a soft outer beam 0.40 m wide at the bottom to 0.22 m at the
  top, about 35 % opacity, and a bright core 0.07 to 0.05 m. Owner colour, additive like the other glows.
- Fade: brightness and opacity x smoothstep from 1 at 2 % of its height to 0 at 90 %.
- Pulses running up: brightness x (0.65 + 0.7 x max(0, sin(22 v - 9 t))^4), v = 0..1 up the beam, t in seconds.
- Movement, random per beam (seed from base id + tower index), the beam pivots at its foot:
  - lean outward 1 to 5°;
  - tilt about X: a1 sin(2π f1 t d + p1) + a2 sin(2π f2 t + p2); tilt about Z: 0.8 a1 sin(2π f3 t + p3) +
    a2 cos(2π f4 t d + p4);
  - a1 3 to 10°, a2 1 to 4°, f1 and f3 0.05 to 0.22 Hz, f2 and f4 0.25 to 0.7 Hz, phases random, d = ±1.
  The video `docs/art-reference/base-v2/capital-beams.mp4` shows it.
- AR: dimmed up high like the rest of the base. Holo table: beams 10 m and half as bright. Faded or building
  base: beams off.

### B5. Map (AR is never forced)
`Ui/FieldMapView.Bases.cs`:
- **Close zoom** (the base's outer radius is at least ~40 px on screen): draw it to scale from the same layout as
  B2 (share the maths, one place): wall rings as hex outlines in the owner colour with the gate gap, towers as pale hexes,
  each filled module as a lit hex sized by Mk (1.3 / 1.7 / 2.1 / 2.5 m), the main tower as a white hex with a lit
  centre. See `map-close.png`.
- **Far zoom:** the hex icon grows with tier (as now) and shows one hex outline per wall ring with a dot on each corner
  tower and the main tower in the middle. See `map-icons.png`.
- Tapping either opens the base card as now.

### B6. Check
- Editor: the hologram capture of all five tiers matches `tier1..5.png` and the gate lines up (fix `GateAngle`);
  `text-check` passes.
- Phone, dev base HOUSE TEST (tier 5): stands on the street at its spot, beams move, tap a module and the core in AR,
  the map shows the close and far views, the holo table shows the same base.
- Frame rate on the phone with the Capital in view (37 towers + 24 modules + 36 beams). If it drops, combine the static
  pieces of one base into one mesh per material when it is built (`Mesh.CombineMeshes`), and keep beams as one mesh.
- When it works, delete the old `base_tower_t1..5`, `base_platform_t1..5`, `base_module` and `base_arm` folders and
  update `Assets/Art/Models/README.md`.
