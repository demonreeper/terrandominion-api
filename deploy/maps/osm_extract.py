"""Extracts what the node seeder stands nodes on from the map's OSM extract, offline.

Writes one JSON file for `tdapi import-osm` (scripts/import-osm.ps1 runs both):
  land     islands: the natural=coastline ways joined into closed rings (as make_ocean.py does),
           plus closed place=island/islet ways
  walk     walkable ways as lines: footways, paths, pedestrian streets, steps, residential, service,
           living-street, unclassified and tertiary roads, and the outlines of parks and squares
  blocked  where a node must never stand: buildings, inland water, aprons, and runways and
           taxiways (lines, widened by "buf" metres on import)
  sensitive  where no faction base or flag may stand near (base-building-plan.md section 1): places of
           worship, cemeteries, schools (with kindergartens, childcare, colleges and universities, owner
           2026-11-12), hospitals, police, military and government sites, as areas
           (closed ways) or points (tagged nodes)

Pure Python, no GDAL/osmium, using the PBF reader in make_ocean.py. Multipolygon relations are
skipped (few in the Maldives); their outer ways are often tagged and closed anyway.
Usage: python3 osm_extract.py maldives.osm.pbf out.json
"""
import json
import os
import struct
import sys
import zlib

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from make_ocean import delta, fields, join_rings, packed, zigzag  # noqa: E402

WALK_HIGHWAYS = {
    "footway", "path", "pedestrian", "steps", "living_street", "residential", "service",
    "unclassified", "tertiary", "cycleway", "track",
}
WALK_AREAS = {("leisure", "park"), ("leisure", "garden"), ("place", "square"), ("leisure", "playground")}
NO_ACCESS = {"no", "private"}
LAND_PLACES = {"island", "islet"}
RUNWAY_BUFFER_M = {"runway": 30, "taxiway": 15}
SENSITIVE_TAGS = [
    ("amenity", "place_of_worship", "worship"), ("building", "mosque", "worship"), ("building", "church", "worship"),
    ("building", "temple", "worship"), ("building", "shrine", "worship"), ("building", "synagogue", "worship"),
    ("landuse", "religious", "worship"),
    ("landuse", "cemetery", "cemetery"), ("amenity", "grave_yard", "cemetery"),
    ("amenity", "school", "school"), ("building", "school", "school"),
    ("amenity", "kindergarten", "school"), ("building", "kindergarten", "school"), ("amenity", "childcare", "school"),
    ("amenity", "college", "school"), ("building", "college", "school"),
    ("amenity", "university", "school"), ("building", "university", "school"),
    ("amenity", "hospital", "hospital"), ("building", "hospital", "hospital"),
    ("amenity", "police", "police"),
    ("landuse", "military", "military"), ("military", None, "military"),
    ("office", "government", "government"), ("government", None, "government"), ("amenity", "townhall", "government"),
    ("amenity", "courthouse", "government"), ("building", "government", "government"),
]
SENSITIVE_KEYS = {k for k, _, _ in SENSITIVE_TAGS}


def sensitive_kind(tags):
    """The sensitive kind a feature's tags name, or None. A value of None matches any value but "no"."""
    for k, v, kind in SENSITIVE_TAGS:
        have = tags.get(k)
        if have is not None and (have == v or (v is None and have != "no")):
            return kind
    return None


def node_tags(strings, keys_vals):
    """Splits DenseNodes keys_vals (k, v, k, v, 0, ...) into one tag dict per node."""
    out, cur, i = [], {}, 0
    while i < len(keys_vals):
        k = keys_vals[i]
        if k == 0:
            out.append(cur)
            cur, i = {}, i + 1
            continue
        cur[strings[k]] = strings[keys_vals[i + 1]]
        i += 2
    return out


def read_ways(path):
    """Returns ({node_id: (lon, lat)}, [(tags, refs)] for every way with tags, [(tags, (lon, lat))] for every
    node with a sensitive tag)."""
    data = open(path, "rb").read()
    i = 0
    nodes = {}
    ways = []
    points = []
    while i < len(data):
        (hlen,) = struct.unpack(">I", data[i:i + 4])
        i += 4
        header = dict((fn, v) for fn, _, v in fields(data[i:i + hlen]))
        i += hlen
        size = header[3]
        blob = dict((fn, v) for fn, _, v in fields(data[i:i + size]))
        i += size
        if header[1].decode() != "OSMData":
            continue
        raw = zlib.decompress(blob[3]) if 3 in blob else blob[1]

        strings, groups = [], []
        gran, lat_off, lon_off = 100, 0, 0
        for fn, _, v in fields(raw):
            if fn == 1:
                strings = [s.decode("utf-8", "replace") for f, _, s in fields(v) if f == 1]
            elif fn == 2:
                groups.append(v)
            elif fn == 17:
                gran = v
            elif fn == 19:
                lat_off = v
            elif fn == 20:
                lon_off = v

        for group in groups:
            for fn, _, v in fields(group):
                if fn == 2:  # DenseNodes
                    d = dict((f, x) for f, _, x in fields(v))
                    ids = delta(packed(d.get(1, b"")))
                    lats = delta(packed(d.get(8, b"")))
                    lons = delta(packed(d.get(9, b"")))
                    tagged = node_tags(strings, packed(d.get(10, b""))) if d.get(10) else []
                    for n, (nid, la, lo) in enumerate(zip(ids, lats, lons)):
                        nodes[nid] = ((lon_off + gran * lo) / 1e9, (lat_off + gran * la) / 1e9)
                        if n < len(tagged) and tagged[n] and SENSITIVE_KEYS & tagged[n].keys():
                            points.append((tagged[n], nodes[nid]))
                elif fn == 1:  # plain Node
                    d = dict((f, x) for f, _, x in fields(v))
                    nodes[zigzag(d[1])] = ((lon_off + gran * zigzag(d[9])) / 1e9,
                                           (lat_off + gran * zigzag(d[8])) / 1e9)
                    keys, vals = packed(d.get(2, b"")), packed(d.get(3, b""))
                    tags = dict((strings[k], strings[v]) for k, v in zip(keys, vals))
                    if SENSITIVE_KEYS & tags.keys():
                        points.append((tags, nodes[zigzag(d[1])]))
                elif fn == 3:  # Way
                    d = {}
                    for f, _, x in fields(v):
                        d[f] = x
                    keys = packed(d.get(2, b""))
                    vals = packed(d.get(3, b""))
                    if not keys:
                        continue
                    tags = dict((strings[k], strings[val]) for k, val in zip(keys, vals))
                    ways.append((tags, delta(packed(d.get(8, b"")))))
    return nodes, ways, points


def coords(nodes, refs):
    pts = [nodes[r] for r in refs if r in nodes]
    return [[round(lon, 7), round(lat, 7)] for lon, lat in pts]


def line(pts):
    return {"type": "LineString", "coordinates": pts}


def polygon(pts):
    return {"type": "Polygon", "coordinates": [pts]}


def main(pbf, out):
    nodes, ways, points = read_ways(pbf)
    land, walk, blocked, sensitive = [], [], [], []
    for tags, (lon, lat) in points:
        kind = sensitive_kind(tags)
        if kind:
            sensitive.append({"k": kind, "g": {"type": "Point", "coordinates": [round(lon, 7), round(lat, 7)]}})
    coast = []
    for tags, refs in ways:
        closed = len(refs) >= 4 and refs[0] == refs[-1]
        if tags.get("natural") == "coastline":
            coast.append((0, refs))
            continue
        pts = coords(nodes, refs)
        if len(pts) < 2:
            continue
        # A sensitive place is recorded as well as whatever else the way is (a school is also a building).
        kind = sensitive_kind(tags)
        if kind:
            g = polygon(pts) if closed and len(pts) >= 4 else line(pts)
            sensitive.append({"k": kind, "g": g})
        if closed and tags.get("place") in LAND_PLACES and len(pts) >= 4:
            land.append({"k": tags["place"], "g": polygon(pts)})
            continue
        hw = tags.get("highway")
        if hw in WALK_HIGHWAYS and tags.get("access") not in NO_ACCESS and "aeroway" not in tags:
            walk.append({"k": hw, "g": line(pts)})
            continue
        area = next((v for k, v in WALK_AREAS if tags.get(k) == v), None)
        if area and closed:
            walk.append({"k": area, "g": line(pts)})
            continue
        if closed and len(pts) >= 4 and tags.get("building", "no") != "no":
            blocked.append({"k": "building", "g": polygon(pts)})
            continue
        if closed and len(pts) >= 4 and (tags.get("natural") == "water" or tags.get("landuse") in ("reservoir", "basin")):
            blocked.append({"k": "water", "g": polygon(pts)})
            continue
        aero = tags.get("aeroway")
        if aero == "apron" and closed and len(pts) >= 4:
            blocked.append({"k": "apron", "g": polygon(pts)})
        elif aero in RUNWAY_BUFFER_M:
            if closed and len(pts) >= 4 and tags.get("area") == "yes":
                blocked.append({"k": aero, "g": polygon(pts)})
            else:
                blocked.append({"k": aero, "g": line(pts), "buf": RUNWAY_BUFFER_M[aero]})

    rings, leftovers = join_rings(coast)
    for ring in rings:
        pts = coords(nodes, ring)
        if len(pts) >= 4 and pts[0] == pts[-1]:
            land.append({"k": "coastline", "g": polygon(pts)})

    with open(out, "w", encoding="utf-8") as f:
        json.dump({"land": land, "walk": walk, "blocked": blocked, "sensitive": sensitive}, f, separators=(",", ":"))
    kinds = {}
    for w in walk:
        kinds[w["k"]] = kinds.get(w["k"], 0) + 1
    print("land %d (coastline rings %d, unclosed chains %d dropped), walkways %d, blocked %d" %
          (len(land), len(rings), len(leftovers), len(walk), len(blocked)))
    skinds = {}
    for x in sensitive:
        skinds[x["k"]] = skinds.get(x["k"], 0) + 1
    print("sensitive places:", ", ".join("%s %d" % kv for kv in sorted(skinds.items(), key=lambda kv: -kv[1])))
    print("walkways by kind:", ", ".join("%s %d" % kv for kv in sorted(kinds.items(), key=lambda kv: -kv[1])))
    print("written to", out)


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
