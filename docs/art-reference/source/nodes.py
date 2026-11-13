# Resource node mock-ups (owner, 2026-11-13): grounded designs per resource, to approve before the models are made.
# Crystal: an outcrop out of a cracked rock. Alloy: an ore vein of half-buried plates. Data: a low relay terminal with
# its cubes just above. Credit: a sealed cache plate with hex chips. Each has a glow ring where it meets the ground.
# Self-contained (procedural materials, no texture files), Blender 4.5:
#   blender -b -P nodes.py -- <shot> <samples> <outdir>     shot: lineup | crystal | alloy | data | credit | all
import bpy, bmesh, math, random, sys, os
from mathutils import Vector, Matrix

ARGS = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
SHOT = ARGS[0] if len(ARGS) > 0 else 'all'
SAMPLES = int(ARGS[1]) if len(ARGS) > 1 and ARGS[1].isdigit() else 96
OUT = ARGS[2] if len(ARGS) > 2 else os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'nodes')


def lin(hx):
    c = tuple(((int(hx[i:i + 2], 16) / 255) ** 2.2) for i in (0, 2, 4)); m = max(c); return tuple(x / m for x in c)


RES = {'crystal': lin('59d9ff'), 'alloy': lin('ffb840'), 'data': lin('b885ff'), 'credit': lin('66f28c')}


def reset():
    global _HEX
    _HEX = None
    bpy.ops.wm.read_factory_settings(use_empty=True)
    return bpy.context.scene


# ---------------- materials ----------------
def mat(name):
    m = bpy.data.materials.new(name); m.use_nodes = True
    nt = m.node_tree; nt.nodes.clear(); out = nt.nodes.new('ShaderNodeOutputMaterial'); return m, nt, out


def hull(name, col=(0.05, 0.055, 0.07), rough=.42, metal=.65, panel=6.0, grime=True):
    """Dark sci-fi metal: noise variation and panel lines from a Voronoi distance."""
    m, nt, out = mat(name); N = nt.nodes.new; L = nt.links.new
    b = N('ShaderNodeBsdfPrincipled'); b.inputs['Metallic'].default_value = metal; b.inputs['Roughness'].default_value = rough
    tc = N('ShaderNodeTexCoord')
    no = N('ShaderNodeTexNoise'); no.inputs['Scale'].default_value = 3.0; no.inputs['Detail'].default_value = 6
    L(tc.outputs['Object'], no.inputs['Vector'])
    vo = N('ShaderNodeTexVoronoi'); vo.feature = 'DISTANCE_TO_EDGE'; vo.inputs['Scale'].default_value = panel
    L(tc.outputs['Object'], vo.inputs['Vector'])
    edge = N('ShaderNodeMapRange'); edge.inputs['From Min'].default_value = 0.0; edge.inputs['From Max'].default_value = 0.03
    edge.inputs['To Min'].default_value = 0.45; edge.inputs['To Max'].default_value = 1.0
    L(vo.outputs['Distance'], edge.inputs['Value'])
    var = N('ShaderNodeMapRange'); var.inputs['To Min'].default_value = 0.7; var.inputs['To Max'].default_value = 1.35
    L(no.outputs['Fac'], var.inputs['Value'])
    mul = N('ShaderNodeMath'); mul.operation = 'MULTIPLY'; L(edge.outputs[0], mul.inputs[0]); L(var.outputs[0], mul.inputs[1])
    rgb = N('ShaderNodeMix'); rgb.data_type = 'RGBA'; rgb.blend_type = 'MULTIPLY'; rgb.inputs['Factor'].default_value = 1
    rgb.inputs['A'].default_value = (*col, 1); L(mul.outputs[0], rgb.inputs['B'])
    L(rgb.outputs['Result'], b.inputs['Base Color'])
    bump = N('ShaderNodeBump'); bump.inputs['Strength'].default_value = .35; L(edge.outputs[0], bump.inputs['Height'])
    L(bump.outputs[0], b.inputs['Normal'])
    L(b.outputs[0], out.inputs[0]); return m


def rock(name):
    m, nt, out = mat(name); N = nt.nodes.new; L = nt.links.new
    b = N('ShaderNodeBsdfPrincipled'); b.inputs['Roughness'].default_value = .85
    tc = N('ShaderNodeTexCoord')
    no = N('ShaderNodeTexNoise'); no.inputs['Scale'].default_value = 4.0; no.inputs['Detail'].default_value = 10; no.inputs['Roughness'].default_value = .65
    L(tc.outputs['Object'], no.inputs['Vector'])
    cr = N('ShaderNodeValToRGB'); cr.color_ramp.elements[0].color = (0.012, 0.013, 0.018, 1); cr.color_ramp.elements[1].color = (0.06, 0.06, 0.075, 1)
    L(no.outputs['Fac'], cr.inputs[0]); L(cr.outputs[0], b.inputs['Base Color'])
    bump = N('ShaderNodeBump'); bump.inputs['Strength'].default_value = .8; L(no.outputs['Fac'], bump.inputs['Height']); L(bump.outputs[0], b.inputs['Normal'])
    L(b.outputs[0], out.inputs[0]); return m


def emit(name, col, strength, alpha=1.0):
    m, nt, out = mat(name); N = nt.nodes.new; L = nt.links.new
    e = N('ShaderNodeEmission'); e.inputs[0].default_value = (*col, 1); e.inputs[1].default_value = strength
    if alpha < 1:
        t = N('ShaderNodeBsdfTransparent'); mx = N('ShaderNodeMixShader'); mx.inputs[0].default_value = alpha
        L(t.outputs[0], mx.inputs[1]); L(e.outputs[0], mx.inputs[2]); L(mx.outputs[0], out.inputs[0])
        m.blend_method = 'BLEND'
    else:
        L(e.outputs[0], out.inputs[0])
    return m


def crystal_glass(name, col):
    """Lit from inside: glassy shell over a bright core, brighter toward the tip."""
    m, nt, out = mat(name); N = nt.nodes.new; L = nt.links.new
    g = N('ShaderNodeBsdfPrincipled'); g.inputs['Base Color'].default_value = (*col, 1)
    g.inputs['Roughness'].default_value = .12; g.inputs['Transmission Weight'].default_value = .6; g.inputs['IOR'].default_value = 1.6
    g.inputs['Emission Color'].default_value = (*col, 1)
    tc = N('ShaderNodeTexCoord'); sep = N('ShaderNodeSeparateXYZ'); L(tc.outputs['Generated'], sep.inputs[0])
    st = N('ShaderNodeMapRange'); st.inputs['To Min'].default_value = 0.35; st.inputs['To Max'].default_value = 2.2
    L(sep.outputs['Z'], st.inputs['Value']); L(st.outputs[0], g.inputs['Emission Strength'])
    L(g.outputs[0], out.inputs[0]); return m


# ---------------- scene ----------------
def hex_image(px=512):
    """A seamless hex line grid, made here (numpy) so the script needs no texture files."""
    import numpy as np
    r = px / 6.0; hw = math.sqrt(3) * r; apothem = hw / 2
    ys, xs = np.mgrid[0:px, 0:px].astype(np.float64)
    best = np.full((px, px), 1e9)
    # Two offset rectangular lattices make the hex centres; the nearest centre's hex metric gives the edge.
    for ox, oy in ((0.0, 0.0), (hw / 2, 1.5 * r)):
        cx = np.round((xs - ox) / hw) * hw + ox; cy = np.round((ys - oy) / (3 * r)) * 3 * r + oy
        dx = np.abs(xs - cx); dy = np.abs(ys - cy)
        metric = np.maximum(dx, 0.5 * dx + 0.8660254 * dy)
        best = np.minimum(best, metric)
    line = (apothem - best) < 2.5
    im = bpy.data.images.new('hexgrid', px, px, alpha=False)
    rgba = np.zeros((px, px, 4), dtype=np.float32); rgba[..., 0] = rgba[..., 1] = rgba[..., 2] = line; rgba[..., 3] = 1
    im.pixels.foreach_set(rgba.ravel()); im.pack(); return im


_HEX = None


def ground(sc, size=120):
    global _HEX
    if _HEX is None: _HEX = hex_image(256)
    bpy.ops.mesh.primitive_plane_add(size=size); g = bpy.context.object
    m, nt, out = mat('ground'); N = nt.nodes.new; L = nt.links.new
    p = N('ShaderNodeBsdfPrincipled'); p.inputs['Base Color'].default_value = (0.01, 0.012, 0.02, 1); p.inputs['Roughness'].default_value = .55; p.inputs['Metallic'].default_value = .3
    tc = N('ShaderNodeTexCoord'); mp = N('ShaderNodeMapping'); mp.inputs['Scale'].default_value = (size / 3.5, size / 3.5, 1)
    tx = N('ShaderNodeTexImage'); tx.image = _HEX
    L(tc.outputs['UV'], mp.inputs[0]); L(mp.outputs[0], tx.inputs[0])
    e = N('ShaderNodeEmission'); e.inputs[0].default_value = (0.25, 0.3, 0.55, 1)
    gr = N('ShaderNodeTexGradient'); gr.gradient_type = 'SPHERICAL'
    mp2 = N('ShaderNodeMapping'); mp2.inputs['Scale'].default_value = (1 / 14, 1 / 14, 1)
    L(tc.outputs['Object'], mp2.inputs[0]); L(mp2.outputs[0], gr.inputs[0])
    mul = N('ShaderNodeMath'); mul.operation = 'MULTIPLY'; L(tx.outputs['Color'], mul.inputs[0]); L(gr.outputs['Fac'], mul.inputs[1])
    mul2 = N('ShaderNodeMath'); mul2.operation = 'MULTIPLY'; mul2.inputs[1].default_value = .22; L(mul.outputs[0], mul2.inputs[0]); L(mul2.outputs[0], e.inputs[1])
    ad = N('ShaderNodeAddShader'); L(p.outputs[0], ad.inputs[0]); L(e.outputs[0], ad.inputs[1]); L(ad.outputs[0], out.inputs[0])
    g.data.materials.append(m)


def world(sc):
    w = bpy.data.worlds.new('w'); sc.world = w; w.use_nodes = True
    bg = w.node_tree.nodes['Background']; bg.inputs[1].default_value = 1
    tc = w.node_tree.nodes.new('ShaderNodeTexCoord'); sep = w.node_tree.nodes.new('ShaderNodeSeparateXYZ'); cr = w.node_tree.nodes.new('ShaderNodeValToRGB')
    w.node_tree.links.new(tc.outputs['Generated'], sep.inputs[0]); w.node_tree.links.new(sep.outputs['Z'], cr.inputs[0])
    cr.color_ramp.elements[0].position = .5; cr.color_ramp.elements[0].color = (0.006, 0.005, 0.012, 1)
    e = cr.color_ramp.elements.new(.53); e.color = (0.03, 0.02, 0.05, 1)
    cr.color_ramp.elements[1].position = .75; cr.color_ramp.elements[1].color = (0.002, 0.002, 0.008, 1)
    w.node_tree.links.new(cr.outputs[0], bg.inputs[0])


def light(sc, kind, loc, rot, energy, col, size=5):
    ld = bpy.data.lights.new('l', kind); ld.energy = energy; ld.color = col
    if kind == 'AREA': ld.size = size
    o = bpy.data.objects.new('l', ld); o.location = loc; o.rotation_euler = rot; sc.collection.objects.link(o)


def cam(sc, loc, target, lens=40):
    cd = bpy.data.cameras.new('c'); cd.lens = lens; o = bpy.data.objects.new('c', cd); sc.collection.objects.link(o)
    o.location = loc; d = Vector(target) - Vector(loc); o.rotation_euler = d.to_track_quat('-Z', 'Y').to_euler(); sc.camera = o


def render(sc, w, h, path):
    sc.render.engine = 'CYCLES'; sc.cycles.samples = SAMPLES
    try:
        prefs = bpy.context.preferences.addons['cycles'].preferences
        for kind in ('OPTIX', 'CUDA'):
            try:
                prefs.compute_device_type = kind; prefs.get_devices()
                if any(d.type == kind for d in prefs.devices):
                    for d in prefs.devices: d.use = d.type == kind
                    sc.cycles.device = 'GPU'; break
            except Exception:
                continue
    except Exception:
        sc.cycles.device = 'CPU'
    sc.cycles.use_denoising = True
    sc.cycles.max_bounces = 6; sc.cycles.transparent_max_bounces = 16; sc.cycles.transmission_bounces = 8
    sc.render.resolution_x = w; sc.render.resolution_y = h; sc.render.filepath = path
    sc.render.image_settings.file_format = 'JPEG'; sc.render.image_settings.quality = 92
    sc.view_settings.view_transform = 'AgX'; sc.view_settings.look = 'AgX - Medium High Contrast'
    bpy.ops.render.render(write_still=True)


def lights(sc, col, s=1.0):
    light(sc, 'AREA', (-6 * s, -6 * s, 8 * s), (math.radians(50), 0, math.radians(-45)), 2600 * s * s, (0.75, 0.82, 1.0), 5 * s)
    light(sc, 'AREA', (6 * s, 5 * s, 3 * s), (math.radians(70), 0, math.radians(135)), 500 * s * s, col, 5 * s)
    light(sc, 'AREA', (0, 8 * s, 8 * s), (math.radians(-40), 0, 0), 600 * s * s, (0.4, 0.5, 1.0), 6 * s)


# ---------------- shared bits ----------------
def link(o, coll=None):
    (coll or bpy.context.scene.collection).objects.link(o); return o


def prism(name, sides, r, h, at, rot=(0, 0, 0), tip=0.0, m=None, taper=1.0):
    """A sides-sided prism from the ground up, pointed when tip > 0 (crystal shards)."""
    me = bpy.data.meshes.new(name); bm = bmesh.new()
    low = [bm.verts.new((r * math.cos(i * 2 * math.pi / sides), r * math.sin(i * 2 * math.pi / sides), 0)) for i in range(sides)]
    up = [bm.verts.new((r * taper * math.cos(i * 2 * math.pi / sides), r * taper * math.sin(i * 2 * math.pi / sides), h)) for i in range(sides)]
    bm.faces.new(list(reversed(low)))
    for i in range(sides):
        j = (i + 1) % sides; bm.faces.new((low[i], low[j], up[j], up[i]))
    if tip > 0:
        t = bm.verts.new((0, 0, h + tip))
        for i in range(sides): bm.faces.new((up[i], up[(i + 1) % sides], t))
    else:
        bm.faces.new(up)
    bm.to_mesh(me); bm.free()
    o = bpy.data.objects.new(name, me); o.location = at; o.rotation_euler = rot
    if m: me.materials.append(m)
    return link(o)


def box(name, size, at, rot=(0, 0, 0), m=None, bevel=0.02):
    bpy.ops.mesh.primitive_cube_add(size=1, location=at, rotation=rot); o = bpy.context.object; o.name = name
    o.scale = size; bpy.ops.object.transform_apply(scale=True)
    if bevel > 0:
        mod = o.modifiers.new('b', 'BEVEL'); mod.width = bevel; mod.segments = 2
    if m: o.data.materials.append(m)
    return o


def cyl(name, r, h, at, sides=6, m=None, rot=(0, 0, 0)):
    bpy.ops.mesh.primitive_cylinder_add(vertices=sides, radius=r, depth=h, location=(at[0], at[1], at[2] + h / 2), rotation=rot)
    o = bpy.context.object; o.name = name
    if m: o.data.materials.append(m)
    return o


def boulder(name, r, at, squash=.45, seed=1, m=None):
    random.seed(seed)
    bpy.ops.mesh.primitive_ico_sphere_add(subdivisions=3, radius=r, location=at); o = bpy.context.object; o.name = name
    for v in o.data.vertices:
        n = v.co.normalized(); v.co += n * (random.uniform(-.18, .18) * r)
        v.co.z *= squash
    if m: o.data.materials.append(m)
    o.modifiers.new('s', 'SUBSURF').levels = 1
    return o


def ground_ring(name, r, col, dashes=24, width=.06):
    """The dashed glow ring where the node meets the ground (as on the TNs)."""
    m = emit(name + '_m', col, 6)
    for i in range(dashes):
        a = i * 2 * math.pi / dashes
        seg = box(name, (2 * math.pi * r / dashes * .6, width, .012), (r * math.cos(a), r * math.sin(a), .008), (0, 0, a + math.pi / 2), m, 0)
    # soft glow pool
    bpy.ops.mesh.primitive_circle_add(vertices=48, radius=r * .95, fill_type='NGON', location=(0, 0, .004)); pool = bpy.context.object
    pm, nt, out = mat(name + '_pool'); N = nt.nodes.new; L = nt.links.new
    e = N('ShaderNodeEmission'); e.inputs[0].default_value = (*col, 1)
    tc = N('ShaderNodeTexCoord'); gr = N('ShaderNodeTexGradient'); gr.gradient_type = 'SPHERICAL'
    mp = N('ShaderNodeMapping'); mp.inputs['Scale'].default_value = (1 / r, 1 / r, 1)
    L(tc.outputs['Object'], mp.inputs[0]); L(mp.outputs[0], gr.inputs[0])
    st = N('ShaderNodeMath'); st.operation = 'MULTIPLY'; st.inputs[1].default_value = .7; L(gr.outputs['Fac'], st.inputs[0]); L(st.outputs[0], e.inputs[1])
    t = N('ShaderNodeBsdfTransparent'); ad = N('ShaderNodeAddShader'); L(t.outputs[0], ad.inputs[0]); L(e.outputs[0], ad.inputs[1]); L(ad.outputs[0], out.inputs[0])
    pm.blend_method = 'BLEND'; pool.data.materials.append(pm)


def cracks(name, col, n=7, reach=1.4, seed=3):
    """Thin glowing cracks running out across the ground from the node."""
    random.seed(seed); m = emit(name + '_m', col, 9)
    for i in range(n):
        a = i * 2 * math.pi / n + random.uniform(-.3, .3); x = y = 0.0; length = random.uniform(.6, 1.0) * reach
        steps = 5
        for s in range(steps):
            a += random.uniform(-.5, .5); seg = length / steps
            nx, ny = x + seg * math.cos(a), y + seg * math.sin(a)
            box(name, (seg, .025 * (1 - s / steps) + .008, .01), ((x + nx) / 2, (y + ny) / 2, .006), (0, 0, a), m, 0)
            x, y = nx, ny


# ---------------- the four nodes (about 1.6 m across, pivot on the ground) ----------------
def node_crystal(off):
    col = RES['crystal']; glass = crystal_glass('crystal_glass', col); rk = rock('crystal_rock')
    root = bpy.data.objects.new('crystal', None); link(root)
    parts = [boulder('rock', .75, (0, 0, .05), .42, 4, rk), boulder('rock2', .38, (.55, -.35, 0), .5, 9, rk),
             boulder('rock3', .3, (-.6, .3, 0), .5, 12, rk)]
    random.seed(7)
    shards = [(.0, .0, .2, .22, 1.25, 0, 0), (.18, .1, .15, .14, .8, .35, .5), (-.2, .08, .18, .13, .75, -.4, 2.1),
              (.05, -.22, .15, .12, .65, .45, 4.0), (-.1, -.15, .12, .09, .45, -.35, 3.1), (.3, .25, .08, .08, .4, .6, 1.2),
              (-.32, -.05, .1, .07, .35, -.7, 3.6), (.55, -.3, .25, .07, .32, .5, 5.5), (-.6, .3, .2, .06, .28, -.5, 2.6)]
    for x, y, z, r, h, tilt, yaw in shards:
        parts.append(prism('shard', 6, r, h, (x, y, z), (tilt * math.cos(yaw), tilt * math.sin(yaw), random.uniform(0, 1)), tip=r * 1.8, m=glass, taper=.85))
    cracks('crystal_crack', col, 8, 1.3, 5)
    ground_ring('crystal_ring', 1.25, col)
    for p in parts:
        if p.parent is None: p.parent = root
    return root


def node_alloy(off):
    col = RES['alloy']; rk = rock('alloy_rock'); plate = hull('alloy_plate', (0.16, 0.12, 0.08), .35, .9, 9); seam = emit('alloy_seam', col, 6)
    parts = [boulder('vein', .8, (0, 0, -.05), .35, 21, rk), boulder('vein2', .45, (.55, .3, -.02), .4, 22, rk)]
    random.seed(11)
    # Plates sunk at angles in the vein, glowing molten seams between them.
    for i, (x, y, w, h, tilt, yaw) in enumerate([(-.25, .0, .55, .9, .35, .2), (.0, .05, .6, 1.05, .15, .35), (.25, -.05, .5, .8, -.3, .5),
                                                (.42, .2, .4, .55, -.45, .9), (-.45, -.18, .38, .5, .5, -.4)]):
        p = box('plate', (w, .09, h), (x, y, h * .32), (tilt, 0, yaw), plate, .015); parts.append(p)
        s = box('seam', (w * .85, .012, .035), (x, y + .05 * math.cos(yaw), h * .32 + h * .18), (tilt, 0, yaw), seam, 0); parts.append(s)
    # Two loose ingots in front.
    for x, y, yaw in ((.55, -.55, .4), (.3, -.75, -.3)):
        parts.append(box('ingot', (.32, .14, .1), (x, y, .05), (0, 0, yaw), plate, .02))
        parts.append(box('ingot_glow', (.26, .145, .012), (x, y, .102), (0, 0, yaw), seam, 0))
    cracks('alloy_crack', col, 6, 1.2, 8)
    ground_ring('alloy_ring', 1.3, col)


def node_data(off):
    col = RES['data']; body = hull('data_body', (0.05, 0.05, 0.065), .4, .7, 7); glow = emit('data_glow', col, 7); soft = emit('data_soft', col, 3, .6)
    # A low hex relay, knee high, set into a sunk collar; slit lights up its sides.
    cyl('collar', .72, .1, (0, 0, 0), 6, body)
    cyl('relay', .5, .55, (0, 0, .08), 6, body)
    cyl('cap', .36, .12, (0, 0, .63), 6, body)
    cyl('lens', .2, .05, (0, 0, .75), 6, glow)
    for i in range(6):
        a = i * math.pi / 3 + math.pi / 6; r = .44
        box('slit', (.05, .02, .36), (r * math.cos(a), r * math.sin(a), .36), (0, 0, a + math.pi / 2), glow, 0)
    # Cables into the ground.
    for i in range(3):
        a = i * 2 * math.pi / 3 + .3
        box('cable', (.55, .07, .05), (.75 * math.cos(a), .75 * math.sin(a), .03), (0, 0, a), body, .02)
        box('cable_glow', (.5, .075, .012), (.75 * math.cos(a), .75 * math.sin(a), .058), (0, 0, a), soft, 0)
    # The lattice and its cubes, just above the relay (they orbit in the game).
    random.seed(4)
    for i in range(8):
        a = i * 2 * math.pi / 8; r = .42 + (i % 2) * .12; z = .95 + (i % 3) * .1
        box('cube', (.1, .1, .1), (r * math.cos(a), r * math.sin(a), z), (random.uniform(0, 1), random.uniform(0, 1), a), glow, 0)
    bpy.ops.mesh.primitive_torus_add(major_radius=.45, minor_radius=.012, location=(0, 0, 1.0)); bpy.context.object.data.materials.append(soft)
    bpy.ops.mesh.primitive_torus_add(major_radius=.55, minor_radius=.01, location=(0, 0, 1.05), rotation=(.35, 0, 0)); bpy.context.object.data.materials.append(soft)
    ground_ring('data_ring', 1.2, col)


def node_credit(off):
    col = RES['credit']; body = hull('credit_body', (0.05, 0.06, 0.055), .35, .8, 8); glow = emit('credit_glow', col, 6); chip = hull('credit_chip', (0.1, 0.3, 0.16), .25, .9, 20)
    # A sealed cache plate set flush in the ground, the lid a little raised, a glowing seam round it.
    cyl('plate', .85, .06, (0, 0, -.01), 6, body)
    cyl('seam', .7, .065, (0, 0, -.01), 6, glow)
    cyl('lid', .66, .14, (0, 0, .0), 6, body)
    cyl('lock', .2, .06, (0, 0, .14), 6, glow)
    for i in range(6):
        a = i * math.pi / 3
        box('bolt', (.12, .06, .05), (.6 * math.cos(a), .6 * math.sin(a), .16), (0, 0, a), body, .01)
    # Stacks of hex chips round it, the top chip lit.
    random.seed(5)
    for k, (r, a, n) in enumerate(((1.0, .5, 5), (1.0, 1.7, 3), (1.05, 2.9, 6), (.98, 4.2, 4), (1.02, 5.3, 2))):
        x, y = r * math.cos(a), r * math.sin(a)
        for j in range(n):
            cyl('chip', .13, .045, (x + random.uniform(-.01, .01), y + random.uniform(-.01, .01), j * .05), 6,
                glow if j == n - 1 else chip, (0, 0, random.uniform(0, 1)))
    ground_ring('credit_ring', 1.35, col)


BUILD = {'crystal': node_crystal, 'alloy': node_alloy, 'data': node_data, 'credit': node_credit}


def shot_one(kind):
    sc = reset(); world(sc); ground(sc); lights(sc, RES[kind])
    BUILD[kind](0)
    cam(sc, (2.6, -3.4, 1.7), (0, 0, .45), 45)
    render(sc, 1080, 1080, os.path.join(OUT, f'node-{kind}.jpg'))


def shot_lineup():
    sc = reset(); world(sc); ground(sc, 160); lights(sc, (0.6, 0.6, 0.8), 1.8)
    for i, kind in enumerate(('crystal', 'alloy', 'data', 'credit')):
        before = set(bpy.data.objects)
        BUILD[kind](0)
        x = (i - 1.5) * 3.2
        for o in set(bpy.data.objects) - before:
            if o.parent is None and o.type in ('MESH', 'EMPTY'): o.location.x += x
    cam(sc, (0, -11.5, 4.2), (0, 0, .4), 32)
    render(sc, 1920, 900, os.path.join(OUT, 'node-lineup.jpg'))


if __name__ == '__main__':
    os.makedirs(OUT, exist_ok=True)
    if SHOT in ('all', 'lineup'): shot_lineup()
    for k in BUILD:
        if SHOT in ('all', k): shot_one(k)
