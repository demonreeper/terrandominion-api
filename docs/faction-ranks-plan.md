# Faction ranks v2 and Nomads

**R1 built 2026-11-12** (API migration 0065, `internal/game/ranks.go`; app RANKS screen). Starting values the plan left
open: `ProspectPoints` 100, `RankPointsCaptain` 1000, `RankPointsWarden` 2000, `RankPointsLead` 5000, `RankSlotsCaptain`
12, `RankSlotsWarden` 6, `RankSlotsLead` 3, `CaptainVaultDailyCap` 500 (all resources together), `NomadAskHours` 48.
Rank places also apply to appointments. Points come from giving to base projects (B2).

Decided by the owner on 2026-11-12 in the base building thread. This replaces the five-rank table in GAME_DESIGN
("Ranks", decided 2026-11-09). Every number here is a setting in `td.settings` with a range, in the settings group
"Ranks".

## The ladder (top first)

| # | Rank | How you get it | What it adds |
|---|---|---|---|
| 1 | **Global Head** | The faction's power holds it for now. Later it is elected from the regional Commanders, when the global election opens. | Everything a Commander can do, in every region. Approves Nomad moves anywhere. |
| 2 | **Assistant Global Head** | **Empty until the global election opens.** Then **elected together with the Global Head** (they stand as a pair). | **A Warden's powers, in every region** of the faction, nothing more. Becomes **acting Global Head at once** if the seat falls vacant (step down, 14 days inactive), so the seat is never empty for long. A vote then follows, as for regional seats. |
| 3 | **Regional Commander** | Claims the faction's seat in their country at `LeadFactionLevel`, or wins a vote. | Starts bases (approves flags), appoints leads, Wardens and Captains in the country. |
| 4 | **Atoll/City Lead** | Appointed by the Commander, or earned with contribution points. One per atoll or city. | Proposes bases (flag), starts T4 projects, appoints Wardens and Captains in their area. |
| 5 | **Warden** | Appointed, or earned with contribution points. | Proposes bases (flag), starts projects, pays from the vault anywhere in the region, posts JOBS, approves Nomads coming into their region. |
| 6 | **Captain** | Appointed by a Warden or higher, or earned with contribution points. | Leads a **squad of up to 10 members** (`CaptainSquadSize`). Posts JOBS and base missions. Gives from the faction vault on site, up to a **daily limit** (`CaptainVaultDailyCap`). |
| 7 | **Member** | Leaves Prospect. | Gives to projects on site, upgrades and repairs, votes in the region. |
| 8 | **Prospect** | Every new pledge starts here. | A newcomer on trial: harvests, gives to projects and joins fights, but can't use the vault, vote, hold a seat or join a squad as more than a member. Becomes Member after `ProspectDays` (7) **or** on reaching `ProspectPoints` contribution points, whichever comes first. |

- **Contribution points** auto-promote Prospect to Member, Member to Captain, Captain to Warden and Warden to
  Atoll/City Lead at set totals (`RankPointsCaptain`, `RankPointsWarden`, `RankPointsLead`). **Never to Regional
  Commander or above.** Captain, Warden and Lead each have **limited slots per region** (`RankSlotsCaptain`, ...);
  when full, promotions queue by points.
- **Succession** stays as decided: the next rank becomes acting regional Commander at once, and a vote within 7 days
  can replace them. For the global seat the Assistant Global Head acts.
- **Votes and seats:** Prospects and Nomads don't vote and can't hold a seat.
- **Squads:** a Captain picks their squad from Members and Prospects in the region, and a player is in at most one
  squad. Squads are a grouping only for now (a roster, the JOBS and base missions the Captain posts to them).
- Ranks are **data, not code**: an ordered list with each rank's powers as a lookup, so a rank can be added or
  renamed later without touching every check.

## Nomads

A **Nomad** is a faction member who can move their **home region** to another unlocked region of their own faction.
It exists for players who **really move to another country** (owner, 2026-11-12). It replaces the admin-only MOVE
REGION for players (admins keep it).

- **Being there still counts:** a move changes where a player belongs, not where they can play from. Missions, jobs,
  orders, base giving, flags and anything else that acts on a place need the player's GPS in that country, with the
  normal anti-spoofing checks (`s.located(...)`), as for everyone. A Nomad who moved home regions but is still abroad
  can talk to the new region but can't do its missions until they are there.

- **Becoming one:** a Member or higher chooses NOMAD on FACTIONS, giving up any rank above Member (and their
  squad) after a confirm. They stay in their faction; the pledge stays final.
- **Moving:** a Nomad asks to move to a region. The move needs a yes from **the receiving region's Commander, a
  Warden in that region, or the Global Head**. The ask lasts 48 h. On a yes, the home region moves: the player
  starts there as Member (Nomad), keeps their contribution points, level and inventory, and their TNs stay where they
  are. There are **30 days between moves** (`NomadMoveCooldownDays`).
- **While a Nomad:** no vote, no seat, no rank above Member, no squad lead. Jobs, orders, missions and base giving
  work in the current home region, only while the player stands in that country.
- **Settling:** a Nomad can SETTLE in their current home region to become an ordinary Member again and climb from
  there. Becoming a Nomad again later starts the 30 days over.
- **Visiting** other regions (talk only) is unchanged and doesn't need Nomad.

## Base plan changes

- Captain: gives from the vault **on site** up to the daily limit and posts base missions. Wardens and up still pay
  from the vault anywhere in the region.
- Prospects give to projects on site and earn points, but can't start projects or use the vault.
- Contribution points promote up the ladder above, not only to Warden and Lead.

## Build (milestone R1, after base milestone B1, before B2)

Server: the ordered rank list and powers lookup, Prospect start for new pledges and the Prospect timer, Captain
(appoint, squad, vault cap), the Assistant Global Head seat (empty, with the acting rule ready), Nomad (become, ask,
approve or decline with push, move, settle, cooldown), admin tools (set rank, clear Nomad cooldown). App: the rank
names and badges everywhere a rank shows, SQUAD on FACTIONS for Captains, NOMAD and SETTLE with confirms, and the
move asks list for Commanders and Wardens. Player-facing text is country-neutral and says "region".
