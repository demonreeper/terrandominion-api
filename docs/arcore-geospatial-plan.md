# ARCore Geospatial plan

Status: APPROVED to start (owner, 2026-11-14): G0 owner steps, then G1 probe.
Build prompt: `arcore-geospatial-build-prompt.md` (same folder).

## Why

AR things are placed by GPS today: `Holo/GeoAnchor.cs` pins a GPS origin in the AR world, north comes from the compass
corrected by the walked path (`Holo/PathFit.cs`, `WalkedNorth`), height from GPS + barometer (`Holo/GroundHeight.cs`)
and AR planes (`FloorLevel`, `LocalGround`). Best case that is about 4 m and a few degrees, after a short walk.

Google's ARCore Geospatial API gives the camera's latitude, longitude, altitude and heading. Where Google has Street
View imagery it localises from the camera picture (VPS): typically under 1 to 3 m and 1 to 3 degrees, in seconds,
standing still, also up on a balcony. Where it has none it falls back to GPS + compass, which is no better than ours.

## The rule (unchanged)

- **AR is never forced, Geospatial is never needed.** It is an extra source of position, north and ground height
  where it has coverage. The GPS path stays the fallback everywhere and is never removed. ARCore stays Optional.
- Phones without Geospatial (no ARCore, old ARCore, no network) play exactly as today.
- Looks only, like the AR ground work: it never goes to the server and never decides a game rule. (Whether "precise
  AR" later earns an advantage is its own decision, Open.)

## Coverage: the big unknown

VPS works where Google Street View has imagery. I found no evidence of Street View on the streets of Malé or
Hulhumalé (only Addu City roads, 2020, and one resort island). If there is none, Geospatial in the Maldives gives
little over what we have, and the real win is in later countries. So the first milestone is a coverage test, and
it decides how far we go now.

Quick check the owner can do today, no build: in Google Maps, start **walking** directions somewhere in Malé, then
**Live View**. Live View uses the same VPS: if it locks on and shows arrows, VPS covers that street. Also look at
Street View's blue lines on the map (drag the yellow figure).

## Design

One placement path stays: everything still goes through `ArGroundService.PlaceAt` and `GeoAnchor`. Geospatial is
fed in as a better source, so every existing feature (bases, TNs, nodes, echoes, sky battles, attack bar) gets it
for free and falls back on its own when tracking is lost.

1. **Position.** While Earth tracking is good (state Tracking, horizontal accuracy within `GeoGoodM`, first guess
   3 m), each frame's `CameraGeospatialPose` is a fix for `GeoAnchor.Step`, with the camera's AR position **of that
   same frame** (exact, no `EyeHistory` lag) and its own accuracy as `best` (so the pin may be surer than 4 m,
   floor 0.5 m). GPS fixes still step too; their weight against a sure pin is already tiny.
2. **North.** The pose's rotation (`EunRotation`, east-up-north) and the camera's AR rotation give north's AR yaw
   directly: `northYaw = (cameraRotation * Inverse(EunRotation)).eulerAngles.y`. Offered to `WalkedNorth` with the
   pose's `OrientationYawAccuracy` as its doubt (good when within `GeoYawGoodDeg`, first guess 5 degrees). The
   change goes through `GeoAnchor.Turn` as today. Standing still now gets a true north.
3. **Ground.** Geospatial altitude is above the WGS84 ellipsoid, not sea level (around the Maldives the two differ by
   roughly 100 m), so do not mix it into `GroundHeight`'s sea-level maths. Instead use **terrain anchors**:
   `ResolveAnchorOnTerrainAsync(lat, lon, 0, rotation)` puts an anchor on Google's terrain; its AR `y` is the real
   ground's AR height. One under the player (re-resolved when the player has moved 30 m) gives the ground for
   up-high detection and the street floor guess; one at each base in AR (the nearest two) gives that base its own
   ground when no AR plane there was seen. Plane-based `LocalGround` still wins within 12 m.
4. **Hand-over.** A pure `GeoSource` decides the source with hysteresis: Geospatial after 2 s good, back to GPS after
   3 s poor, so one bad frame does not make things jump. Logged as "Geo: ..." every 5 s and shown on the dev AR GPS
   line (`VPS 1.2 m 2°` / `GPS ±4 m`).
5. **Not now (Open):** Geospatial anchors per object (own TN/base pinned by Google), Streetscape Geometry (building
   meshes, for occlusion past depth's 12 m and from high up), rooftop anchors, any gameplay advantage for precise AR.

## Milestones

| | What | Done when |
|---|---|---|
| G0 | **Owner setup + coverage check** (no build): Live View test in Malé / Hulhumalé / Villimalé; enable ARCore API in the Google Cloud project; iOS API key (see "Owner steps") | Owner reports where Live View worked |
| G1 | **Package + probe.** ARCore Extensions (AR Foundation 6 build, `arf6`) added; `ARCoreExtensions` + config (Geospatial on, rest off) and `AREarthManager` built in `ARRigBuilder` in code; keyless auth on Android. Dev-only "GEO" line and log: support, Earth state, tracking state, VPS availability at the spot (`CheckVpsAvailabilityAsync`), horizontal / vertical / yaw accuracy. Nothing placed by it yet | Owner walks the test spots + the 24th-floor balcony; we read the numbers. **Gate:** if VPS is missing everywhere in the Maldives, stop after G1 + G5 (keep the probe) and park G2-G4 for the next country |
| G2 | **Position and north** from Geospatial (Design 1, 2, 4), unit tests for `GeoSource` and the north maths | Things snap into place standing still where VPS covers; unchanged elsewhere |
| G3 | **Ground from terrain anchors** (Design 3), player + nearest two bases | Base from the balcony stands on the street; floor right before any plane is seen |
| G4 | **iOS parity.** ARCore Extensions iOS support (Geospatial pod through EDM), API key auth from a git-ignored file / env var, Codemagic's `pod install` must succeed (today a pod failure is tolerated, which would now drop Geospatial or break the link) | Geospatial line shows on an iPhone TestFlight build |
| G5 | **Disclosure.** Google requires apps using ARCore / Geospatial to disclose it: privacy policy (API repo legal pages) and a line under Settings > USE AR VIEW | Pages live |

## Owner steps (Google Cloud)

Use the same Google Cloud project as Firebase / Google sign-in.

1. APIs & Services > Library > **ARCore API** > Enable. (No charge as far as Google's docs say; quota shows on the API's page.)
2. **Android, keyless (recommended, no key in the app):** ARCore uses the Android OAuth clients you already have
   (package `com.hisstudios.terrandominion` with the debug and upload SHA-1, from the sign-in setup). Add one more
   Android OAuth client with the **Play App Signing SHA-1** (0B:D4:C7...D0:20) so Play-installed builds work too.
3. **iOS (G4):** Credentials > Create API key, restrict it to the iOS app (bundle id) and to ARCore API only. Keep it
   out of git: put it on the PC as `References\temp\arcore-ios-key.txt` (the build reads it) or as a Codemagic
   secret. A restricted iOS key ships inside the app anyway, but it still stays out of the repo.

## Risks and gotchas

- **Coverage** may be nil in the Maldives (above). The probe tells us for sure.
- **Package clash:** ARCore Extensions pulls in Play services location / EDM; the project already has EDM 1.2.190
  (Firebase) and `TDLocation.androidlib` uses fused location. Use the one EDM; resolve version clashes, don't duplicate.
- **Background shaders:** our occlusion shader copies (`TdArBackground*.shader`) stay as they are; Extensions does
  not replace the camera background. Check depth occlusion still works after adding it.
- **Network + battery:** VPS uses data while AR is up. Earth tracking runs only while the AR session runs.
- **Altitude:** never subtract an ellipsoid altitude from a sea-level one (about 100 m out here). Terrain anchors
  avoid it.
- **North convention:** Unity yaw is clockwise from +Z; `NorthYawWorld + bearing` gives a direction. Prove the
  `EunRotation` maths with an edit-mode test and a log line against the compass before trusting it.

## Open decisions

- Whether precise (VPS) AR gives any advantage or AR-only feature (default: no, looks only).
- Thresholds: `GeoGoodM` 3 m, `GeoYawGoodDeg` 5 degrees, hand-over 2 s / 3 s (first guesses, tune on the street).
- G6 later: Geospatial anchors for your own TN / base; Streetscape Geometry for occlusion.
