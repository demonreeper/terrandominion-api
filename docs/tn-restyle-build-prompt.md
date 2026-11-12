# Prompt for local Claude Code: Tether Node restyle (T1)

Start this **before the base build** (`docs/base-build-prompt.md`), or finish the current build first. `git pull`
both repos first (API: `docs/base-building-plan.md` section 9c; mobile: `docs/GAME_DESIGN.md`, "TN look"). Paste
this into Claude Code in `D:\Development\Github\ProjectAR`.

---

Read section 9c of `TerranDominion-API\docs\base-building-plan.md` and the Tether Node part of
`TerranDominion-Mobile\docs\GAME_DESIGN.md` ("Living TNs in AR" and "TN look"). The plan is approved; follow it and
don't guess anything it doesn't say. If the current build isn't finished, finish it first.

Build milestone **T1** on a branch `tn-restyle` in the mobile repo (and the API repo only if it needs a change), one
commit per repo. Run the edit-mode tests, install on my phone, then go straight on to the base build
(`docs/base-build-prompt.md`) without waiting for me.

1. **New TN model** replacing the thin wireframe pylon in `BaseHologram` (AR and holo table share it): a dark
   panelled, tapered hex pylon on a stepped hex plate (hex-cell glow inset, rim lights, corner blocks, survey
   ticks), glowing bands and slits per segment, three leaning fins, three prongs holding the floating cut core in
   a glass halo, hex orbit rings round the core (one at tier 1, more at tier 2), a thin sky beam, perimeter posts
   with lit tips (3 at tier 1; 6 with a two-line energy lattice at tier 2), and the dashed health ring on the
   ground that shows the lost part in red. Match the renders in the project files
   `video/base-mockup/v3-blender/tether-node/` (`tn-tier1.png`, `tn-tier2.png`, `tn-shield.png`,
   `tn-six-factions.png`, `tn-freelancer-vs-lumen.png`; `tether-node-t2.glb` and `source/tn.py` show the shapes
   and sizes).
2. **Colour by owner:** a faction member's TN (your own too) glows in the faction colour; a freelancer's TN (your
   own too) glows **freelancer blue `#3a8cff`**. Add it as one shared colour constant that bases reuse later. The
   HUD stays cyan. Faction TNs carry their faction's banners on the posts; freelancer TNs have none.
3. **Keep what works:** links and node models keep the resource colours; the living-TN life (packets, idle turn,
   breathing glow, COLLECT burst, sparks under a quarter health) stays in AR only; the holo table stays plainer;
   the shield dome takes the owner's colour.
4. **No flat rings around the sky beam** (they read as a cross at eye level); tilted orbit rings round the core
   are fine. Look at it from eye level in AR and check every asset for cross shapes.
5. **Art:** free assets only (the kits and CC0 textures listed in section 9b), or build it in code like now. Keep
   it light enough for mid-range phones.

Player-facing text is country-neutral, names no power except in perk tags, and never hints at a seventh persona.

At the end, list what you changed and anything you couldn't do. Then check with Shinan in Malé: a freelancer TN
shows blue with no banners, a faction TN shows the faction colour with banners, and your own TN follows the same
rule.
