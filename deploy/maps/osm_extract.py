"""Extracts what the node seeder stands nodes on from the map's OSM extract, offline.

Writes one JSON file for `tdapi import-osm` (scripts/import-osm.ps1 runs both):
  land     islands: the natural=coastline ways joined into closed rings (as make_ocean.py does),
           plus closed place=island/islet ways
  walk     walkable ways as lines: footways, paths, pedestrian streets, steps, residential, service,
           living-street, unclassified and tertiary roads, and the outlines of parks and squares
  blocked  where a node must never stand: buildings, inland water, aprons, and runways and
           taxiways (lines, widened by "buf" metres on import)

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


def read_ways(path):
    """Returns ({node_id: (lon, lat)}, [(tags, refs)] for every way with tags)."""
    data = open(path, "rb").read()
    i = 0
    nodes = {}
    ways = []
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
                    for nid, la, lo in zip(ids, lats, lons):
                        nodes[nid] = ((lon_off + gran * lo) / 1e9, (lat_off + gran * la) / 1e9)
                elif fn == 1:  # plain Node
                    d = dict((f, x) for f, _, x in fields(v))
                    nodes[zigzag(d[1])] = ((lon_off + gran * zigzag(d[9])) / 1e9,
                                           (lat_off + gran * zigzag(d[8])) / 1e9)
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
    return nodes, ways


def coords(nodes, refs):
    pts = [nodes[r] for r in refs if r in nodes]
    return [[round(lon, 7), round(lat, 7)] for lon, lat in pts]


def line(pts):
    return {"type": "LineString", "coordinates": pts}


def polygon(pts):
    return {"type": "Polygon", "coordinates": [pts]}


def main(pbf, out):
    nodes, ways = read_ways(pbf)
    land, walk, blocked = [], [], []
    coast = []
    for tags, refs in ways:
        closed = len(refs) >= 4 and refs[0] == refs[-1]
        if tags.get("natural") == "coastline":
            coast.append((0, refs))
            continue
        pts = coords(nodes, refs)
        if len(pts) < 2:
            continue
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
        json.dump({"land": land, "walk": walk, "blocked": blocked}, f, separators=(",", ":"))
    kinds = {}
    for w in walk:
        kinds[w["k"]] = kinds.get(w["k"], 0) + 1
    print("land %d (coastline rings %d, unclosed chains %d dropped), walkways %d, blocked %d" %
          (len(land), len(rings), len(leftovers), len(walk), len(blocked)))
    print("walkways by kind:", ", ".join("%s %d" % kv for kv in sorted(kinds.items(), key=lambda kv: -kv[1])))
    print("written to", out)


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
