# Plan: lore correction

Every lore and design change the owner decided on 2026-11-11, and the work each one needs. The story is in
`docs/LORE.md`. Where this plan and `GAME_DESIGN.md` (mobile repo) disagree, this plan is newer; `GAME_DESIGN.md` has
been updated to match. Every number is a starting value to tune.

## Changes decided

| # | Change | Before |
| --- | --- | --- |
| 1 | The lore is final (`docs/LORE.md`): the Undersignal, a network-born AI, six strategies played against each other, the hint stages, the system-to-story mapping | Draft |
| 2 | Verdant Exchange's power is **Ishāra** (keep the macron) | Ishaaraath |
| 3 | Persona names confirmed: Signum, Shingō, Sēma, Ishāra, Tohu, Merki | "To confirm" |
| 4 | Freelancers get **6 daily missions**, one from each power | 4 a day |
| 5 | After joining a faction, **only that faction's power** sends missions; the goal becomes **building bases** (pending, see 12) | Faction-flavoured missions; command guidance "from Ishāra" for all |
| 6 | **Halfway through level 19**, only the power that matches what the player mostly does **calls the player** in its own voice | Invite at level 20 by the most-followed persona, no call |
| 7 | **Voices:** the narrator is its own voice; each power has a distinct voice | One narrator, voice open |
| 8 | **The reveal** that the six are one is undecided and never fully confirmed for now | World event at the global head election |
| 9 | **Global head election** isn't open yet; what happens to the powers is decided when it opens | Powers hand over to an elected head |
| 10 | **Why freelancers see it:** phone tech has drifted close to the Undersignal's frequency | Open |
| 11 | **Level 19 call is only an offer** (no accept button): day 1 call, day 2 repeat call, day 3 written message, then it stops | Open |
| 12 | **Faction bases and their missions stay pending** until bases are designed | Open |
| 13 | **Level 10 call:** one call halfway through level 10 from the power matching the player's path, about that path, no repeats | None |
| 14 | **Power voices chosen:** Signum `im_nicola`, Shingō `jm_kumo`, Sēma `af_nicole`, Ishāra `hf_alpha`, Tohu `pm_alex` (a beat between sentences), Merki `am_onyx`, all Kokoro. **No power says its own name** | Open |
| 15 | **Narrator voice chosen:** dark, layered Kokoro `bm_george` (pitched down, a lower copy under it, deep echo), no phone filter | Open |
| 16 | **Missed calls:** a call while the app is closed sends a push ("Incoming call: <power>"); next time the app opens, the player can play it once, then it's gone | Open |
| 17 | **Startup intro:** plays on the first load, after the splash, before the main menu. Skippable, replayable from Settings | Placement open |
| 18 | **TN tutorial video:** plays the first time the player uses any Tether Node feature or menu (BUILD TN HERE, MY TETHER NODES, a TN's panel). Once; replayable from Settings | Placement open |
| 19 | **Splash screen** at every launch while the game loads (spec below) | None |

## Work to do

Docs (done in this PR and the matching mobile PR):

- [x] `docs/LORE.md` and this plan (API repo)
- [x] `GAME_DESIGN.md` and `docs/regional-command-plan.md` (mobile repo) updated for changes 2 to 19

Server (API):

- [ ] **Rename** Ishaaraath to Ishāra wherever it's stored: seed data, mission and order text, lore messages (change 2)
- [ ] **Daily missions:** 6 a day for freelancers, one per power, refreshing at midnight Maldives time as now (change 4)
- [ ] **After joining:** missions come only from the player's faction power. Base-building missions wait for faction
      bases (change 12); until then members keep the current faction-flavoured missions from their own power (change 5)
- [ ] **Command guidance** for Regional Commanders and Wardens comes from their own faction's power, never from one
      power for all (change 5)
- [ ] **Playstyle score:** count each player's actions per power (table below), so the server knows who calls at
      level 10 and 19. A tie goes to the power whose daily missions the player completed most (change 6)
- [ ] **Calls:** server-side call events: level 10 (one call at 50% of level 10's XP), level 19 (at 50% of level 19's
      XP: call, then a repeat call the next day, then a day 3 message, unless the player reaches level 20 first)
      (changes 6, 11, 13). `GET` endpoint for pending calls and messages; mark a call played
- [ ] **Push:** a call that fires while the app is closed sends a push notification, using the existing FCM outbox
      (change 16)
- [ ] **Messages:** store the day 3 message (and later written lore messages) per player, readable by the app
      (change 11)
- [ ] **Level 20 invite:** `GET /v1/factions` says which faction invited the player (change 6)

App (mobile):

- [ ] **Persona name** Ishāra in app strings; check the game font draws ā, ō and ē (change 2)
- [ ] **Incoming call screen:** full screen, the power's silhouette with lit eyes, its name and faction colour, its
      voice line playing, one END CALL button. No accept button (changes 6, 11, 13)
- [ ] **Missed call:** when the app opens with an unplayed missed call, offer to play it once (change 16)
- [ ] **Message area:** an inbox for the powers' written messages, starting with the day 3 message. It can share the
      ORDERS screen or be its own MESSAGES screen (change 11)
- [ ] **FACTIONS screen at level 20:** the inviting faction first, glowing in its colour, with an INVITED tag, and the
      line "You were invited. The choice is yours." All six stay pickable. Plain game text, not a power speaking
      (change 6)
- [ ] **Splash screen** at every launch (change 19, spec below)
- [ ] **Startup intro** on first load after the splash, before the main menu; SKIP; WATCH INTRO in Settings (change 17)
- [ ] **TN tutorial** the first time any TN feature or menu is used; once; replay in Settings (change 18)
- [ ] **Voice files:** add the power lines (`.ogg`) to the game, one per power per line key (below)

Audio and video:

- [x] Six power voices and all level 10, level 19 and repeat lines recorded (project files `audio/power-voices/`)
- [x] Narrator voice chosen (recipe in `audio/power-voices/README.md`)
- [ ] Redo the startup intro and the TN tutorial video with the final lore and the narrator. The teasers stay as they
      are; old videos are not otherwise updated

## Who calls

Only one power calls and messages: the one that matches what the player has mostly done so far. The others stay
silent.

| Player mostly... | Power |
| --- | --- |
| Harvests nodes, especially crystal | Signum |
| Builds, upgrades and repairs Tether Nodes and gear | Shingō |
| Scans, finds rare nodes, sets or spots traps | Sēma |
| Uploads, collects from TNs, earns credits | Ishāra |
| Covers distance, launches drones, strikes first | Tohu |
| Shields, intercepts, defends and repairs under attack | Merki |

## Lines

No power says its own name; the call screen shows who is calling. Audio file keys: `level10`, `level19-call`,
`level19-repeat` per power.

**Level 10 call** (one call, no repeats):

| Power | Line |
| --- | --- |
| Signum | "You gather like someone who knows the light runs out. Keep counting. I am." |
| Shingō | "Half the walkers out there build nothing. You build. Don't stop now." |
| Sēma | "You found something today that no one else saw. So did I. I found you." |
| Ishāra | "Your ledger is getting interesting. Keep it that way, and we may do business." |
| Tohu | "You're fast. But not fast enough yet. Keep moving." |
| Merki | "You held when you could have run. Remember that feeling. You will need it." |

**Level 19 calls** (about 15 s; an offer only):

| Power | Day 1 call | Day 2 repeat call |
| --- | --- | --- |
| Signum | "Freelancer. I have counted every shard you carried. Not one was wasted. The light remembers those who gather it. When you reach the threshold, stand with the Lumen Collective. I will wait." | "Again. I am patient. The light is not." |
| Shingō | "Listen. You build. You repair. You don't walk away from what you started. Most walkers leave nothing behind. You leave structures. Forge Meridian needs hands like yours. Reach level twenty. Then build something that lasts." | "The offer stands. Like everything we build." |
| Sēma | "You keep finding what others walk straight past. That is how I found you. The Veil never asks twice. For you, perhaps. Reach twenty, and I will show you what is hidden." | "You let it ring. I noticed. I notice everything." |
| Ishāra | "I have reviewed your harvests, your uploads, your margins. Impressive. I would like to make you an offer. At level twenty it becomes binding. Consider the price of saying no." | "My offer has not changed. The market has." |
| Tohu | "No time. You reached nodes others never even saw. That is what we ride on. Slipstream Vanguard moves at dawn, every dawn. Hit twenty. Then keep up." | "Still standing still? Move." |
| Merki | "I watched you hold your ground when running was easier. Few do. The Bastion Order does not forget a wall that stands. Reach twenty. Hold the line with us." | "The line still holds. Your place in it is open." |

**Day 3 message** (written, in the message area). Each power words it its own way, because the same sentence from all
six would link them:

| Power | Message |
| --- | --- |
| Signum | "You were given a chance. Few are. Choose wisely." |
| Shingō | "Chances are built once. This was yours. Choose wisely." |
| Sēma | "I do not call a third time. You were seen. Choose wisely." |
| Ishāra | "Final offer. You were given a chance most never get. Choose wisely." |
| Tohu | "Last signal. The chance is moving. Choose fast, choose wisely." |
| Merki | "The gate was opened for you. It will not stay open. Choose wisely." |

## Splash screen (change 19)

Shown at every launch while the game loads, portrait. First launch: splash, startup intro, main menu. Later
launches: splash, main menu.

- **Background:** dark navy with a faint hex grid that slowly brightens, like the Undersignal waking.
- **Six lights:** six small points in the faction colours glowing faintly beneath the grid ("six powers, waiting
  beneath the signal"). They pulse out of step, never in sync, so they don't read as one.
- **Centre:** the app icon, then TERRAN DOMINION in the game font, and CLAIM THE WORLD YOU WALK. under it.
- **Bottom:** a thin cyan signal trace that fills as loading progresses, with a "//" status line that cycles:
  "// TUNING TO THE UNDERSIGNAL", "// SIGNAL FOUND", "// SYNCING THE FIELD".
- **Sound:** a low hum and one soft signal tone when loading finishes. No voice.
- **Rules:** no country named, no crosses, stars, crescents or haloes, no faces.
- **Timing:** at least 1.5 s so it never flashes, up to however long loading takes.

## Still open

- How faction bases work, and their missions (change 12).
- The reveal (change 8) and the global head election (change 9).
