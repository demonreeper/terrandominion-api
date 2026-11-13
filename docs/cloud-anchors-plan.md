# Player-placed reference points (Cloud Anchors) plan

Status: PROPOSED (2026-11-14, owner: "can we improve this by actually going and placing it on ground myself?",
"extract data while players place things on ground and use it ... like our own VPS streetmap"). C1 is being built
as a dev-only probe; everything after it waits for the owner's decisions below.
Related: `docs/arcore-geospatial-plan.md` (Geospatial; no VPS coverage was found in Hulhumalé on 14 Nov 2026).

## Why

AR things are placed by GPS (about 4 m at best) and north from the compass corrected by walking. Google's
Geospatial API would fix that where Street View imagery exists, and the Maldives has none where we tested.
A map tap is already as accurate as GPS, so "placing it myself" does not help through better coordinates. It helps
through what the camera sees at the spot.

**Cloud Anchors** (same ARCore Extensions package, already in the project): a phone scans a spot for about
30 seconds, Google stores a 3D feature map of it and returns an id. Any phone that later looks at the same spot
gets the anchor's pose to centimetres. No Street View needed.

**The idea:** every anchor a player places has known coordinates. A phone that resolves one knows exactly where it
is and which way it faces, so that one lock corrects the GPS pin and north for everything nearby, not only the
anchored thing. The more players place, the more of the map is covered: a positioning network of our own, with
Google only storing the scans.

## What Google allows (read 14 Nov 2026, developers.google.com/ar/develop/unity-arf/cloud-anchors/developer-guide)

| | |
|---|---|
| Anchors per project | unlimited |
| Host requests | 30 a minute, per IP address and project |
| Resolve requests | 300 a minute, per IP address and project |
| Operations at once on a phone | 40 |
| Lifetime | over 1 day needs keyless authorisation (we use it); with an API key 24 hours. The Management API's example shows a maximum of one year from creation; the lifetime can be extended to that maximum by the server (service account, scope `arcore.management`) |
| What Google keeps | "Visual data uploaded to the cloud when hosting an anchor is discarded within 24 hours, and previously uploaded visual data is never sent to a user's device." The feature map lives until the anchor expires |
| If Google ends it | announced in the release notes, run for about a year after ("commercially reasonable efforts") |
| Price | not stated on the pages read; check the ARCore terms before launch (Open) |
| Range | not stated; "best used for room-scale AR": the phone must look at the scanned area from nearby |

Not a true VPS of our own: Google keeps the scans, we keep ids and coordinates. A real image-based VPS (uploading
players' camera pictures, reconstructing streets on our servers) is out of scope: storage, compute, and pictures of
streets and people in every country.

## Limits

- Only near the spot. It does nothing for the view from a balcony.
- Needs network when scanning and when resolving.
- A changed scene (construction, parked cars, night against day) can stop a match.
- Anchors expire (a year at most) and must be scanned again.
- iOS needs the Extensions' iOS support and an API key or token (Geospatial plan G4 covers the same setup).

## Milestones

| | What | Done when |
|---|---|---|
| C1 | **Probe, dev only, nothing to the server.** In the AR view on the dev server: SCAN SPOT hosts an anchor on the floor ahead (with the scan-quality hint) and saves its id with the GPS pin's coordinates for that point on the phone. Near a saved spot the app resolves it and logs and shows how far the GPS pin's guess is from where the anchor really is, and how far north is off. No game object uses it | The owner walks, scans spots, comes back at another time and from another direction, and we read: does it resolve, from how far, how big the GPS error really is |
| C2 | **Reference points correct the pin and north.** A resolved anchor is a fix for `GeoAnchor` accurate to centimetres relative to its own coordinates, and its stored facing gives north; hand-over like `GeoSource`. Still phone-only data | Things near a scanned spot stop moving and stand in the same place every visit |
| C3 | **Server.** Table of reference points (id, coordinates, facing, ground height, who, when, expiry); nearby ones come with the field scan; the server extends lifetimes. Placing a Tether Node in AR hosts one (the player already stands there and scans the floor) | Another player's phone locks on to a spot the owner scanned |
| C4 | **Refining.** Each resolve reports the GPS position it saw for the anchor; the server averages them, so an anchor's coordinates get better than any single fix. Also kept per place: the ground's height, how far the compass is off, how good GPS is | An anchor's coordinates stop moving after enough visits |
| C5 | **Bases and disclosure.** A base's reference point (scanned by a member on site), the privacy pages, a line in Settings | Pages live |

## Open decisions (owner)

- Who may scan a spot: anyone, only when placing their own Tether Node, or members for their base.
- Whether scanning earns anything (it is work for the player), and whether locking on gives any advantage, or it is
  looks only like the rest of the AR anchoring.
- What goes to the server and what the privacy pages say: today nothing from AR anchoring leaves the phone. C3 sends
  the anchor id, coordinates and correction numbers (no pictures; Google gets the camera's feature data while
  scanning).
- What happens when an anchor expires or stops matching: silently fall back to GPS (default), or ask players near
  it to scan again.
- Price and terms of the ARCore API at scale: confirm before launch.
- Whether bad or hostile scans need moderation (a scan in the wrong place would move things for others): C4's
  averaging and a sanity check against GPS (reject an anchor more than 15 m from its claimed spot) are the default.
