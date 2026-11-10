# Prompt for local Claude Code: effects restyle

Paste this into Claude Code in `D:\Development\Github\ProjectAR\TerranDominion-Mobile`.

---

Read `..\TerranDominion-API\docs\effects-restyle-plan.md` (copy also in the project files as
`changes/effects-restyle-plan.md`). It restyles the polish-pass sounds and visual effects so they fit
Terran Dominion's dark, tactical look instead of feeling childish. Follow its style rules exactly:
no overshoot/bounce/scale pops, no flash to white, glows capped at 0.55 alpha, sounds low and
filtered with no rising major arpeggios, bells or coins, and no cross shapes anywhere (the success
toast's "+" glyph goes).

Work on a branch `effects-restyle`, one commit per milestone, E0 to E5 in order:

- E0: effects constants in `UiStyle`, the `DecodeText`, `ScanBand`, `GlitchSplit` and
  `CornerBrackets` helpers in `Ui/UiEffects.cs`, and an admin-only Sound board in the Admin screen
  that lists and plays every `AudioCue`.
- E1: the new synth blocks and every cue change in the plan's sound table (`Audio/SoundSynth.cs`,
  gains in `Audio/AudioCues.cs`).
- E2: HUD motion (`NumberTicker`, `FloatingGain`, `LevelFlash`, `BusySweep`, button press tint,
  `PanelIntro` slide).
- E3: level-up screen (`Ui/UiBuilder.LevelUp.cs` `LevelUpBurst`). Keep the title "LEVEL UP" until
  I confirm the new copy.
- E4: map markers (`Breathe`, `Spin` → corner brackets, shockwave in `Ui/FieldMapView.cs`), AR
  harvest burst in `Holo/ArNodeService.cs`, TN panel `Shake` and jam static.
- E5: toasts in `Ui/ToastService.cs`.

Keep reduce-motion working for every effect. Build after each milestone and fix compile errors.
Don't push or open a PR until I say so. When done, make an Android test build so I can check the
Sound board, a harvest, a level-up (admin XP grant) and a TN hit on my phone.
