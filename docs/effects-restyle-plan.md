# Effects restyle: fit the dark theme

Status: **done**, 2026-11-11 (E0–E5 built and merged, terrandominion-mobile PR #9, tested as builds 86–87).
Drafted 2026-11-10. Follows the polish pass (`docs/polish-plan.md`, milestones 5 and 6).

## What was built

- **E0** Effects constants in `UiStyle` (glow cap, emphasis floor, durations, decode glyphs, split);
  `DecodeText`, `ScanBand`, `GlitchSplit`, `CornerBrackets` in `Ui/UiEffects.cs`; ADMIN → SOUND BOARD.
- **E1** Every cue in the sound table, from new `SoundSynth` blocks with a shared 4 kHz low-pass and
  soft clip; common cues ~15 % quieter, interface sounds at 0.5.
- **E2** Ticker, gain label with a traced line, menu level flash, busy sweep, button press (darkens;
  the 96 % press shrink was removed too, as a scale change for emphasis), 16 px panel slide.
- **E3** Level-up: traced hex (`HexTrace`), scan band, decoding number, one ring, 2.4 s. The title
  stays "LEVEL UP".
- **E4** Breathing marker rings, corner-bracket lock-on, the drone-reach crosshair replaced by a ring
  in brackets (no cross), thinner shockwave, AR slivers with a decoding label, TN hit jitter plus
  red/cyan split, quieter jam static with scan lines.
- **E5** Toasts with a left bar and a `// OK` / `// INFO` / `// WARN` / `// ALERT` tag.
- **Also:** salvage drops got their own map marker; they looked exactly like alloy nodes.

Not done, by choice: the startup intro and first-TN video (question 2), the "// CLEARANCE RAISED"
title (question 3), and the "+" on the map zoom and admin buttons.

## Why

The owner tried the polish-pass effects and found them "too childish, doesn't match the dark theme of
Terran Dominion". The audit below agrees. The colours are mostly fine (the resource colours already
match the teaser and intro faction colours). What reads as childish is:

1. **Sounds.** Most new cues are bright, major-key arpeggios and bells in the 1–4 kHz range: a coin
   for collecting, a C-major six-note fanfare for level-up, glassy "ding" harvests. That is the sound
   of a casual mobile game, not a tactical network.
2. **Bouncy motion.** Things pop past full size and settle (level-up hex 0.2 → 1.15 → 1), numbers grow
   12% and flash white, map markers pulse their size, a big "+20 CRYSTAL" arcs across the screen, shards
   spin at 400°/s into the camera.
3. **White flashes and full-strength glows.** Flashes go to pure white and additive glows run at 0.9.

The teasers and the startup intro already define the look we want: near-black navy, thin glowing
lines, text that decodes from noise, scan bands, a small chromatic split on glitches, `//` mono labels,
low synth hits under the dark music. Effects should feel like a field terminal reporting, not a reward
popping.

## Style rules (apply to every effect)

- **No overshoot, bounce or scale pops.** Things appear by fading, wiping, tracing or decoding.
  Scale changes only for real size (zoom), never for emphasis. Ease-out only, 120–400 ms.
- **No flash to white.** Emphasis goes from the colour at ~55% brightness up to the colour itself,
  then back. Additive glows cap at 0.55 alpha.
- **Keep the palette.** Backdrop `#030a14`, accent cyan `#59d9ff`, resource colours as now (they match
  the faction colours in the intro). Saturated green `Success` is used only for small text, never for
  glows.
- **Text effects come from the teasers:** decode (characters cycle through `0-9 A-F # / _` before
  locking), a scan band, and a 2 px red/cyan split for hits and glitches.
- **Sound: low, short, filtered.** Body under 800 Hz, a short top transient for clarity, a low-pass
  around 4 kHz on everything. Intervals are fourths, fifths and minor seconds; no rising major
  arpeggios, bells or coins. Rewards sound like a lock or a confirmation, not a prize.
- **Quiet by default.** Common cues (tap, harvest, collect) are the quietest; only danger cues and
  level-up duck the music.
- **No cross shapes** anywhere, including icons and glyphs (owner rule). Reduce-motion keeps working
  as now.

## Audit and changes

### Sound (`Audio/SoundSynth.cs`, `Audio/AudioCues.cs`)

| Cue | Now | Change to |
|---|---|---|
| HarvestCrystal/Alloy/Data/Credit | Glassy two-note rise, 784–1175 Hz, plus an octave sparkle | "Intake": a low thump (90 → 60 Hz, 120 ms) under a short band-passed noise draw. Resource changes the noise colour and a faint tone (crystal glassy 1.2 kHz, alloy metallic 300 Hz ring, data a fast 3-step digital chirp, credit a dry double click), not the melody. |
| Collect, PickUp (`Coin`) | Bright coin, 2–4 kHz | "Latch": a dry mechanical click and a low 220 Hz blip, 150 ms. |
| Store | 1318–988–1318 Hz triplet | Two low blips (330, 247 Hz), soft, like a drawer closing. |
| Upgrade | E-major four-note arpeggio | Servo whine (filtered saw 150 → 300 Hz, 250 ms) ending on a low lock clunk. |
| Craft (`Install`) | Two-note rise | Short servo plus two clicks. |
| LevelUp | C-major six-note fanfare, 523–1568 Hz | 1.6 s swell like the intro chords: a 55 Hz sub with an open fifth (A1/E2) fading in, a filtered noise rise, ending on one quiet high ping (1.76 kHz, long decay). Still ducks music. |
| ShieldUp | Four-note major arpeggio | Rising filtered hum (80 → 160 Hz) with a soft "seal" thump. |
| SpikeArm | C–F–B♭ arpeggio | Three low ticks speeding up, under a thin rising hum. |
| Defused | Descending three notes | Power-down: hum falling 200 → 50 Hz with a click. |
| CooldownReady | High G–C pair (1.6–2 kHz) | One quiet sonar ping, 660 Hz, long low-passed tail. |
| ContestStart | A–A–E, 440–660 Hz | Two low horn pulses (110 Hz, slightly detuned saw, low-passed). |
| Online (NoticeInfo) | Bell chime, 1.3–2.6 kHz | Short radio squelch: filtered noise burst plus a 440 Hz blip. |
| Warning | 880–698 Hz falling pair | Same shape, an octave down, triangle with a little grit. |
| Alarm, DroneInbound | Triangle 800–1320 Hz | Keep the rhythm, move to 400–660 Hz with a low-passed saw so it reads as a klaxon, not a toy siren. |
| Tap, Back, Toggle, Panel, ViewSwitch, InRange | 1.1–2.6 kHz sine blips | Keep them short, drop to 600–1200 Hz and add a 1 ms noise click at the start, so they sound like hardware keys. |
| Hit, Boom, DroneHit, Jam, DroneIntercept, Reject, Deploy, Anchor | Noise bursts, static, sweeps | Already in the right world. Only add the shared low-pass and a little saturation. |

New building blocks in `SoundSynth`: `Thump`, `NoiseBand` (one-pole band-pass on noise), `Servo`
(filtered saw glide), `Drone` (sub plus fifth swell), `Click`, and a final `LowPass` plus soft clip in
`Clip()` for every sound. Gains in `AudioCues` drop about 15% for the common cues.

### Visual (`Ui/UiEffects.cs`, `Ui/LevelFlash.cs`, `Ui/UiBuilder.LevelUp.cs`, `Ui/ToastService.cs`, `Holo/ArNodeService.cs`, `Ui/FieldMapView.cs`)

| Effect | Now | Change to |
|---|---|---|
| Top-bar number count-up (`NumberTicker`) | Counts up 0.6 s, flashes to white, grows 12% | Counts up 0.4 s while changed digits decode; colour goes 55% → 100% of the resource colour; no scale. A 2 px underline in the resource colour wipes left to right once. |
| Gain float (`FloatingGain`) | Big heading-size "+20 CRYSTAL", arcs up then flies to the bar, shrinking | Small mono label "+20 CRYSTAL" (Label size) appears at the marker by decoding, rises 24 px and fades over 0.6 s. A thin 1 px line traces from the marker to the top-bar number in 200 ms, then the number ticks. |
| Level-up screen (`LevelUpBurst`) | Hex pops 0.2 → 1.15 → 1, ring spreads twice, slow breath, 72 pt "LEVEL UP", 3.2 s | The hex outline traces itself in (300 ms), a scan band passes down, the level number decodes, one thin ring spreads once at 0.4 alpha. Title in the teaser mono style: "// CLEARANCE RAISED" with "LEVEL 12" under it (copy to confirm). 2.4 s, still tap to dismiss. |
| Menu card level flash (`LevelFlash`) | Flashes white, grows 25%, bar refills | No scale or white. Bar refills with a scan tick at its leading edge; the number decodes. |
| TN health hit (`Shake`) | 10 px shake, 0.35 s | 3 px jitter, 0.2 s, plus a 2 px red/cyan split on the bar for 150 ms. |
| Jam overlay (`JamStatic`) | Static flicker | Keep; lower to 0.25 alpha and add thin horizontal scan lines. |
| In-range map marker (`Breathe`) | Size pulses 8% at ~0.4 Hz | Size stays; the outer ring's opacity pulses 0.45 ↔ 0.9 over 3 s. |
| Selected marker (`Spin`) | Ring spins at 90°/s | Four corner brackets close in to lock on (200 ms), then hold still. Brackets are separate L-shaped corners, no lines through the middle. |
| AR harvest burst (`ArNodeService`) | 10 shards at 0.9 alpha, spinning 400°/s, fly into the camera | 6 thin vertical slivers rise off the node and stream toward the bottom of the view, no spin, 0.5 alpha, 0.7 s; the node's floating label decodes "+20 CRYSTAL". |
| Toasts (`ToastService`) | Hex icon with "+", "i", "!" glyphs; outline at 0.75 | Drop the hex icon and the "+" (it reads as a cross). A 4 px left bar in the kind's colour and a mono tag ("// OK", "// INFO", "// WARN", "// ALERT"); outline 0.35; slide in 8 px and fade in 150 ms; the message decodes in the first 200 ms. |
| Busy sweep (`BusySweep`) | White bar at 0.85 | Accent colour at 0.5, 2 px tall. |
| Button press (`UiStyle` colour tint) | Brightens to 1.35 | Darkens the plate to 0.8 and brings the outline to full accent. |
| Map shockwave (`FieldMapView`) | Orange ring | Keep; thinner (2 px) and cap at 0.6 alpha. |
| Panel intro (`PanelIntro`) | Fade plus 40 px slide | Fine. Slide drops to 16 px. |

New helpers in `UiEffects.cs`: `DecodeText` (one component used by the ticker, gains, toasts and
level-up), `ScanBand`, `GlitchSplit`, and `CornerBrackets`. Constants (glow cap, flash floor,
durations) go in `UiStyle` under an `Effects` section so they are tuned in one place.

### Haptics

No change; they were not called out and are already short. Level-up stays a medium pulse.

## Milestones

- **E0 Rules and helpers.** `UiStyle` effects constants, `DecodeText`, `ScanBand`, `GlitchSplit`,
  `CornerBrackets`. An admin-only **Sound board** in the Admin screen that plays every `AudioCue`, so
  the owner can judge sounds on the phone without playing.
- **E1 Sounds.** New synth blocks and every cue in the table above.
- **E2 HUD motion.** Ticker, gain float, menu level flash, busy sweep, button press, panel slide.
- **E3 Level-up screen.**
- **E4 Map and AR.** Markers, selection brackets, shockwave, harvest slivers, TN panel hit and jam.
- **E5 Toasts.**
- **E6 Phone check.** Android build, owner listens to the sound board and runs a harvest, level-up
  (admin XP grant) and a TN hit; anything still "too cute" goes back to its milestone.

No API change. Reduce-motion keeps every effect still, as now.

## Open questions for the owner

1. Which effects bothered you most: the sounds, the level-up screen, the gains flying to the bar, or
   the map/AR effects? This plan changes all of them; we can start with the worst.
2. Did you also mean the startup intro or the first-TN tutorial video? Those are separate and not
   changed here.
3. Level-up title: "// CLEARANCE RAISED" or keep "LEVEL UP" in the new style?
