# Build prompt: WALK TOGETHER fix and the three missing base features

Run after B7. `git pull` both repos first. B8 stays for later. Each part is its own commit. Run `go test ./...` and the
edit-mode tests, deploy the API to dev, build and install, then report.

## Part 1: WALK TOGETHER location (mobile)

**Bug:** WALK TOGETHER always says "Your position isn't accurate enough to link yet." (E1816). The server needs a fix of
`WalkLinkMaxAccuracyM` (25 m) or better. WALK TOGETHER opens from FRIENDS, on the menus, and the menus run coarse,
network-only location: only Field Ops publishes `PreciseLocationWanted`. So ACCEPT and LINK are always refused, FRIENDS
NEARBY stays empty, and a link would drop as soon as the player leaves Field Ops.

1. **Precise location by source.** Make `PreciseLocationWanted` a request with a source (for example
   `{ source, wanted }`). `GeoLocationService` runs precise while any source wants it.
   - Field Ops behaves as now.
   - `WalkTogetherController` wants precise while its panel is open or a link is up (`m_Link != null`).
   - Background still stops location.
2. **Wait for a good fix.** While `AccuracyMeters` is above the public setting `WalkLinkMaxAccuracyM`:
   - the panel shows "Getting a precise position…" with the current ± metres;
   - ACCEPT and LINK are greyed out and don't send;
   - `/v1/walk/open` is skipped.

   Once the fix is good, poll as now and refresh the panel.
3. **Indoors hint.** If the fix stays weak for 20 s, show "Step outside or near a window for a better signal." in place
   of the waiting line.
4. **Heartbeat.** Skip a beat with a weak fix, unless the grace rules in `service/walk.go` need it sent. Don't change
   the server's checks.

Test: an edit-mode test for the request set: with two sources, one releases and location is still precise.

## Part 2: Captain squad missions (API + mobile)

Plan: `docs/faction-ranks-plan.md` (Captain, Squads) and `docs/base-building-plan.md` 10 (base missions).
`PowerBaseMissions` exists in `game/ranks.go` but nothing uses it yet.

- A Captain (`PowerBaseMissions`) posts a mission to their squad, from the base mission kinds that already exist: give
  at the base (a resource and an amount) or stand at it (minutes). Repair a module too, if B5 makes that easy. It names
  one of the faction's standing bases in the region.
- Each squad member gets it as an extra mission for the day, in their power's own words, shown with the other missions
  and marked "from your Captain" (the Captain's name as a `PlayerRef` link). It counts on site only (`s.located`), like
  the other base missions.
- Limits, as settings in the "ranks" group: missions a Captain can post per day (`CaptainMissionsPerDay`, default 2);
  each mission's points and XP (the base mission's reward, no more).
- App: SQUAD on FACTIONS gets POST MISSION for a Captain (the base, the kind, the amount, a confirm). Squad members get a
  push with ALERTS.
- If the squad changes, the mission goes with the player who has it; a Captain who loses the rank keeps nothing open.

## Part 3: Guard job at a base (API + mobile)

Plan: `docs/base-building-plan.md` (Defence: "Wardens+ can post guard jobs on the base on the JOBS board, so
freelancers can be hired to defend it"). Today guard jobs need a Tether Node (`td.jobs.node_id`).

- New migration: `td.jobs.base_id` (nullable, `ON DELETE SET NULL`). A guard job has a node or a base, never both.
- Posting: Wardens and up post GUARD THIS BASE on a base they can see in their region. The guard spot is the base's
  core, and the radius is the base's own zone or `JobGuardRadiusM`, whichever is larger.
- Counting: the existing `GuardTime` and `JobFix` with the base's spot. A base that collapses or is destroyed ends the
  job, and the reward goes back as for a node that is gone.
- App: JOBS shows "Guard <base name>" with the base's place on the map; the base panel has POST GUARD JOB for Wardens
  and up.

## Part 4: Acting Commander on a vacant seat (API + mobile)

Plan: GAME_DESIGN "Leader succession" (decided) and `docs/faction-ranks-plan.md`. Today nothing fills the seat when a
regional Commander leaves.

- **Triggers:** the regional Commander steps down (new STEP DOWN on FACTIONS, with a confirm), is inactive for
  `CommanderInactiveDays` (14, setting), is banned, or deletes the account.
- **Acting Commander:** the highest rank in that country takes the seat at once as acting Commander; ties go to most
  contribution points, then the longest in the faction. Prospects and Nomads are skipped. Store it on the branch
  (`leader_id` plus `leader_acting = true`), so `RegionCommanders`, flag approval and the base rules pick it up as
  they already expect.
- **Vote:** for `CommanderVoteDays` (7, setting) after the seat changes, any Member or higher in that country can start a
  vote. Candidates are members who can hold a seat. One vote per voter; voting ends after 48 h
  (`CommanderVoteHours`). A majority of votes cast wins; with no majority the acting Commander stays and the seat stops
  being acting. With no vote started in the 7 days, the acting Commander becomes the Commander.
- **Nobody left:** the branch goes dormant and the seat can be claimed fresh, as now.
- **Pushes and ALERTS:** to the members in that country when the seat changes and when a vote opens or ends. The FACTIONS
  roster shows "ACTING COMMANDER" with VOTE while a vote is open.
- Admin: SET RANK already exists. Add END VOTE to the admin faction view for a stuck vote.
- Tests (game): the order of succession, the vote result, a tie, and the 7-day window.

## Report

What each part looks like on the phone, the settings added, and anything from the plans that you chose differently.
