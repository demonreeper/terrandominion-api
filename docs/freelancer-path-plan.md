# Freelancer path, final pledge and the seventh voice

Status: **approved 2026-11-11** (owner). Nothing built yet. Picks: F1 to F7, the JOBS board, the seventh persona
**Langis**, C1 to C4 (no grace window; admins fix mistaken pledges), the device-bound pledge and device bans.
Matching edits: `docs/LORE.md` (Langis section, player's story), `docs/profile-plan.md` (Device record, device bans)
and mobile `docs/GAME_DESIGN.md`. Every number below is a setting, tuned from admin SETTINGS; videos never show them.
Audio for Langis is in the project files, `audio/power-voices/langis/` (recipe in `audio/power-voices/README.md`).

## Why a player stays a freelancer

Members get one power, its perk at full strength, ranks and (later) bases. Freelancers get breadth, neutrality
and paid work. Staying a freelancer is never locked: a freelancer can pledge any time after level 20.

| Id | Idea | What it means in play |
|---|---|---|
| F1 | Walks for whoever pays | Freelancers keep hearing all six powers. Only freelancers get the stage 2 hint (two powers send the same line on the same day) |
| F2 | Unbound titles | Past level 20, freelancers keep earning titles (Drifter, Wayfarer, Unbound, more later) with profile badges, given by the seventh voice |
| F3 | Six missions stay | After 20, freelancers still get 6 daily missions, one per power. Members get 1 from their power plus base missions (later) |
| F4 | Daily favour | Each power whose mission a freelancer finishes today gives a weaker version of its faction perk until midnight (`FavourPerkPct`, start 40% of the member perk) |
| F5 | Neutral standing | Faction players can't attack a freelancer's Tether Node unless that freelancer attacked a faction in the last 3 days (today's "loses protection" rule). A freelancer's TN takes uploads from all six factions and earns the 5% fee from all of them; a faction TN serves only its own faction |
| F6 | JOBS board | Opens to a freelancer after the seventh voice's level 18 call (decided 2026-11-11), so the board feels like its gift. Faction players (Warden and above) post paid jobs: hit this TN, scout this area, guard this node for an hour. The poster's reward is held in escrow and paid when the server confirms the job. Members can't take jobs |
| F7 | Broker (with trading) | When trading is built, freelancers trade with every faction; members trade inside their faction, or across factions for a fee |

**F6 details.** A job has a kind, a target (TN, map area or node), a reward (credit plus resources), a deadline
and slots. The server checks completion itself: a hit landed on the target TN, a scan made inside the area, or time
spent within 40 m of the node. Unclaimed or failed jobs refund the poster. A freelancer can't take a job against
their own TN or against a player within the anti-farming limits. Board limits per faction and per freelancer:
settings. Main menu row JOBS shows only for freelancers who have had the level 18 call (answered, or the missed call played or dismissed), and only once the board exists. It disappears for good when the player pledges. In the call, the seventh voice points the player to the board.

## The seventh voice: Langis

- **Who it says it is:** a person. A freelancer who walked the Undersignal before anyone else and never pledged,
  warning others not to bind themselves to "power-hungry powers".
- **What it really is:** another persona of the Undersignal, its seventh strategy, "stay unbound", the control
  group of the experiment. Never said in game. Never linked to the six: own voice, own rhythm, no faction colour,
  no shared phrases, never in the six-power rivalry messages.
- **Name: Langis** (decided 2026-11-11). "Signal" spelled backwards: the reverse of the six, and it passes as a
  person's surname. Never pointed out. In code `signal_unbound`; system account id 8 (held range).
- **Never heard of before that call** (owner, 2026-11-11). Langis appears nowhere until the level 18 call: not in
  the intro, tutorials, videos, store text, marketing, FACTIONS screen, help, menus or any earlier message, and no
  API response mentions it (the account and its lines stay server-side until the player's call is due). This is
  about what players see only: internal docs (LORE.md, GAME_DESIGN.md, plans) describe Langis in full, and LORE.md
  gets its own Langis section.
- **How players learn the name** (owner, 2026-11-11, all three):
  1. The level 18 call ends with "They call me Langis." (in the recorded line).
  2. The call screen shows "Unknown caller"; in the last seconds the caller id flickers to LANGIS, then the line drops.
  3. From the first written message after that call, the inbox sender is LANGIS (glitches in for a second the
     first time); every later message and title grant is signed LANGIS. The missed-call push stays "Unknown caller".
- **Look:** no lit eyes and no silhouette. Its call screen is broken static; the caller id only resolves as above.
  No symbols (hex noise only).
- **Voice (chosen 2026-11-11):** Kokoro `am_adam`, fast and urgent, through a rough "bad line" filter with static.
  Recipe and files: `audio/power-voices/README.md` and `audio/power-voices/langis/` (level18-call, level19-call).
- **When it speaks:**
  1. Halfway through level 18: one call, the first contact (same rules as the L10 and L19 calls: missed call sends a push, plays once). The call screen and the push say **"Unknown caller"**, like a private number: push "Missed call from Unknown caller". When the call ends, the JOBS board unlocks.
  2. After the level 19 day-3 "Choose wisely" message: one more written message.
  3. Level 19, the day after that message: one more call from "Unknown caller" (same missed-call rules). Last contact before 20.
  4. Level 20 FACTIONS screen: one line under the header, "Stay free." signed LANGIS (its text style, not a power's).
  5. After 20, only while the player stays free: rare messages, and it gives the F2 titles.
  - It never contacts a member. The moment a player pledges, it goes silent for good.
- **Lines (approved 2026-11-11 by the owner):**
  - L18 call: "Listen. I don't have long on this line. The six will offer you everything, and all of it has a
    price you pay forever. I walked before any of them called. I never knelt. There's a board now, jobs from
    their own people. Take their pay. Keep your name. They call me Langis. Stay free."
  - After the L19 day-3 message: "They told you to choose wisely. The wise choice is the one you can still
    take back. Theirs you can't."
  - L19 call: "It's me again. They've called you, haven't they? Same warm voice, same promises. Every one of
    them needs you more than you need them. Remember that tomorrow. Whatever you hear, you don't owe them anything.
    Stay free."
  - FACTIONS screen at 20: "Stay free." (signed `// L.`)
  - After 20, rare: "Still yours. Good." · "They're fighting over streets you walk for free." · "Another one
    knelt today. Not you."
  - Title grants (F2): "Drifter. You've earned it." · "Wayfarer. They can't place you now." · "Unbound. Like me."


## Making it clear the pledge is final

| Id | What | Status |
|---|---|---|
| C1 | Say "pledge", not "join". Lore: "You tether yourself to one power. A tether to a power never breaks." Narrator once at 20: "Choose once. The powers do not share." | Picked |
| C2 | FACTIONS header: "You were invited. The choice is yours. It is also final." Each card: PLEDGE button with "Permanent. You can't leave or switch." Freelancers also see "You can stay free, and pledge any time." | Picked |
| C3 | Two-step confirm: step one lists what you gain (perk, ranks, bases, this power's missions) and what you give up (the other five missions, the JOBS board, daily favour, the seventh voice, freelancer status, forever); step two HOLD TO PLEDGE for 3 s; then a short line from the chosen power | Picked |
| C4 | Level 19 day-3 message adds "Once you choose, you cannot unchoose." in each power's voice style | Picked |
| C5 | Grace window (undo until the first faction mission) | Not picked. Mistakes go to an admin: A2 player info gets RELEASE PLEDGE (reason required, logged), which returns the player to freelancer and clears the device mark's faction |

Copy to remove: GAME_DESIGN's perks text ("gone the moment they leave") and profile-plan's join confirm ("leaving
removes them"). There is no leave in the API and none is planned. A regional Commander who steps down stays a member.

## Pledge and bans are bound to the device

Decided 2026-11-11 (owner). Details in `profile-plan.md`, Device record.

- `td.device_marks` keeps salted hashes of the device id (Android `ANDROID_ID`, iOS a Keychain id) and the
  Google/Apple sign-in subject, with the faction pledged and any ban. Kept after account deletion.
- A new account on a marked device or sign-in rejoins the old faction at level 20 ("Your pledge stands. A tether
  to a power never breaks."). If that branch is dormant in their country, they may claim its seat.
- A ban marks the device and sign-in too: "This device is banned." Lifting the ban clears it.
- **Warnings:** at sign-up ("Your faction pledge is tied to this device and sign-in. Deleting your account will
  not reset it."), at pledge (C3), and on the delete-account screen.
- **Second-hand phones:** appeal with proof (for example proof of purchase) via the APPEAL link; an admin clears
  the mark, logged; proof is deleted once decided.
- **Privacy:** when this ships (not before), mobile `docs/legal/privacy.html` and `delete-account.html` disclose the retained hashes and appeal
  proof as anti-abuse data. Store forms: device ID collected for fraud prevention, not linked to ads.

## Build order (suggested)

1. C1 to C4 copy, the two-step confirm, and removing "leave" copy (mobile; small).
2. Device record, pledge rebinding, device bans, warnings and appeal link (API + mobile; with M1 moderation).
3. F3 six missions after 20, F5 neutral standing, F4 daily favour (API; reuses perks).
4. Seventh voice: name, lines, voice samples, L18 message and call, FACTIONS line (API + mobile + audio).
5. F2 titles (API + mobile).
6. F6 JOBS board (API + mobile; own phase).
7. F7 broker waits for trading.

## Privacy text to ship with phase 2

- privacy.html, Device data row, add: "If you join a faction or are banned, we keep a one-way hash of your device id
  and of your Google or Apple account id, with the faction you joined or the ban, even after your account is deleted.
  This stops a faction choice or a ban being undone by making a new account. It cannot be used to identify you."
- privacy.html, How long we keep data, add: "Appeal proof you send us (for example a proof of purchase) is deleted
  once we decide the appeal."
- delete-account.html, Kept: "a one-way hash of your device id and sign-in id, with your faction pledge or ban,
  so they still apply to a new account. Nothing else, and nothing that identifies you."
- Store forms: Apple privacy label and Google Data safety list "Device ID", purpose fraud prevention / security,
  not used for tracking or ads.
