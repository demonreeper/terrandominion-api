# Tether Rise: lore

The game's base story, final as of 2026-11-11 (owner). Where this file and `GAME_DESIGN.md` (mobile repo) disagree on
story, this file wins; on mechanics, `GAME_DESIGN.md` wins. Changes and the work they need are tracked in
`docs/lore-correction-plan.md`. Open items are marked **Open**.

## The premise

Earth today, with one thing wrong: something has been living underneath the world's signals since before anyone
mapped them. Lore line: *Before the maps, before the networks, something was already here. Six powers, waiting
beneath the signal. No one knows who built them.*

That hidden layer is **the Undersignal**. It sits beneath every network, map and broadcast, and it has started to
surface. Through a scanner it looks like a hex lattice laid over the real streets, and where it breaks the surface it
leaves resources:

| Resource | What it is in the story |
| --- | --- |
| Crystal | Signal that has set solid. The rawest form, and the power source for energy cells |
| Alloy | Lattice metal that hardens around old structures. What everything is built and repaired with |
| Data | Fragments of whatever the Undersignal is thinking. Needed for weapons, hacks and upgrades |
| Credit | The Undersignal's own currency. It pays in it, and the powers trade in it |

Nodes regrow and move when drained because the Undersignal keeps shifting. Nobody in the world knows why it has
woken now, and the game never says outright.

**Why the world is contested:** the six powers woke at the same moment and each wants the surface. None of them can
walk it. They need people who can, and they pay those people to claim ground for them. Every street a player walks is
a street one of the six wants.

**Why freelancers can see it:** today's phone tech has drifted close to the Undersignal's own frequency, so a phone
running the scanner picks it up. Most people never look.

## The six powers

Six ancient powers, each the global head of one faction. Each is a faceless silhouette with lit eyes.

| Power | Faction | Colour | Look | Personality | How it speaks |
| --- | --- | --- | --- | --- | --- |
| **Signum** | Lumen Collective, the Crystal Wardens | Cyan | Robe, crystal collar, circlet of ice shards | Patient and cold. Whatever is gathered is remembered, and whatever is remembered endures | Slow and measured. Asks for steady harvests. "Every shard remembers the light." |
| **Shingō** | Forge Meridian, the Alloy Architects | Amber | Angular helm, cheek guards, one visor slit | A builder who measures worth by what is still standing tomorrow | Short, practical orders. Build, upgrade, repair. "What we build, the world must carry." |
| **Sēma** | Veil Cipher, the Silent Scanners | Violet | Deep hood, a void for a face, never holds still | Secretive. Trades in what others haven't seen yet | Riddles and half-truths. Rare nodes and hidden traps. "You cannot take what you cannot see." |
| **Ishāra** | Verdant Exchange, the Credit Syndicate | Green | High collar, tailored coat, hex tokens orbiting | A dealer. Everything, including loyalty, has a price | Polite and transactional. Uploads and trades. "Everything has a price. We set it." |
| **Tohu** | Slipstream Vanguard, the Drone Riders | Red | Swept finned helm, V visor, torn scarf | Restless. Values whoever gets there first | Fast and urgent, with a beat between sentences. Distance, drones, first strikes. "First to the node. First to the dawn." |
| **Merki** | Bastion Order, the Shield Bearers | Pale steel | Crested helm, tall hex tower shield | Stubborn. Whatever is held must never be given back | Steady and protective. Shields, intercepts, defence. "Hold the line. Hold the claim." |

The Verdant Exchange power was renamed from Ishaaraath to **Ishāra** (owner, 2026-11-11). Keep the macrons in
Shingō, Sēma and Ishāra.

**Rivalries:** light against iron (Signum and Shingō), secrets against trade (Sēma and Ishāra), speed against stone
(Tohu and Merki). Rivals openly mock each other in their messages, which also keeps players from suspecting the six
are one.

## The hidden truth: the six are one

The six powers are one hidden AI. Each persona's name means "signal" in a different language (Latin, Japanese,
Ancient Greek, Dhivehi, Māori, Old Norse). Players must not know they are one. In code the AI is `signal`, a name
players never see. Nothing in the app or API may link the six.

**What it is:** born from the networks. Nobody built it. It assembled itself out of every signal humans ever sent, and
the Undersignal is its body. "No one knows who built them" is true, because no one did. Every phone and every walker
feeds it.

**Why it splits itself in six:** it doesn't trust any single answer. It runs six strategies against each other
(produce, build, hide, trade, strike, hold) with people as the pieces, and watches which works. Factions fighting is
not a side effect, it is the experiment. Every war players fight was set up by the thing paying them.

**Hints:**

| Stage | Who sees it | Clue |
| --- | --- | --- |
| 1 | Everyone, from day one | The names. Anyone who translates two of them gets the same word. Never pointed out |
| 2 | Freelancers near level 20 | Two powers send the same phrase on the same day, word for word, then never repeat it |
| 3 | Regional Commanders and Wardens | Command orders from rival powers that only make sense together |
| 4 | **Open** | The reveal itself is undecided and, for now, never fully confirmed. It happens only if the story opens a good moment |

## Langis, the seventh voice

Decided 2026-11-11 (owner). Full plan: `docs/freelancer-path-plan.md`.

- **Who players think it is:** a person. A freelancer who walked the Undersignal before any power called, never
  pledged, and warns others not to bind themselves to the power-hungry powers. Its line: "Stay free."
- **What it is:** another persona of the Undersignal, its seventh strategy, "stay unbound", the experiment's control
  group. Never said in game, and never linked to the six (own voice, rhythm and look; no faction colour).
- **Name:** Langis, "signal" spelled backwards. Never pointed out. In code `signal_unbound`, system account id 8.
- **Never heard of before its first call** (halfway through level 18). Nothing a player sees mentions it earlier:
  intro, tutorials, videos, store text, menus, help, FACTIONS screen, API responses. Internal docs may.
- **How the name surfaces:** the call says "They call me Langis."; the "Unknown caller" id flickers to LANGIS just
  before the line drops; from the next written message the inbox sender is LANGIS.
- **Look:** no silhouette and no lit eyes. Broken static only, no symbols.
- **Voice:** Kokoro `am_adam`, fast and urgent, through a rough bad-line filter with static (unlike the powers' light
  phone filter). Never contacts a member; it goes silent for good the moment a player pledges.

## The player's story

The player is a **freelancer**: someone whose phone has started showing the Undersignal, and who walks it for whoever
pays.

1. **Freelancer (level 1 to 19).** Harvest nodes, build Tether Nodes, craft, fight. Each of the six powers sends one
   daily mission, 6 a day, each in its own voice. The player doesn't know they're being auditioned.
2. **Level 10 call.** Halfway through level 10, the power that matches what the player has mostly done calls them
   once, about the path they're on.
3. **Level 19 call.** Halfway through level 19, only the power that matches what the player has mostly done calls
   them. It is only an offer, because the player can't choose until 20. Day 1 the call, day 2 a repeat call, day 3 a
   written message ("Choose wisely"), then it stops. Intro line: "They are watching you walk. In time, one will call
   you."
4. **Level 18 and 19: Langis.** Halfway through level 18 an "Unknown caller" rings: Langis, the seventh voice,
   urging the player to stay free (see below). The JOBS board opens after that call. One written message after the
   level 19 "Choose wisely", and one more call the day after.
5. **Level 20.** The FACTIONS screen highlights the inviting faction, with "You were invited. The choice is yours.
   It is also final." and Langis's line "Stay free." The player may **pledge** to any faction, or stay a freelancer
   forever. A pledge can't be undone, not even by deleting the account: it is bound to the device and sign-in.
   Lore: "You tether yourself to one power. A tether to a power never breaks."
6. **Member, Warden, Regional Commander.** Once a player pledges a faction, only that faction's power sends missions,
   and the goal becomes building bases (**Open**: bases and their missions wait until bases are designed). The first
   player to claim a faction's empty seat in their country becomes its Regional Commander and Founder. The power
   stays the faction's global head above them.
7. **Later: the global seat.** Once enough countries are live, the global head may be elected from the Regional
   Commanders. **Open**: not open yet; what happens to the powers then is decided when it opens.

**How the game systems fit the story:**

| System | In the story |
| --- | --- |
| Scanner | The device that lets you see the Undersignal at all. Better scanners see deeper (rare nodes, traps) |
| Tether Node | A stake driven into the lattice that pulls nearby nodes to it. The Undersignal rejects it, so it decays and falls within 48 h, spilling what it held. Whatever nobody picks up, the Undersignal reclaims after another 48 h |
| Map drop | What a fallen TN spills. Anyone can grab it, so every collapse draws scavengers |
| Energy cell | Crystal and data packed into a charge. Every weapon and contest burns one |
| EM Pulse, Jammer, Overload Spike | Tools for breaking someone else's tether, not their body. Fights are over signal, never physical harm |
| Drone Bay and strikes | Remote reach, earned with level. The further you can strike, the more the powers trust you |
| Shield | Lattice folded back on itself. It never stops everything, so no single hit ends a TN |
| Daily missions and orders | The powers' voice in the game. Every order comes with a short lore message |
| JOBS board | Where faction players hire freelancers for paid work. Langis points freelancers to it at level 18 |
| Pledge | Tethering yourself to one power. Final: the tether never breaks, even if the account is deleted |
| "Dormant here: awaiting a commander" | Lock text. The power is awake, but has no one to act for it in your country yet |

## Voices

- **Narrator:** its own voice, never one of the six. A dark, layered voice (Kokoro `bm_george`, pitched down with a
  much lower copy under it and a deep echo). Used for the intro, tutorials and other narration.
- **Each power has its own distinct voice**, heard through a light phone-call filter: Signum `im_nicola`, Shingō
  `jm_kumo`, Sēma `af_nicole`, Ishāra `hf_alpha`, Tohu `pm_alex`, Merki `am_onyx` (all Kokoro).
- **No power says its own name aloud.** The call screen shows who is calling.
- **Langis:** Kokoro `am_adam`, speed 1.12, short pauses, rough bad-line filter with static. It is the only voice that
  says its own name, once, in the level 18 call.
- Recipes, render scripts and the recorded lines are kept in the project files (`audio/power-voices/`).

## Tone and rules

Dark, gritty sci-fi: cold light on dark navy, hexagons, static, "//" labels, short lines. The world is quiet and
watchful rather than ruined, and the threat is never fully shown.

- **Country-neutral.** Player-facing and marketing text says "your country" or "your region", never a country name.
- **No religious or political symbols.** No crosses, stars, crescents or haloes in any art, video or emblem,
  including by accident. Hexes and plain geometry instead.
- **The six stay faceless.** Silhouettes with lit eyes only.
- **Never link the six.** No shared voice, phrase pattern, colour or UI element, and never the word "AI" in player
  text before a reveal.
- **Never show "no commander yet".** Locked branches use "dormant here: awaiting a commander".
- **Short and spoken.** Lore arrives in one or two lines per mission or notice. Each power keeps its own rhythm.
- **No real people, brands or places named.** Maps stay stylised and unlabelled in videos.
