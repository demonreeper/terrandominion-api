# Faction bases: plan

Status: **approved by the owner 2026-11-11.** **B0 built 2026-11-12** (API: migration 0063, `internal/game/bases.go`,
admin tools; settings group "Bases"). T1 (TN restyle) is on hold until ready-made models arrive (owner, 2026-11-12);
**B1 built 2026-11-12** (migration 0064): flags and the Commander's answer, the map, the base card and the
holo table from the models. Choices in B1 the plan left open: the proposer picks the base name when planting the flag
(six offered, SHOW OTHERS), and the Spire takes it; flags show on the map to everyone with their time left; the cap
counts every player seen in the region within `BaseCapActiveDays`, freelancers included (owner, 2026-11-12); sensitive
places include kindergartens, childcare, colleges and universities with schools (owner, 2026-11-12).
**B2 built 2026-11-12** (migration 0066) with the R1 ranks: Wardens and up start modules from anywhere; members give on
site; Wardens and up pay from the vault anywhere in the region, Captains on site up to their daily cap. Starting values
the plan left open: module Mk I bills (Lattice 300/200/100/50, Shield 250/300/150/50, Vault 400/100/100/100, Uplink
200/150/200/50) and 30 min assembly, both times the Mk; 300 health per Mk; 1 contribution point per resource given, 5 XP
per 100; the Forge Meridian (25% faster) and Verdant Exchange (15% cheaper) perks apply. The Spire's own project can't be
cancelled. Modules lock on but do nothing yet: their effects come with B4 and B5.
**B3 built 2026-11-12**: tier projects (Outpost and Stronghold by Wardens, Citadel by leads), the border rule at start and
again when assembled (a finished tier waits until there is room), the ghost ring on the map (cyan with room, red
`BORDER: TOO CLOSE TO <BASE>`; it stays 90 s after the card closes, since the card covers the map) and `BASES n / m` on
FACTIONS. Core health is restored to the new tier's full health when it grows.
**B4 built 2026-11-12** (migration 0067): the Collector Array (from the Outpost; 2, 3, 3 and 4 nodes at Mk I to IV, bill
350/250/150/100 and 45 min, times the Mk), deposits on site (they earn points, like giving), Uplink uploads with no
charge, the VAULT LOG for Wardens and up, and base missions: with a standing base in the region one of a member's daily
missions becomes base work in their power's own words (give 200 alloy, stand 10 minutes; repair waits for B5). A member
on site marks the base visited each minute (upkeep). Captain-posted base missions to a squad are not built yet.
**B5 built 2026-11-12** (migration 0068): EM Pulse, Overload Spike and drones aim at a module or the core; the Shield
Matrix cuts every hit and covers members' TNs inside the zone; Energy Lattices power 4 modules each (modules need power:
without a Lattice none is online); module at 0 offline until repaired; core at 0 drops a tier (outer modules packed
away at 50% of their cost to the vault, 12 h protection) or destroys a Spire (half the vault dropped on the map, 48 h);
hacks (75 m, 50 data, 5-15% above the 20% safe floor, Bastion Order 35%, 30 min per player per base; the puzzle is on
the phone); intercept, defuse and repair on site; the push once per base per 30 min; a freelancer attacker loses
neutral standing; the attacker's walk link with a member ends. Guard jobs on a base use the existing guard job at the
base's spot; no base-specific guard job was added.
**B6 built 2026-11-12** (migration 0069): fading after 7 days without a member on site (no new projects, shield halved,
dimmed, members told "<Base> is fading. Walk to it."), a tier lost each further week, a faded Spire collapsing after 14
days. A collapse is treated like a destruction (half the vault drops on the map): the plan doesn't say what a collapse
leaves, so this follows the destruction rule.
In the API repo as `docs/base-building-plan.md`, with a short
`GAME_DESIGN.md` entry replacing "Faction base design: Open". Every number is a starting value and
a server setting, tuned in testing.

## The ask (owner, 2026-11-11)

> Bases start small on a tower, maybe, then expand as long as the faction keeps building and doesn't
> reach near other bases. But maybe set a limit?

Concept art (References/, owner's photos): a dark tower core on a round hex platform, glowing
modules **descending on beams and locking onto the tower**, faction **banners** on tall poles around
the rim, a **CONSTRUCTION 87%** bar and a **MODULES LOCKING** list (structural core, energy lattice,
shield matrix, banners). Other frames: modules floating in wireframe before they lock, a fortress
with a shield dome and drones, a seed beacon on the ground in a park.

## What is already decided and this plan keeps

- **Only factions build bases.** Changed by the owner 2026-11-11: **only the regional Commander starts a
  base**; Wardens and atoll/city leads **propose** one, and the Commander approves (section 1b).
- **AR is a view, never a gate** (`no-ar-support-plan.md`): a base is a **GPS spot plus a hex-slot
  layout owned by the server**. Every phone builds and manages it in the **holo table**; AR shows the
  same base life-size, looks only, no gameplay bonus.
- Bases **grow in place** in tiers. One base per faction within **500 m** (`BaseSpacingSameFactionM`)
  and a minimum distance from other factions' bases (`BaseSpacingOtherFactionM`, 150 m).
- **Hacking a base**: hex-path puzzle at 75 m, costs data, steals 5–15% of stored resources, 30 min
  per player per base. Bastion (Merki) bases keep a larger safe floor.
- **Walk together**: attacking a linked member's faction base ends the link.
- Freelancers keep neutral standing; a freelancer who attacks a faction loses it for 3 days.
- Lore rules from `LORE.md`: no power says its own name, the six are never linked, nothing hints at a
  seventh voice, country-neutral text, no cross shapes in the art.

## The idea in one paragraph

A Warden proposes a spot and plants the faction's **flag** there; once the regional Commander approves, the
faction raises a **Spire**: a single tower on a small hex platform, standing on a real spot, with a
**zone** around it drawn on the map in the faction's colour. Every member can feed it. Each tier adds
a ring of **module slots** around the tower and **pushes the zone outward**, so a base that its
faction keeps building visibly spreads across the map. Growth stops at a **border**: a zone may never
come closer than a gap to another base's zone, of any faction, so the first faction to build claims
the ground. There is a **top tier** (so yes, a limit), a **cap on how many bases** a faction has in a
country, and **upkeep**: a base nobody visits fades and shrinks back, freeing the ground.

## 1. The base and its zone

- **Spot:** the builder's GPS position (accuracy ≤ 20 m, the same spoof checks as everything else).
  Like seeded nodes it must be on land within **15 m of a walkable way**, not inside a building, not in
  water. **Never at sensitive places** (OpenStreetMap tags): places of worship, cemeteries, schools,
  hospitals, police, military and government sites, with a **50 m** buffer (`BaseSensitiveBufferM`).
  This matters in the launch region and is also what store reviewers look for.
- **Zone:** a circle around the spot whose radius grows with tier (table below). On the map it is a
  soft faction-coloured disc with a hex-pattern edge; the Spire icon sits in the middle.
- **Name:** picked from six generated names (like `IRONROOT`, `GLASS MERIDIAN`), with SHOW OTHERS,
  never typed (same reason as callsigns). Each faction has its own word list so names feel like it.

### Placement rules (server-checked when a spot is proposed, and again on approval)
1. The proposer is a **Warden, atoll/city lead or the regional Commander** of a pledged faction, standing on
   the spot, in their **home region**.
2. The spot is **≥ 3 000 m** from every base of their own faction (`BaseSpacingSameFactionM`, raised from 500 m by the owner 2026-11-11).
3. The spot is **≥ 150 m** from every other faction's base centre (existing rule), and the new
   starting zone stays **≥ 30 m** (`BaseZoneGapM`) outside every existing zone's edge.
4. The faction is under its **base count cap** for the country (section 3). Open proposals count toward it.
5. Not a sensitive place (above).

## 1b. Proposing a base: the flag (owner, 2026-11-11)

Only the **regional Commander** can start a base. Everyone else proposes.

1. **Propose:** a Warden or atoll/city lead stands on the spot and taps **PROPOSE BASE**. The server runs the
   placement rules. If they pass, the faction's **flag** is planted there: a tall banner in the faction's colour
   on the map and in AR, with the proposer's callsign and a countdown.
2. **The flag holds the ground:** while it stands, **no other faction** can propose, flag or build a base there.
   The blocked area is the Spire's starting zone plus the border gap, the same area the base would take.
   Other factions' bases also can't grow into it. The flag can't be attacked; it is a claim, not a structure.
3. **Approve:** the Commander gets a push and a **PROPOSALS** list (spot on the map, proposer, distance to the
   nearest bases). They can **APPROVE** or **DECLINE** from anywhere in the country, without walking there.
   On approval the flag becomes the seed beacon and the Spire project starts (members fund it on site as usual).
   The Commander can also raise a base directly: their own proposal counts as approved.
4. **Expiry:** if the Commander hasn't answered within **48 h** (`BaseProposalHours`), or declines, the flag
   comes down and the ground is free again. The proposer is told either way.
5. **Limits (anti-spam):** each Warden or lead has **one open proposal** at a time, and a declined or expired
   proposal puts that player on a **24 h cooldown** (`BaseProposalCooldownHours`). The faction has at most
   **one open proposal per free base slot** (if the cap allows one more base, only one flag can stand), so a
   faction can never hold more ground with flags than it could with bases. The Commander can also turn
   proposals off for the region. While the Commander seat is held by an
   **acting Commander** (succession), the acting Commander approves.
6. Proposing is free. A declined or expired spot can be proposed again by the same faction after the flag
   comes down.

## 2. Growth: tiers, slots and the border

| Tier | Name | Slots | Zone radius | Core health | Who can start it | Upgrade cost (alloy / crystal / data / credit) | Assembly |
|---|---|---|---|---|---|---|---|
| T1 | **Spire** | 4 | 25 m | 500 | Commander approves (Wardens+ propose) | 400 / 400 / 150 / 50 | 15 min |
| T2 | **Outpost** | 8 | 40 m | 1 500 | Warden+ | 1 200 / 600 / 400 / 200 | 2 h |
| T3 | **Stronghold** | 12 | 60 m | 3 500 | Warden+ | 3 000 / 1 500 / 1 000 / 500 | 8 h |
| T4 | **Citadel** | 18 | 80 m | 7 000 | Atoll/city lead+ | 7 000 / 3 500 / 2 500 / 1 200 | 24 h |
| T5 | **Capital** | 24 | 100 m | 12 000 | Regional Commander only | 15 000 / 8 000 / 6 000 / 3 000 | 72 h |

- **Layout:** the tower in the centre, slots on hex rings around it (ring 1 at T1–T2, ring 2 from T3,
  ring 3 from T4). The server stores each slot as `(ring, index)`, so the holo table, AR and the map
  all draw the same base. Banners stand on the platform rim, one pair per tier (looks only, no slot).
- **The border rule (your idea):** an upgrade is refused if the new zone would come within
  `BaseZoneGapM` (30 m) of **any other base's zone edge**, own faction included. The base screen shows
  the next tier's ring as a ghost: cyan when there is room, red with
  `BORDER: TOO CLOSE TO <BASE NAME>` when there isn't. A blocked base can still fill its slots,
  upgrade modules and fight; it just can't spread.
- **Growing against a neighbour:** if the neighbour shrinks (it fades or loses a tier in a fight),
  the room opens again. So pushing a rival back is a reason to fight.
- **The limit:** **T5 is the top.** Only **one Capital per faction per country**, the regional
  Commander's seat, and it is the only base that reaches 100 m. Everything else tops out at T4 (80 m).

## 3. How many bases

**Decided (owner, 2026-11-11):** the cap **grows with the region's signal population** (active players of
all factions in the country), and admins can adjust it as factions grow.
- `BaseMaxPerRegion` = a base number plus steps by the region's active player count (all factions together),
  so every faction's cap rises as the whole region grows. Thresholds are admin settings, tuned live.
- Together with the **3 000 m spacing between a faction's own bases**, Malé and Hulhumalé can hold about
  **2 to 3 bases per faction** at launch (Malé fits one; Hulhumalé one or two).
- The FACTION screen shows `BASES 2 / 3`.
- **Built (B0) with these starting values:** `BaseCapBase` 2, plus one at each of `BaseCapStep1Players` 50, `Step2`
  150, `Step3` 300 and `Step4` 600 active players (0 turns a step off). "Active" is any player, freelancers included,
  whose last fix in the region is within `BaseCapActiveDays` 14 (owner, 2026-11-12). Open flags count toward the cap.

## 4. Building together: projects and contributions

Everything that takes time (raising the Spire, a tier upgrade, a module, a module upgrade) is a
**project** with a resource bill and an assembly time, like the concept art's progress bar.

1. **Start:** a Warden+ (T4 needs a lead, T5 the Commander) starts a project at the base from the holo
   table. Only **2 projects run at once** per base (`BaseProjectsAtOnce`).
2. **Fund:** any member gives resources toward the bill. Members give from their **uploaded inventory
   while on site** (inside the zone), so building means walking to the base. Wardens+ can also pay from
   the **faction vault** (section 6) from anywhere in the region.
3. **Assemble:** once funded, the assembly timer runs. Each member **on site with the app open** speeds
   it up by 10%, up to **x2** (`BasePresenceStepPct`, `BasePresenceMaxMult`). In AR and on the holo
   table the module descends on its beam and locks on as the bar fills.
4. **Credit (decided, owner 2026-11-11):** every resource given earns **contribution points** and a little
   XP; the values are set later. Points **auto-promote** up the rank ladder (Prospect, Member, Captain, Warden,
   atoll/city lead; see `docs/faction-ranks-plan.md`) at set totals, as well as by appointment. A **Captain** gives
   from the vault on site up to a daily limit; Prospects give but can't start projects or use the vault. **Never to regional Commander** (that seat is claimed or voted).
   **Each rank has a limited number of slots per region** (settings): when the slots are full, the next
   promotion waits for a free slot, and the queue goes by points.
5. A project can be **cancelled** by whoever started it or a higher rank: what was given goes to the
   faction vault, not back to members (so nobody farms points).

## 5. Modules (first set)

Modules fill slots. Each has health, a Mk (I to IV, like TN gear, upgraded in place) and needs power.

| Module | From | What it does |
|---|---|---|
| **Energy Lattice** | T1 | Powers up to 4 other modules. Knock it out and those modules go offline: the siege target from the concept doc. |
| **Shield Matrix** | T1 | The base's shield, same share rule as the TN shield (by Mk against the attack's Mk, 5–75%, no one-hit kill). **Members' TNs inside the zone** use the better of their own shield and this one. |
| **Vault** | T1 | Stores the faction's resources (section 6). Hacks steal from here; a safe floor is never taken. |
| **Uplink** | T1 | Members inside the zone upload carried resources here with **no 5% charge**, and it counts as a TN upload point for them. |
| **Collector Array** | T2 | Tethers free nodes **inside the zone** (2 per array at Mk I, 4 at Mk IV) into the vault, on the TN rules (5-min cooldown, slowdown if no member came by). |
| **Sensor Net** | T2 | Warns members of attackers inside the zone and reveals traps there (shows only what the map shows, sooner). |
| **Drone Hangar** | T3 | Auto-intercepts one inbound drone per charge at the base, with the intercept shares by Mk. Members on site can launch strikes from it like a Drone Bay. |
| **Fabricator** | T3 | Members crafting on site pay 15% less. |
| **Command Relay** | T5 only | The Capital's unique module: shows all the faction's bases in the country on one holo view, lets the Commander post region-wide orders. |

Banners are not modules: every tier adds a pair, in the faction's colour and mark, looks only.

### Faction perks on bases (concept strengths, every number a setting)
Signum +25% crystal into the vault from Collector Arrays · Shingō assembly 25% faster · Sēma Sensor
Net range doubled · Ishāra projects 15% cheaper · Tohu Hangar drones 30% faster · Merki base shield
+10% and a larger vault safe floor (already decided for Bastion).

## 6. The faction vault

- Each base has its own vault (no global pool, so a raid on one base can't empty the faction).
- Fed by Collector Arrays, cancelled projects and members' gifts (a member can also just deposit).
- Spent only on that base's projects by Wardens+. Members can't withdraw. Every movement is logged
  and visible to Wardens+ (`VAULT LOG`), so a bad Warden is easy to spot.
- **Safe floor:** the first 20% (Merki 35%) can't be stolen by hacks.

## 7. Upkeep and fading

Bases are permanent, but only while the faction looks after them, so ground never stays locked by a
dead faction.
- A base needs **a member on site at least once every 7 days** (`BaseUpkeepDays`).
- Missed: the base **fades**: no new projects, shield halved, the map shows it dimmed. Members get a
  power message ("<Base> is fading. Walk to it.").
- After **7 more days** it **drops a tier**: the zone shrinks, modules in the outer ring are packed
  away (50% of their cost to the vault). This repeats each week. A faded Spire collapses after 14 days.
- Any member on site ends the fade at once.

## 8. Attacking a base

Asynchronous and location-gated, like all combat. Anyone can attack a base (a freelancer who does loses
neutral standing for 3 days; attacking it ends a walk-together link that has one of its members).

- **Protection:** a new base is untouchable for **24 h**; after it drops a tier in a fight, 12 h.
- **Targeting:** EM Pulse, Overload Spike and drones pick a **module** (or the core). A module at 0
  goes offline until repaired. The Shield Matrix cuts every hit; the Lattice is the way to drop it.
- **Hack** (decided): steals 5–15% from the vault above the safe floor.
- **Core at 0:** the base **drops a tier** (zone shrinks, outer ring packed away) instead of being
  wiped. A Spire whose core hits 0 is **destroyed**: half its vault drops on the map as a pickup (the
  usual 48 h, reclaimed by the Undersignal), the rest is lost.
- **Repair:** members on site, alloy per point (the TN repair rule).
- **Defence:** members on site intercept drones and defuse spikes as at a TN; the Hangar intercepts
  automatically. Wardens+ can post **guard jobs** on the base on the JOBS board, so freelancers can be
  hired to defend it.
- **Feedback:** push to the faction's members in that country when their base is hit (one push per base
  per 30 min), ALERTS entry, sky battles in AR.

## 9. Seeing it

- **Map:** zone disc in faction colour, Spire icon sized by tier, a progress ring while a project
  assembles, dimmed while fading. Tap opens the base card (name, tier, faction, health; members see
  projects and BUILD).
- **Holo table** (every phone): the base on a dark hex platform, tower in the middle, slots as hex
  pads. Tap an empty pad to start a module; tap a module for status, GIVE, UPGRADE, REPAIR. Projects
  show their module floating above its pad in wireframe; it descends on a beam and **locks on** when
  done (the concept image). Same model as AR, plainer.
- **Raising a Spire** starts as a **seed beacon**: a crystal on rings on the ground (the park frame),
  filling a DEPLOYING bar during its 15 min assembly, then the tower rises out of it.
- **Zone edge** in AR: low hex/triangle shards of light in the faction's colour along the border, and
  tall holographic banners with the faction mark and the base name at the entrance (the fortress frame).
  Where two factions' zones face each other, both colours show across the gap.
- **AR** (looks only): life-size. Tower height by tier (about 6 m Spire to 30 m Capital), modules
  locking on with beams and a small shockwave, banners moving in the wind, the shield dome flickering
  when hit, the living-TN packet flow along Collector lines. PHOTO works on bases.
- **AR extras on bases** (the "AR sees more" rules from `no-ar-support-plan.md`, looks only: no resources, XP
  or combat information):
  - **Living base:** packets flow along Collector lines to the tower, patrol drones circle, banners and mist
    move, turrets idle. The holo table shows the same base **plainer**: static lines, no packets, no idle motion.
  - **Signal echoes** are also placed near bases (same placement and CODEX rules as near nodes and TNs).
    Tap one in AR to read its line; the approved echo lines only, nothing about a seventh voice.
  - **Photo mode** works on bases: frame line, TETHER RISE mark and callsign, no place name or coordinates.
  - **Sky battles** over bases: strikes, intercepts and the shield lighting up, only what the map shows.
- **Shield colour:** the shield, beams, module glow, banners, zone and edge shards take the **faction's colour**;
  the HUD stays cyan.
- **Other factions' bases** show to everyone in their colour; only members see the inside (projects,
  vault).

## 9b. Art assets (owner, 2026-11-11)

Start with **free assets only**; paid packs come later, after funding.
- Structures: **Quaternius Modular Sci-Fi MegaKit** and **Sci-Fi Essentials Kit** (CC0), with **Kenney Space
  Station Kit / Modular Space Kit** (CC0) for placeholders. Kitbash the Spire tower, modules and turrets from them.
- Shield: **Free Force Field Shader for URP** (Void1 Gaming, itch.io), tinted to the faction's colour.
- Textures: CC0 sci-fi panels (ambientCG / Poly Haven) or the procedural panel set from the mock-up renders.
- **Models (owner, 2026-11-12):** the Blender mock-ups are exported as game models (mobile `Assets/Art/Models/`, TN and base parts with baked textures and separate glow parts); the game uses these, not shapes built in code. Reference renders and sources: API `docs/art-reference/`. One glow colour per faction.
- Check every asset for cross shapes before it goes in the game.
- Later, after funding: a heavier paid kit (for example Sci-Fi Heavy Station Kit) and a hit-effect shield.

## 9c. Tether Node restyle (owner, 2026-11-12)

The TN gets the same solid, panelled look as the bases, so the two read as one family. Look reference: the
renders in `video/base-mockup/v3-blender/tether-node/` (project files; source `source/tn.py`, model
`tether-node-t2.glb`).
- **Model:** a dark panelled, tapered hex pylon on a stepped hex plate (hex-cell glow inset, rim lights,
  corner blocks, survey ticks), glowing bands and slits on each segment, three leaning fins, a crown of three
  prongs holding a floating cut core in a glass halo, hex orbit rings around the core (one at tier 1, more at
  tier 2), a thin sky beam, perimeter posts with lit tips (3 at tier 1; 6 with a two-line energy lattice at
  tier 2) and a dashed health ring on the ground that turns red for the lost part.
- **Colour:** the glow takes the owner's colour. A faction member's TN (your own included) glows in the
  faction colour; a freelancer's TN (your own included) glows **freelancer blue `#3a8cff`**, because Lumen's
  cyan looked too alike. The HUD stays cyan.
- **Banners:** faction TNs carry their faction's banners on the posts; freelancer TNs have none.
- **Links and nodes** keep the resource colours (crystal, alloy, data, credit), with hex packets on the lines in AR.
- **Shield:** the hex-cell dome in the owner's colour (the same shield shader as bases).
- **Holo table** uses the same model, plainer (static lines, no packets, no idle motion), as now.
- **No flat rings around the sky beam**: seen at eye level they make a cross. Tilted orbit rings around the core
  are fine. Check the result from eye level.
- **Assets:** the same free kits and textures as bases (9b); keep it light enough for mid-range phones.
- Built as milestone **T1**, before B0, because bases reuse its materials and colour code.

## 10. Missions and story

- Members' daily mission from their power becomes base work when the faction has a base: *give 200
  alloy to <base>*, *stand at <base> for 10 minutes*, *repair a module*. **No mission ever asks for a flag** (it would reward flag spam); proposing a base is
  only the Warden's own call, or a Commander's order.
- New power lines (one voice line each, same voices, no power names itself): Spire raised, tier
  reached, base under attack, base fading, base lost. Lore framing: a base is a **stake driven into
  the lattice** (echo line 4 already says "Every stake driven into the lattice is felt."). Nothing
  about the seventh voice.

## 11. Build order (milestones, each tested on the owner's phone)

| # | Milestone | Side |
|---|---|---|
| B0 | Base data: bases, slots, zones, projects, vault tables; placement rules incl. sensitive places from OSM; admin tools (place, clear, set tier, settings) | API, **built 2026-11-12** |
| B1 | **Propose and approve**: plant a flag, PROPOSALS for the Commander, 48 h expiry; **raise a Spire** on approval; zone on the map; base card; holo table with tower and empty pads | API + app, **built 2026-11-12** |
| B2 | **Projects and giving**: fund on site, assembly timer, presence speed-up, contribution points; modules Lattice, Shield Matrix, Vault, Uplink; the lock-on animation | API + app, **built 2026-11-12** |
| B3 | **Growth**: tiers T2–T4, zone growth with the border rule and the ghost ring; base count cap | API + app, **built 2026-11-12** |
| B4 | **Economy**: Collector Array, vault log, Uplink uploads; base missions | API + app, **built 2026-11-12** |
| B5 | **Combat**: module targeting, hack on the vault, drones, protection windows, tier drop and destruction, link end, push | API + app, **built 2026-11-12** |
| B6 | **Upkeep**: fading, tier drop, collapse | API, **built 2026-11-12** |
| B7 | **AR and polish**: life-size base, banners, sky battles over bases, PHOTO; power voice lines | app + audio |
| B8 | **Later**: Capital and Command Relay, Sensor Net, Drone Hangar, Fabricator | API + app |

## Owner's choices

Decided 2026-11-11: base cap grows with the region's signal population (admin-adjustable); own-faction bases 3 000 m apart; contribution points auto-promote to Warden and lead (not Commander), with limited rank slots per region; only the regional Commander starts a base; Wardens and leads propose with a flag that
blocks other factions for up to 48 h (section 1b).

**All points below approved by the owner 2026-11-11**, except 3 and 9, which were replaced by the owner's
own rules above (cap by signal population; contribution points auto-promote with rank slots, values later).

1. **Cap the size?** Recommended: yes, T5 Capital is the top (100 m zone), one Capital per faction per
   country; other bases stop at T4 (80 m).
2. **Border gap** between any two zones: recommended 30 m, own faction included.
3. **Bases per faction per country:** recommended 2 + 1 per 5 members, at most 8.
4. **Who gives:** recommended every member, on site; Wardens+ start projects and spend the vault.
5. **Upkeep:** recommended a member on site every 7 days, then fade, then lose a tier each week.
6. **Destruction:** recommended a core at 0 drops a tier; only a Spire can be destroyed outright.
7. **Names:** Spire / Outpost / Stronghold / Citadel / Capital, and picked (not typed) base names.
8. **Sensitive places** kept clear (worship, cemeteries, schools, hospitals, government): recommended yes.
9. **Warden by contribution points:** recommended yes, at 2 000 points.

### Values settled at build time (owner, 2026-11-12)
- **Base cap:** 2 bases per faction per region, plus 1 each at 50, 150, 300 and 600 active players (all settings).
- **Active players for the cap:** everyone seen in the region in the last 14 days, freelancers included (the
  region's whole signal population).
- **Capital:** its extra 6 slots go on ring 3.
- **Admin flag release:** no proposal cooldown for the proposer (it fixes mistakes).
- **Sensitive places** also include kindergarten, childcare, college and university, along with school.
