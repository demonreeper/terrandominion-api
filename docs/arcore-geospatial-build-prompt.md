# Build prompt: ARCore Geospatial, G1 (probe) then G2-G3

For the owner's local Claude Code in `D:\Development\Github\ProjectAR`. Plan: `arcore-geospatial-plan.md` (same
folder; API repo `docs/arcore-geospatial-plan.md` once merged). Read it, plus `TerranDominion-Mobile/CLAUDE.md`,
`docs/architecture.md` (AR ground, AR north) first.

Before starting: `git pull` both repos. Mobile main has a local build-number bump in `ProjectSettings.asset` (208);
keep the higher number. Work on a branch `feat/arcore-geospatial`. Don't commit to main or push without the owner's go.

## Rules that hold throughout

- AR is never forced and Geospatial is never needed: every phone without it plays exactly as today. ARCore stays
  Optional, Depth stays Optional (`Assets/XR/Settings/ARCoreSettings.asset`).
- Everything is built in code (no scenes or prefabs): add the Extensions components in `Ar/ARRigBuilder.cs` the way
  the managers are added (disabled, fields set, then enabled). ARCore types stop in `Ar/` and `Holo/`.
- Looks only: nothing from Geospatial goes to the server or decides a rule.
- No secrets in git. Android uses keyless auth; the iOS key is read at build time from a git-ignored file.

## G1: package and probe (build this now)

1. Add **ARCore Extensions for AR Foundation 6** (the `arf6` release; newest tag on
   github.com/google-ar/arcore-unity-extensions releases, 1.48 or later) as a tarball in `Packages/` like Firebase,
   or the git URL `#arf6` pinned to a tag. Keep the one External Dependency Manager (1.2.190); resolve Play services
   version clashes with `TDLocation.androidlib` rather than adding a second copy.
2. Project settings for Extensions: Android authentication **Keyless**, iOS **API key** (empty in git; see G4),
   iOS support off for now. Geospatial on; Cloud Anchors, Semantics, Streetscape Geometry off.
3. `ARRigBuilder`: add `ARCoreExtensions` (session, origin, camera manager wired) with an
   `ARCoreExtensionsConfig` made by `ScriptableObject.CreateInstance` (`GeospatialMode.Enabled`), and an
   `AREarthManager`. Make sure the occlusion manager is still added before `ARCameraBackground` and our background
   shader copies still draw (check depth occlusion still works on the phone).
4. New `Ar/ArGeospatialService.cs` (registered after `ArSessionService`, before `ArGroundService`): each frame while
   the AR session runs, read `IsGeospatialModeSupported`, `EarthState`, `EarthTrackingState`, and when tracking
   `CameraGeospatialPose` (lat, lon, altitude, `HorizontalAccuracy`, `VerticalAccuracy`, `OrientationYawAccuracy`,
   `EunRotation`). Once per 50 m moved, call `AREarthManager.CheckVpsAvailabilityAsync(lat, lon)` and keep the
   answer. Expose these as plain properties (no ARCore types outside `Ar/`).
5. Dev server only: a GEO line on the AR GPS diagnostics: `GEO VPS yes|no|? · TRACKING 1.2 m ±1.8 v · 2.1°` or the
   Earth state when not tracking (e.g. `GEO ERROR_NOT_AUTHORIZED`). Log `[ProjectAR] Geo: ...` every 5 s with the
   same plus the compass north and the Geospatial north (see G2 maths) and their difference.
6. Edit-mode tests stay green; build 2xx with `build-apk.bat`, install, and tell the owner what to look at:
   the GEO line at home, on a Malé street, in Hulhumalé, and on the 24th-floor balcony. Ask the owner for the
   numbers (or `adb logcat -s Unity | findstr "Geo:"`). Stop here and report. G2 only if the owner says go.

## G2: position and north (after the coverage gate)

- Pure `Holo/GeoSource.cs` (edit-mode tests): picks Geospatial when tracking with horizontal accuracy within 3 m
  and yaw accuracy within 5 degrees for 2 s; back to GPS after 3 s worse. Constants named, first guesses.
- In `ArGroundService.StepAnchor`, while Geospatial is the source: each frame call `m_Anchor.Step(cameraPosNow,
  pose.lat, pose.lon, north, dt, pose.HorizontalAccuracy, best: Mathf.Max(0.5f, pose.HorizontalAccuracy))` with the
  camera position of the **same** frame (no `EyeHistory`). GPS fixes keep stepping as today.
- North: `northYaw = (cameraRotation * Quaternion.Inverse(EunRotation)).eulerAngles.y` (Unity yaw, clockwise from
  +Z; `NorthYawWorld + bearing` is a direction). Prove the convention in a test with a made-up pose, and check it
  against the compass in the log before wiring. Offer it to `WalkedNorth.Offer(yaw, yawAccuracy)` so the existing
  glide and `GeoAnchor.Turn` apply. If `WalkedNorth`'s trust model caps how much it follows, give Geospatial its own
  path rather than bending the walking maths.
- The AR GPS line shows the source: `VPS 1.2 m 2°` or `GPS ±4 m`.

## G3: ground from terrain anchors

- Do not mix Geospatial altitude (WGS84 ellipsoid) with `GroundHeight`'s sea-level maths (about 100 m apart here).
- While Geospatial is the source: resolve a terrain anchor (`ResolveAnchorOnTerrainAsync(lat, lon, 0, identity)`)
  under the player, again after 30 m moved; its AR y is the ground. Use it for up high (`eye.y - groundY`, with
  vertical accuracy within 2 m) ahead of GPS + barometer, and as the street floor guess before any plane is seen.
- One terrain anchor at each base in AR (nearest two), its y as that base's ground when `LocalGround` has no plane.
  Remove anchors that are no longer used. Planes within 12 m still win.

## When done

Update `docs/architecture.md` (an "AR Geospatial" row), `CLAUDE.md` gotchas (altitude datum, keyless auth, the
probe line), `docs/WORKSPACE.md` status. Report to the owner what to check on the street. No commit or push to
main without the owner's go.
