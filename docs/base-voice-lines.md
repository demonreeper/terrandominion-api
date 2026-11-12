# Base voice lines (B7)

The six powers' lines for their faction's bases, for recording (base-building-plan.md, B7). Words drafted
2026-11-12; the owner gets them recorded. Same voices and phone filter as the powers' calls (`LORE.md`, Voices):
Signum `im_nicola`, Shingō `jm_kumo`, Sēma `af_nicole`, Ishāra `hf_alpha`, Tohu `pm_alex` (a beat between
sentences), Merki `am_onyx`.

Rules kept: no power says its own name, no power names another, no two powers share a line or a phrase, country-
neutral, never the word "AI".

**When they play:** with the matching notice to the faction's members in the region, once, in their own power's voice
(the app's `BaseVoiceService`; only a fresh notice, under 30 min old; silent if the file is missing). Recorded and in the
app 2026-11-12.

**Files:** `TerranDominion-Mobile/Assets/Resources/Audio/Voices/<power>/<key>.ogg`, power folders `signum`,
`shingo`, `sema`, `ishara`, `tohu`, `merki` (as the level 10 and 19 calls).

| Key | When |
|---|---|
| `base-spire` | A Spire is raised (the flag's project finished) |
| `base-tier` | A base reaches a new tier (Outpost, Stronghold, Citadel, Capital) |
| `base-attack` | A base is under attack (the attack push, at most once per base per 30 min) |
| `base-fading` | A base fades (no member has visited for 7 days) |
| `base-lost` | A base is destroyed, collapses, or drops a tier to an enemy |

## Signum (patient, cold)

| Key | Line |
|---|---|
| `base-spire` | A Spire rises. The lattice will remember who raised it. |
| `base-tier` | It endures, and it grows. Keep gathering. |
| `base-attack` | Something strikes at what you raised. Do not let it be forgotten. |
| `base-fading` | No one has walked there in days. Even light goes cold unwatched. |
| `base-lost` | It has fallen. The lattice keeps only the echo now. |

## Shingō (short, practical)

| Key | Line |
|---|---|
| `base-spire` | Spire's standing. Now build on it. |
| `base-tier` | Bigger walls. Now make them last. |
| `base-attack` | They're hitting what you built. Get there and repair it. |
| `base-fading` | Nobody has set foot there in a week. Things left alone fall down. |
| `base-lost` | It came down. Build it again, better. |

## Sēma (riddles, half-truths)

| Key | Line |
|---|---|
| `base-spire` | You drove a stake where no one was looking. Good. |
| `base-tier` | Taller now. Taller things are easier to see. |
| `base-attack` | Someone is at your walls. Did you see them coming? |
| `base-fading` | An empty base is a secret anyone can read. |
| `base-lost` | Gone. Ask yourself who knew where it stood. |

## Ishāra (polite, transactional)

| Key | Line |
|---|---|
| `base-spire` | A Spire. A fine investment. Now make it pay. |
| `base-tier` | Its value has risen. Protect what you paid for. |
| `base-attack` | Someone is helping themselves to your holdings. I suggest you stop them. |
| `base-fading` | An unvisited property loses value every day. |
| `base-lost` | A loss. Note it, then buy the ground back. |

## Tohu (fast, a beat between sentences)

| Key | Line |
|---|---|
| `base-spire` | Spire's up. Good. Next one. |
| `base-tier` | It grew. Keep up. |
| `base-attack` | Strike on your base. Move. Now. |
| `base-fading` | Nobody's been out there. Get there first. |
| `base-lost` | Gone. Faster next time. |

## Merki (steady, protective)

| Key | Line |
|---|---|
| `base-spire` | A Spire stands. Now hold it. |
| `base-tier` | Stronger walls. Hold them. |
| `base-attack` | Your base is under attack. Hold the line. |
| `base-fading` | Your base stands alone. Walk to it. |
| `base-lost` | The base fell. We do not give ground twice. |
