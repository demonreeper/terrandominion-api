# Prompt for local Claude Code: faction bases

Start this **after the current build is finished** and after the Tether Node restyle (`docs/tn-restyle-build-prompt.md`, T1). `git pull` both repos first (API:
`docs/base-building-plan.md`; mobile: `docs/GAME_DESIGN.md`). Paste this into Claude Code in
`D:\Development\Github\ProjectAR`.

---

Read `TerranDominion-API\docs\base-building-plan.md` and the Factions, Combat and Field and AR parts of
`TerranDominion-Mobile\docs\GAME_DESIGN.md`. The plan is approved; follow it and don't guess anything it doesn't
say. If the current build isn't finished, finish it first.

Build it milestone by milestone: B0, B1, B2, B3, B4, B5, B6, B7. Leave B8 (Capital, Command Relay, Sensor Net,
Drone Hangar, Fabricator) for later. One branch per milestone (e.g. `bases-b1`) in each repo it touches, one
commit per milestone per repo; API and app ship together; breaking API changes are fine (only I play on dev). Use
the next free migration number and new error codes. Every number is a setting in `td.settings` with a range, in a
new settings group "Bases". Wrap every base route that acts on a place in `s.located(...)`. Run `go test ./...` and
the edit-mode tests, deploy to dev, install on my phone, then go straight on to the next milestone without waiting
for me.

1. **B0 data and rules:** tables for bases, slots `(ring, index)`, zones, proposals (flags), projects,
   contributions, vault and vault log. Placement rules as pure functions in `internal/game`: proposer rank and home
   region, on land within 15 m of a walkable way, never at sensitive places (OSM worship, cemetery, school,
   hospital, police, military, government, with `BaseSensitiveBufferM` 50 m), `BaseSpacingSameFactionM` **3000 m**,
   `BaseSpacingOtherFactionM` 150 m, zone gap `BaseZoneGapM` 30 m, and the base cap per faction per region that
   **grows with the region's signal population** (active players of all factions; thresholds are settings). Admin
   tools: place, clear, set tier, release a flag.
2. **B1 flags and the Spire:** PROPOSE BASE (Warden or atoll/city lead, on the spot) plants the faction's flag; it
   blocks other factions' proposals, bases and zone growth there; one open proposal per player, 24 h cooldown after
   a decline or expiry (`BaseProposalCooldownHours`), never more open flags than free base slots, the Commander can
   turn proposals off. The regional Commander (or acting Commander) gets a push and a PROPOSALS list and approves or
   declines from anywhere in the country; 48 h expiry (`BaseProposalHours`). On approval the flag becomes the seed
   beacon and the Spire project starts. Base names are picked from six generated names per faction word list, never
   typed. Map: zone disc in the faction's colour, Spire icon by tier, base card. Holo table: tower on its hex
   platform with empty pads (same model AR uses later). No mission ever asks for a flag.
3. **B2 projects and giving:** projects with a bill and an assembly time; 2 at once per base; members give from
   their uploaded inventory **on site** (inside the zone), Wardens+ can pay from the vault anywhere in the region;
   presence speed-up per member on site with a cap; cancelling sends what was given to the vault. Modules Energy
   Lattice, Shield Matrix, Vault, Uplink. **Contribution points** for giving, with the values as settings, that
   **auto-promote** to Warden and then atoll/city lead (never Commander), with **limited slots per rank per
   region**; when full, promotions queue by points. The module lock-on animation (wireframe, beam, lock flash).
4. **B3 growth:** tiers Spire, Outpost, Stronghold, Citadel (Capital stays B8); zone radius per tier; the border rule
   with the ghost ring (cyan room, red `BORDER: TOO CLOSE TO <BASE>`); the cap shown as `BASES n / m`.
5. **B4 economy:** Collector Array on nodes inside the zone (TN rules, slowdown if no member came by), vault log for
   Wardens+, Uplink uploads with no 5% charge for members; base missions from the member's power (give, stand at,
   repair).
6. **B5 combat:** module targeting for EM Pulse, Overload Spike and drones; hack steals from the vault above the
   safe floor (Merki larger); Shield Matrix by Mk and covers members' TNs inside the zone; Lattice down takes
   modules offline; protection 24 h new, 12 h after a tier drop; core at 0 drops a tier (outer ring packed away at
   50% to the vault), a Spire is destroyed and half its vault drops on the map (48 h); repair on site; attacking a
   base ends a walk-together link of its members; a freelancer who attacks loses neutral standing for 3 days; push
   to the faction's members in that country (once per base per 30 min); guard jobs on the JOBS board.
7. **B6 upkeep:** a member on site every 7 days or the base fades (no new projects, shield halved, dimmed, a power
   message), then drops a tier each further week; a faded Spire collapses after 14 days.
8. **B7 AR and polish:** life-size base in AR (looks only), each faction's own glow colour on beams, modules, rings,
   banners, zone and shield; living base in AR only (packets on Collector lines, patrol drones, mist, idle turrets),
   the holo table plainer; signal echoes also near bases; PHOTO on bases; sky battles over bases.

**Art: use the ready-made models in `TerranDominion-Mobile\Assets\Art\Models\Base\`, don't build them in code.**
`base_platform_t1`..`t5` and `base_tower_t1`..`t5` (Spire to Capital, same pivot), `base_module`, `base_module_pad`,
`base_arm`, `base_turret`, `base_banner_pole`; materials and sizes are in `Assets\Art\Models\README.md`. Place them on
the server's hex-slot layout: platform and tower at the base origin, a pad and module per slot, an arm from the tower to
each inner-ring module, turrets and banner poles round the rim. Owner colour on every glow part, faction banner on the
banners. Match the reference renders in `TerranDominion-API\docs\art-reference\base\` (hero, lineup, assemble, shield,
six-factions). For anything the models don't cover, use free assets only: Quaternius Modular Sci-Fi MegaKit and
Sci-Fi Essentials Kit (CC0), Kenney Space Station Kit / Modular Space Kit, the Free Force Field Shader for URP (Void1
Gaming, itch.io) for the shield, tinted to the faction colour. Check every asset for cross shapes. Set texture max
size 1024 so it stays light on mid-range phones.

Player-facing text is country-neutral, names no power except in perk tags, and never hints at a seventh persona.

At the end, list what you built per milestone, anything you couldn't do, and the settings you added. Then test with
Shinan in Malé: propose a flag as a Warden, approve it as Commander, give on site, grow to Outpost, and check the
border ghost ring near another base.
