# Prompt for local Claude Code: faction ranks v2 and Nomads (R1)

Run this **after base milestone B1 and before B2**. `git pull` both repos first. Paste into Claude Code in
`D:\Development\Github\ProjectAR`.

---

Read `TerranDominion-API\docs\faction-ranks-plan.md` and the Ranks and Nomads parts of
`TerranDominion-Mobile\docs\GAME_DESIGN.md`. The plan is approved; follow it and don't guess anything it doesn't say.

Build milestone **R1** on a branch `ranks-r1` in each repo, one commit per repo; API and app ship together; breaking
API changes are fine (only I play on dev). Use the next free migration number and new error codes. Every number is a
setting in `td.settings` with a range, in a new settings group "Ranks". Wrap every route that acts on a place in
`s.located(...)`. Run `go test ./...` and the edit-mode tests, deploy to dev, install on my phone, then go straight on
to B2 (`docs/base-build-prompt.md`), using the new ranks there, without waiting for me.

1. **Ranks as data:** one ordered list (Global Head, Assistant Global Head, Regional Commander, Atoll/City Lead,
   Warden, Captain, Member, Prospect) with each rank's powers as a lookup; replace hard-coded rank checks with it.
   Existing members become Member; existing appointed ranks stay.
2. **Prospect:** new pledges start as Prospect; Member after `ProspectDays` or `ProspectPoints`, whichever first.
3. **Captain:** appoint (Warden+), auto-promote by points, slots per region, squad of up to `CaptainSquadSize`, posts
   JOBS and base missions, gives from the vault on site up to `CaptainVaultDailyCap`.
4. **Assistant Global Head:** the seat exists, stays empty while the power holds the Global Head seat; later it is
   elected together with the Global Head, has a Warden's powers in every region, and acts as Global Head if that seat
   falls vacant.
5. **Nomads:** NOMAD (give up rank above Member, confirm), ask to move (48 h, push to the receiving region's
   Commander and Wardens), approve or decline (Commander, a Warden there, or the Global Head), move home region
   (start as Member, keep points, level, inventory; TNs stay put), `NomadMoveCooldownDays` 30, SETTLE. No vote or
   seat while a Nomad. Admin: set rank, clear Nomad cooldown, MOVE REGION stays.
6. **App:** rank names and badges wherever a rank shows, SQUAD on FACTIONS for Captains, NOMAD and SETTLE with
   confirms, the move asks list for Commanders and Wardens.

Player-facing text is country-neutral ("your region"), names no power except in perk tags, and never hints at a
seventh persona. At the end, list what you built, anything you couldn't do and the settings you added.
