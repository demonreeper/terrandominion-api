import bpy, bmesh, math, random, sys
from mathutils import Matrix, Vector, Euler

TEX = '/tmp/claude-0/bl3/tex/'
SHOT = sys.argv[sys.argv.index('--') + 1] if '--' in sys.argv else 'hero'
RS = float(__import__('os').environ.get('RS','1'))
SAMPLES = int(sys.argv[sys.argv.index('--') + 2]) if '--' in sys.argv and len(sys.argv) > sys.argv.index('--') + 2 else 64
import os
_hx = os.environ.get('FACHEX', 'b57cff'); FCODE = os.environ.get('FCODE', 'veil')
FAC = tuple(((int(_hx[i:i+2], 16) / 255) ** 2.2) for i in (0, 2, 4))
_m = max(FAC); FAC = tuple(c / _m for c in FAC)

bpy.ops.wm.read_factory_settings(use_empty=True)
sc = bpy.context.scene

# ---------------- materials ----------------
def node_mat(name):
    m = bpy.data.materials.new(name); m.use_nodes = True
    nt = m.node_tree; nt.nodes.clear()
    out = nt.nodes.new('ShaderNodeOutputMaterial'); return m, nt, out

_IMG = {}
def timg(n, noncolor=True):
    if n not in _IMG:
        im = bpy.data.images.load(TEX + n + '.png')
        if noncolor: im.colorspace_settings.name = 'Non-Color'
        _IMG[n] = im
    return _IMG[n]

def metal(name, col, rough, panel=True, scale=.2, glow=0.0):
    m, nt, out = node_mat(name)
    N_ = nt.nodes.new; L_ = nt.links.new
    b = N_('ShaderNodeBsdfPrincipled'); b.inputs['Metallic'].default_value = .65
    tc = N_('ShaderNodeTexCoord'); mp = N_('ShaderNodeMapping'); mp.inputs['Scale'].default_value = (scale, scale, scale)
    L_(tc.outputs['Object'], mp.inputs[0])
    def tex(n):
        t = N_('ShaderNodeTexImage'); t.image = timg(n); t.projection = 'BOX'; t.projection_blend = .15; L_(mp.outputs[0], t.inputs[0]); return t
    th_, ta, tr, te = tex('sci_height'), tex('sci_albedo'), tex('sci_rough'), tex('sci_emit')
    # albedo: base colour x panel variation
    am = N_('ShaderNodeMapRange'); am.inputs['To Min'].default_value = .55; am.inputs['To Max'].default_value = 1.6
    L_(ta.outputs['Color'], am.inputs['Value'])
    colm = N_('ShaderNodeMixRGB'); colm.blend_type = 'MULTIPLY'; colm.inputs[0].default_value = 1; colm.inputs[1].default_value = (*col, 1)
    L_(am.outputs['Result'], colm.inputs[2])
    # grime: darker toward ground + noise
    nz = N_('ShaderNodeTexNoise'); nz.inputs['Scale'].default_value = 2.5; nz.inputs['Detail'].default_value = 8; L_(tc.outputs['Object'], nz.inputs['Vector'])
    sep = N_('ShaderNodeSeparateXYZ'); L_(tc.outputs['Object'], sep.inputs[0])
    gz = N_('ShaderNodeMapRange'); gz.inputs['From Min'].default_value = 0; gz.inputs['From Max'].default_value = 3; gz.inputs['To Min'].default_value = .45; gz.inputs['To Max'].default_value = 1
    L_(sep.outputs['Z'], gz.inputs['Value'])
    gr = N_('ShaderNodeMath'); gr.operation = 'MULTIPLY'; L_(gz.outputs['Result'], gr.inputs[0])
    nzr = N_('ShaderNodeMapRange'); nzr.inputs['To Min'].default_value = .7; nzr.inputs['To Max'].default_value = 1.15; L_(nz.outputs['Fac'], nzr.inputs['Value']); L_(nzr.outputs['Result'], gr.inputs[1])
    grm = N_('ShaderNodeMixRGB'); grm.blend_type = 'MULTIPLY'; grm.inputs[0].default_value = 1; L_(colm.outputs[0], grm.inputs[1])
    gcol = N_('ShaderNodeCombineColor'); L_(gr.outputs[0], gcol.inputs[0]); L_(gr.outputs[0], gcol.inputs[1]); L_(gr.outputs[0], gcol.inputs[2]); L_(gcol.outputs[0], grm.inputs[2])
    # edge wear via bevel normal
    bv = N_('ShaderNodeBevel'); bv.inputs['Radius'].default_value = .035
    geo = N_('ShaderNodeNewGeometry'); dot = N_('ShaderNodeVectorMath'); dot.operation = 'DOT_PRODUCT'
    L_(bv.outputs[0], dot.inputs[0]); L_(geo.outputs['Normal'], dot.inputs[1])
    ew = N_('ShaderNodeMapRange'); ew.inputs['From Min'].default_value = .985; ew.inputs['From Max'].default_value = .93; L_(dot.outputs['Value'], ew.inputs['Value'])
    nz2 = N_('ShaderNodeTexNoise'); nz2.inputs['Scale'].default_value = 18; L_(tc.outputs['Object'], nz2.inputs['Vector'])
    ewm = N_('ShaderNodeMath'); ewm.operation = 'MULTIPLY'; L_(ew.outputs['Result'], ewm.inputs[0])
    nz2r = N_('ShaderNodeMapRange'); nz2r.inputs['From Min'].default_value = .4; nz2r.inputs['From Max'].default_value = .65; L_(nz2.outputs['Fac'], nz2r.inputs['Value']); L_(nz2r.outputs['Result'], ewm.inputs[1])
    wear = N_('ShaderNodeMixRGB'); L_(ewm.outputs[0], wear.inputs[0]); L_(grm.outputs[0], wear.inputs[1]); wear.inputs[2].default_value = (.55, .57, .62, 1)
    L_(wear.outputs[0], b.inputs['Base Color'])
    # roughness
    rr = N_('ShaderNodeMapRange'); rr.inputs['To Min'].default_value = rough * .55; rr.inputs['To Max'].default_value = rough * 1.5; L_(tr.outputs['Color'], rr.inputs['Value'])
    rw = N_('ShaderNodeMix'); rw.data_type = 'FLOAT'; L_(ewm.outputs[0], rw.inputs['Factor']); L_(rr.outputs['Result'], rw.inputs['A']); rw.inputs['B'].default_value = .22
    L_(rw.outputs['Result'], b.inputs['Roughness'])
    # bump from panel height
    bp = N_('ShaderNodeBump'); bp.inputs['Strength'].default_value = .8; bp.inputs['Distance'].default_value = .02
    L_(th_.outputs['Color'], bp.inputs['Height']); L_(bv.outputs[0], bp.inputs['Normal']); L_(bp.outputs['Normal'], b.inputs['Normal'])
    if glow > 0:
        em = N_('ShaderNodeMath'); em.operation = 'MULTIPLY'; em.inputs[1].default_value = glow; L_(te.outputs['Color'], em.inputs[0])
        b.inputs['Emission Color'].default_value = (*FAC, 1); L_(em.outputs[0], b.inputs['Emission Strength'])
    L_(b.outputs[0], out.inputs[0]); return m

def emit(name, col, strength, alpha=1.0):
    m, nt, out = node_mat(name)
    e = nt.nodes.new('ShaderNodeEmission'); e.inputs[0].default_value = (*col, 1); e.inputs[1].default_value = strength
    if alpha < 1:
        tr = nt.nodes.new('ShaderNodeBsdfTransparent'); mx = nt.nodes.new('ShaderNodeMixShader'); mx.inputs[0].default_value = alpha
        nt.links.new(tr.outputs[0], mx.inputs[1]); nt.links.new(e.outputs[0], mx.inputs[2]); nt.links.new(mx.outputs[0], out.inputs[0])
        m.blend_method = 'BLEND'
    else:
        nt.links.new(e.outputs[0], out.inputs[0])
    return m

def img_mat(name, img, emit_strength=0.0, col=(1, 1, 1), alpha_mul=1.0):
    m, nt, out = node_mat(name)
    tx = nt.nodes.new('ShaderNodeTexImage'); tx.image = bpy.data.images.load(TEX + img)
    tr = nt.nodes.new('ShaderNodeBsdfTransparent'); mx = nt.nodes.new('ShaderNodeMixShader')
    if emit_strength > 0:
        e = nt.nodes.new('ShaderNodeEmission'); e.inputs[1].default_value = emit_strength
        mul = nt.nodes.new('ShaderNodeMixRGB'); mul.blend_type = 'MULTIPLY'; mul.inputs[0].default_value = 1; mul.inputs[2].default_value = (*col, 1)
        nt.links.new(tx.outputs['Color'], mul.inputs[1]); nt.links.new(mul.outputs[0], e.inputs[0]); sh = e
    else:
        sh = nt.nodes.new('ShaderNodeBsdfPrincipled'); sh.inputs['Roughness'].default_value = .85
        nt.links.new(tx.outputs['Color'], sh.inputs['Base Color'])
        e = nt.nodes.new('ShaderNodeEmission'); e.inputs[1].default_value = 1.5
        nt.links.new(tx.outputs['Color'], e.inputs[0]); ad = nt.nodes.new('ShaderNodeAddShader')
        nt.links.new(sh.outputs[0], ad.inputs[0]); nt.links.new(e.outputs[0], ad.inputs[1]); sh = ad
    am = nt.nodes.new('ShaderNodeMath'); am.operation = 'MULTIPLY'; am.inputs[1].default_value = alpha_mul
    nt.links.new(tx.outputs['Alpha'], am.inputs[0]); nt.links.new(am.outputs[0], mx.inputs[0])
    nt.links.new(tr.outputs[0], mx.inputs[1]); nt.links.new(sh.outputs[0], mx.inputs[2]); nt.links.new(mx.outputs[0], out.inputs[0])
    m.blend_method = 'BLEND'; return m

M = {
    'dark': metal('dark', (0.05, 0.055, 0.07), .4, glow=3.0),
    'mid': metal('mid', (0.11, 0.12, 0.15), .45),
    'light': metal('light', (0.28, 0.3, 0.34), .38, scale=.5),
    'emit': emit('emit', FAC, 3.5),
    'emit_soft': emit('emit_soft', FAC, 2, .5),
    'beam': emit('beam', FAC, 2.2, .3),
    'glass': emit('glass', FAC, 1.2, .1),
    'white': emit('white', (0.9, 0.95, 1), 14),
}

# ---------------- geometry builder ----------------
class B:
    def __init__(s): s.bm = {}
    def get(s, mat):
        if mat not in s.bm: s.bm[mat] = bmesh.new()
        return s.bm[mat]
    def box(s, mat, c, size, rz=0.0, rx=0.0, ry=0.0):
        mtx = Matrix.Translation(Vector(c)) @ Euler((rx, ry, rz)).to_matrix().to_4x4() @ Matrix.Diagonal((*size, 1))
        bmesh.ops.create_cube(s.get(mat), size=1.0, matrix=mtx)
    def cyl(s, mat, c, r1, r2, depth, seg=16, rz=0.0, rx=0.0, ry=0.0):
        mtx = Matrix.Translation(Vector(c)) @ Euler((rx, ry, rz)).to_matrix().to_4x4()
        bmesh.ops.create_cone(s.get(mat), cap_ends=True, segments=seg, radius1=r1, radius2=r2, depth=depth, matrix=mtx)
    def sphere(s, mat, c, r):
        bmesh.ops.create_uvsphere(s.get(mat), u_segments=24, v_segments=12, radius=r, matrix=Matrix.Translation(Vector(c)))
    def ring(s, mat, c, R, w, h=.03, seg=96):
        # flat ring from boxes around a circle
        for i in range(seg):
            a = i / seg * math.tau; L = math.tau * R / seg * 1.08
            s.box(mat, (c[0] + R * math.cos(a), c[1] + R * math.sin(a), c[2]), (w, L, h), rz=a)
    def build(s, name, bevel=True):
        objs = []
        for mat, bm in s.bm.items():
            me = bpy.data.meshes.new(name + '_' + mat); bm.to_mesh(me); bm.free()
            ob = bpy.data.objects.new(name + '_' + mat, me); sc.collection.objects.link(ob)
            ob.data.materials.append(M[mat])
            if bevel and mat in ('dark', 'mid', 'light'):
                bv = ob.modifiers.new('bv', 'BEVEL'); bv.width = .025; bv.segments = 2; bv.limit_method = 'ANGLE'
                bv.harden_normals = False
            for p in me.polygons: p.use_smooth = False
            objs.append(ob)
        s.bm = {}; return objs

def face_greebles(b, rnd, cx, cy, z0, w, d, lh, rz):
    # four faces of a box centred (cx,cy), size w x d, from z0 to z0+lh
    ca, sa = math.cos(rz), math.sin(rz)
    def P(x, y, z): return (cx + x * ca - y * sa, cy + x * sa + y * ca, z)
    for f in range(4):
        nx, ny = [(1, 0), (0, 1), (-1, 0), (0, -1)][f]
        half_n = (w if nx else d) / 2; half_t = (d if nx else w) / 2
        tx, ty = -ny, nx
        for i in range(rnd.randint(10, 18)):
            u = rnd.uniform(-half_t * .8, half_t * .8); v = rnd.uniform(.12, .88) * lh
            sw = rnd.uniform(.08, .32) * w; sh = rnd.uniform(.06, .25) * lh; dp = rnd.uniform(.04, .18)
            x = nx * (half_n + dp / 2) + tx * u; y = ny * (half_n + dp / 2) + ty * u
            b.box(rnd.choice(['mid', 'dark', 'mid', 'light']), P(x, y, z0 + v), (dp if nx else sw, sw if nx else dp, sh), rz=rz)
        # vents
        u0 = rnd.uniform(-half_t * .5, half_t * .5); v0 = rnd.uniform(.2, .6) * lh
        for k in range(5):
            x = nx * (half_n + .03) + tx * u0; y = ny * (half_n + .03) + ty * u0
            b.box('dark', P(x, y, z0 + v0 + k * .09), (.06 if nx else w * .2, w * .2 if nx else .06, .035), rz=rz)
        # emissive strips
        for sgn in (-1, 1):
            u = sgn * half_t * .55
            x = nx * (half_n + .015) + tx * u; y = ny * (half_n + .015) + ty * u
            b.box('emit', P(x, y, z0 + lh * .5), (.03 if nx else .05, .05 if nx else .03, lh * .7), rz=rz)
        # pipe
        u = rnd.uniform(-half_t * .9, half_t * .9)
        x = nx * (half_n + .07) + tx * u; y = ny * (half_n + .07) + ty * u
        b.cyl('light', P(x, y, z0 + lh / 2), .045, .045, lh * .95, seg=8)

def flag(name, pos, rz, w, h):
    bpy.ops.mesh.primitive_plane_add(size=1)
    ob = bpy.context.object; ob.name = name
    ob.scale = (w, h, 1); bpy.ops.object.transform_apply(scale=True)
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.subdivide(number_cuts=14); bpy.ops.object.mode_set(mode='OBJECT')
    for v in ob.data.vertices:
        u = v.co.x + w / 2
        v.co.z = math.sin(u * 2.4 + v.co.y * .6) * .12 * (u / w + .2)
        v.co.x = u
    ob.rotation_euler = (math.pi / 2, 0, rz); ob.location = pos
    ob.data.materials.append(M['banner'])
    return ob

M['banner'] = img_mat('banner', 'banner_%s.png' % FCODE)
M['holo'] = img_mat('holo', 'holo_%s.png' % FCODE, emit_strength=4, col=FAC, alpha_mul=.9)
M['hexdome'] = img_mat('hexdome', 'hexgrid.png', emit_strength=3, col=FAC)
# ---------------- world ----------------
w = bpy.data.worlds.new('w'); sc.world = w; w.use_nodes = True
bg = w.node_tree.nodes['Background']; bg.inputs[1].default_value = 1
_tc = w.node_tree.nodes.new('ShaderNodeTexCoord'); _sep = w.node_tree.nodes.new('ShaderNodeSeparateXYZ'); _cr = w.node_tree.nodes.new('ShaderNodeValToRGB')
w.node_tree.links.new(_tc.outputs['Generated'], _sep.inputs[0]); w.node_tree.links.new(_sep.outputs['Z'], _cr.inputs[0])
_cr.color_ramp.elements[0].position = .5; _cr.color_ramp.elements[0].color = (0.006, 0.005, 0.012, 1)
_e = _cr.color_ramp.elements.new(.53); _e.color = (0.035, 0.014, 0.05, 1)
_cr.color_ramp.elements[1].position = .75; _cr.color_ramp.elements[1].color = (0.002, 0.002, 0.008, 1)
w.node_tree.links.new(_cr.outputs[0], bg.inputs[0])

def ground(size=200):
    bpy.ops.mesh.primitive_plane_add(size=size); g = bpy.context.object
    m, nt, out = node_mat('ground')
    p = nt.nodes.new('ShaderNodeBsdfPrincipled'); p.inputs['Base Color'].default_value = (0.01, 0.012, 0.02, 1); p.inputs['Roughness'].default_value = .55; p.inputs['Metallic'].default_value = .3
    tc = nt.nodes.new('ShaderNodeTexCoord'); mp = nt.nodes.new('ShaderNodeMapping'); mp.inputs['Scale'].default_value = (size / 6, size / 6, 1)
    tx = nt.nodes.new('ShaderNodeTexImage'); tx.image = bpy.data.images.load(TEX + 'hexgrid.png')
    e = nt.nodes.new('ShaderNodeEmission'); e.inputs[0].default_value = (0.25, 0.3, 0.55, 1)
    gr = nt.nodes.new('ShaderNodeTexGradient'); gr.gradient_type = 'SPHERICAL'
    mp2 = nt.nodes.new('ShaderNodeMapping'); mp2.inputs['Scale'].default_value = (1 / 30, 1 / 30, 1)
    nt.links.new(tc.outputs['Object'], mp2.inputs[0]); nt.links.new(mp2.outputs[0], gr.inputs[0])
    mul = nt.nodes.new('ShaderNodeMath'); mul.operation = 'MULTIPLY'
    nt.links.new(tx.outputs['Color'], mul.inputs[0]); nt.links.new(gr.outputs['Fac'], mul.inputs[1])
    mul2 = nt.nodes.new('ShaderNodeMath'); mul2.operation = 'MULTIPLY'; mul2.inputs[1].default_value = .5
    nt.links.new(mul.outputs[0], mul2.inputs[0]); nt.links.new(mul2.outputs[0], e.inputs[1])
    nt.links.new(tc.outputs['UV'], mp.inputs[0]); nt.links.new(mp.outputs[0], tx.inputs[0])
    ad = nt.nodes.new('ShaderNodeAddShader'); nt.links.new(p.outputs[0], ad.inputs[0]); nt.links.new(e.outputs[0], ad.inputs[1]); nt.links.new(ad.outputs[0], out.inputs[0])
    g.data.materials.append(m)

def light(kind, loc, rot, energy, col, size=5):
    ld = bpy.data.lights.new('l', kind); ld.energy = energy; ld.color = col
    if kind == 'AREA': ld.size = size
    o = bpy.data.objects.new('l', ld); o.location = loc; o.rotation_euler = rot; sc.collection.objects.link(o)

def cam(loc, target, lens=35):
    cd = bpy.data.cameras.new('c'); cd.lens = lens; o = bpy.data.objects.new('c', cd); sc.collection.objects.link(o)
    o.location = loc; d = Vector(target) - Vector(loc); o.rotation_euler = d.to_track_quat('-Z', 'Y').to_euler(); sc.camera = o

def render_setup(w_, h_, out):
    sc.render.engine = 'CYCLES'; sc.cycles.device = 'CPU'; sc.cycles.samples = SAMPLES
    sc.cycles.use_denoising = True; sc.cycles.denoiser = 'OPENIMAGEDENOISE'
    sc.cycles.max_bounces = 4; sc.cycles.transparent_max_bounces = 16
    sc.render.resolution_x = w_; sc.render.resolution_y = h_; sc.render.filepath = out
    sc.view_settings.view_transform = 'Filmic'; sc.view_settings.look = 'Medium High Contrast'
    sc.use_nodes = True; nt = sc.node_tree; nt.nodes.clear()
    rl = nt.nodes.new('CompositorNodeRLayers'); gl = nt.nodes.new('CompositorNodeGlare'); gl.glare_type = 'FOG_GLOW'; gl.threshold = 1.2; gl.size = 7; gl.mix = 0.0
    co = nt.nodes.new('CompositorNodeComposite'); nt.links.new(rl.outputs[0], gl.inputs[0]); nt.links.new(gl.outputs[0], co.inputs[0])

def std_lights(scale=1.0):
    light('AREA', (-20 * scale, -18 * scale, 25 * scale), (math.radians(50), 0, math.radians(-45)), 26000 * scale ** 2, (0.75, 0.82, 1.0), 12 * scale)
    light('AREA', (22 * scale, 20 * scale, 10 * scale), (math.radians(70), 0, math.radians(135)), 3500 * scale ** 2, FAC, 15 * scale)
    light('AREA', (0, 30 * scale, 30 * scale), (math.radians(-40), 0, 0), 5000 * scale ** 2, (0.4, 0.5, 1.0), 20 * scale)


# ================= growing base (v4 mock-up) =================
# Real metres. A base grows three ways:
#   tier up   -> central tower +1 storey and bigger crown, a new outer terrace, more pylon towers, bastions from T4
#   new module -> a small tower stands up on its pad
#   module Mk  -> that small tower gains a storey (Mk I = 1 ... Mk IV = 4) and a bigger cap
PLAT = {1: 13, 2: 15.5, 3: 20, 4: 25, 5: 30}      # outer terrace radius per tier
STOREYS = {1: 4, 2: 5, 3: 6, 4: 7, 5: 8}          # central tower storeys
STH = 4.0; W0 = 4.6; TAPER = .9
RING_R = {1: 10.5, 2: 16.5, 3: 22.5}
PYLON_ORDER = [0, 3, 1, 4, 2, 5]                  # new pylons join, old ones stay and grow
PYLON_COUNT = {1: 0, 2: 2, 3: 3, 4: 4, 5: 6}
PYLON_R = 7.4
SLOTS = {1: (4, 0, 0), 2: (8, 0, 0), 3: (8, 4, 0), 4: (8, 4, 6), 5: (8, 4, 12)}
KINDS1 = ['lattice', 'shield', 'vault', 'uplink', 'collector', 'sensor', 'lattice', 'collector']
KINDS2 = ['hangar', 'fabricator', 'lattice', 'shield']
KINDS3 = ['collector', 'sensor', 'hangar', 'relay', 'uplink', 'fabricator', 'lattice', 'shield', 'collector', 'sensor', 'vault', 'lattice']

M['person'] = metal('person', (0.5, 0.52, 0.56), .6, scale=1)

def hexplate(b, mat, c, R, h, ztop):
    b.cyl(mat, (c[0], c[1], ztop - h / 2), R, R, h, seg=6, rz=math.pi / 6)

def hex_edges(b, mat, c, R, z, w=.07, hgt=.04, inset=0.0):
    ap = R * math.cos(math.pi / 6) - inset
    for k in range(6):
        a = k * math.pi / 3
        b.box(mat, (c[0] + ap * math.cos(a), c[1] + ap * math.sin(a), z), (w, R * .97 - inset, hgt), rz=a)

def deck_z(tier, r):
    for t in range(1, tier + 1):
        if r <= PLAT[t] * math.cos(math.pi / 6) - .6:
            return .35 * (tier - t + 1)
    return 0.0

def storey(b, rnd, ox, oy, z, w, lh, rz, greeble=True):
    d = w * .86
    b.box('dark', (ox, oy, z + lh * .47), (w, d, lh * .94), rz=rz)
    if greeble:
        face_greebles(b, rnd, ox, oy, z, w, d, lh, rz)
    ca, sa = math.cos(rz), math.sin(rz)
    for cx in (-1, 1):
        for cy in (-1, 1):
            x, y = cx * w * .5, cy * d * .5
            b.box('mid', (ox + x * ca - y * sa, oy + x * sa + y * ca, z + lh * .5), (w * .15, w * .15, lh * 1.02), rz=rz)
    b.box('light', (ox, oy, z + lh * .97), (w * 1.1, d * 1.1, .09), rz=rz)
    b.box('emit', (ox, oy, z + lh * .9), (w * 1.02, d * 1.02, .035), rz=rz)

def central_tower(b, tier, ox, oy, z0):
    # plinth widens each tier
    pr = W0 * .78 + tier * .35
    b.cyl('mid', (ox, oy, z0 + tier * .45), pr, pr * .88, tier * .9, seg=6, rz=math.pi / 6)
    hex_edges(b, 'emit', (ox, oy), pr * .9, z0 + tier * .9 + .02, w=.05)
    z = z0 + tier * .9; w = W0
    for k in range(STOREYS[tier]):
        rnd = random.Random(1000 + k)            # same storey look every tier: the tower visibly grows
        rz = (k % 2) * math.pi / 12
        storey(b, rnd, ox, oy, z, w, STH, rz)
        # side masses on the lower half for a jagged silhouette
        if k < STOREYS[tier] - 2:
            for sd in (-1, 1):
                sw = w * .36
                b.box('dark', (ox + sd * w * .62, oy + w * .08, z + STH * .42), (sw, sw * 1.1, STH * .8), rz=rz)
                b.box('emit', (ox + sd * (w * .62 + sw * .5 + .01), oy + w * .08, z + STH * .42), (.02, sw * .7, .04), rz=rz)
        z += STH; w *= TAPER
    # crown grows with tier
    b.box('mid', (ox, oy, z + .35), (w * .9, w * .8, .7))
    b.cyl('dark', (ox, oy, z + .7 + tier * .35), w * .42, w * .22, tier * .7, seg=6)
    cz = z + .7 + tier * .7
    for i in range(2 + tier):
        a = i * 2.4; r = w * .28 * (i > 0)
        hgt = 1.6 + tier * .9 - (i > 0) * 1.2 * rnd.random()
        b.cyl('dark', (ox + r * math.cos(a), oy + r * math.sin(a), cz + hgt / 2), .07 + .04 * (i == 0), .02, hgt, seg=8)
        b.sphere('emit', (ox + r * math.cos(a), oy + r * math.sin(a), cz + hgt), .07 + .03 * (i == 0))
    b.sphere('emit', (ox, oy, cz + .45), .3 + tier * .07)
    return cz + 1.6 + tier * .9

def pylon(b, rnd, ox, oy, z0, n, w, face):
    lh = 3.0; z = z0
    b.cyl('mid', (ox, oy, z0 + .3), w * .95, w * .8, .6, seg=6, rz=face)
    z += .6; r = w * .62
    for k in range(n):
        b.cyl('dark', (ox, oy, z + lh * .47), r, r * .96, lh * .94, seg=6, rz=face)
        for f in range(6):
            a = face + f * math.pi / 3 + math.pi / 6; ap = r * .87
            b.box('emit', (ox + math.cos(a) * (ap + .02), oy + math.sin(a) * (ap + .02), z + lh * .5), (.03, .06, lh * .55), rz=a)
            if (f + k) % 2 == 0:
                b.box('mid', (ox + math.cos(a) * (ap + .1), oy + math.sin(a) * (ap + .1), z + lh * rnd.uniform(.25, .75)), (.2, r * .5, lh * .3), rz=a)
        b.cyl('light', (ox, oy, z + lh * .97), r * 1.12, r * 1.12, .1, seg=6, rz=face)
        b.cyl('emit', (ox, oy, z + lh * .9), r * 1.02, r * 1.02, .04, seg=6, rz=face)
        z += lh; r *= .93
    b.cyl('mid', (ox, oy, z + .3), r * .9, r * .6, .6, seg=6, rz=face)
    b.cyl('dark', (ox, oy, z + .6 + 1.4), r * .45, .04, 2.8, seg=6, rz=face)
    b.sphere('emit', (ox, oy, z + .9), .26)
    return z

def bridge(b, x1, y1, x2, y2, z):
    dx, dy = x2 - x1, y2 - y1; L = math.hypot(dx, dy); a = math.atan2(dy, dx)
    b.box('mid', ((x1 + x2) / 2, (y1 + y2) / 2, z), (L, .9, .55), rz=a)
    b.box('emit', ((x1 + x2) / 2, (y1 + y2) / 2, z + .3), (L, .06, .04), rz=a)
    b.box('emit', ((x1 + x2) / 2, (y1 + y2) / 2, z - .3), (L, .06, .04), rz=a)


def fade_beam_mat(name, strength, alpha, z0=0.02, z1=0.9):
    m, nt, out = node_mat(name)
    N_ = nt.nodes.new; L_ = nt.links.new
    tc = N_('ShaderNodeTexCoord'); sp = N_('ShaderNodeSeparateXYZ'); L_(tc.outputs['Generated'], sp.inputs[0])
    mr = N_('ShaderNodeMapRange'); mr.interpolation_type = 'SMOOTHSTEP'
    mr.inputs['From Min'].default_value = z0; mr.inputs['From Max'].default_value = z1
    mr.inputs['To Min'].default_value = 1.0; mr.inputs['To Max'].default_value = 0.0
    L_(sp.outputs['Z'], mr.inputs['Value'])
    e = N_('ShaderNodeEmission'); e.inputs[0].default_value = (*FAC, 1)
    # travelling pulses up the beam: 0.65 + 0.7 * max(0, sin(z*k - phase))^4
    ph = N_('ShaderNodeValue'); ph.name = 'phase'; ph.outputs[0].default_value = 0.0
    zk = N_('ShaderNodeMath'); zk.operation = 'MULTIPLY'; zk.inputs[1].default_value = 22.0; L_(sp.outputs['Z'], zk.inputs[0])
    sb = N_('ShaderNodeMath'); sb.operation = 'SUBTRACT'; L_(zk.outputs[0], sb.inputs[0]); L_(ph.outputs[0], sb.inputs[1])
    sn = N_('ShaderNodeMath'); sn.operation = 'SINE'; L_(sb.outputs[0], sn.inputs[0])
    mxp = N_('ShaderNodeMath'); mxp.operation = 'MAXIMUM'; mxp.inputs[1].default_value = 0.0; L_(sn.outputs[0], mxp.inputs[0])
    pw = N_('ShaderNodeMath'); pw.operation = 'POWER'; pw.inputs[1].default_value = 4.0; L_(mxp.outputs[0], pw.inputs[0])
    pl = N_('ShaderNodeMath'); pl.operation = 'MULTIPLY_ADD'; pl.inputs[1].default_value = .7; pl.inputs[2].default_value = .65; L_(pw.outputs[0], pl.inputs[0])
    m1 = N_('ShaderNodeMath'); m1.operation = 'MULTIPLY'; L_(mr.outputs['Result'], m1.inputs[0]); L_(pl.outputs[0], m1.inputs[1])
    st = N_('ShaderNodeMath'); st.operation = 'MULTIPLY'; st.inputs[1].default_value = strength
    L_(m1.outputs[0], st.inputs[0]); L_(st.outputs[0], e.inputs[1])
    am = N_('ShaderNodeMath'); am.operation = 'MULTIPLY'; am.inputs[1].default_value = alpha
    L_(mr.outputs['Result'], am.inputs[0])
    tr = N_('ShaderNodeBsdfTransparent'); mx = N_('ShaderNodeMixShader')
    L_(am.outputs[0], mx.inputs[0]); L_(tr.outputs[0], mx.inputs[1]); L_(e.outputs[0], mx.inputs[2]); L_(mx.outputs[0], out.inputs[0])
    m.blend_method = 'BLEND'; return m

M['beamfade'] = fade_beam_mat('beamfade', 2.4, .35)
M['beamcore'] = fade_beam_mat('beamcore', 5.0, 1.0)

_BEAMS = [0]
BEAM_OBJS = []
def sky_beam(b, x, y, z, H=30.0):
    """Faction-colour light beam straight up, fading out toward the top (its own object, so the fade runs
    bottom to top of each beam). No rings round it."""
    b.sphere('emit', (x, y, z), .2)
    bb = B()
    bb.cyl('beamfade', (x, y, z + H / 2), .4, .22, H, seg=12)
    bb.cyl('beamcore', (x, y, z + H / 2), .07, .05, H, seg=8)
    _BEAMS[0] += 1
    objs = bb.build('beam%d' % _BEAMS[0], bevel=False)
    for ob in objs:
        ob.data.transform(Matrix.Translation(Vector((-x, -y, -z)))); ob.location = (x, y, z)
    BEAM_OBJS.append((objs, x, y))

def bastion(b, rnd, ox, oy, z0, n, face):
    top = pylon(b, rnd, ox, oy, z0, n, 2.8, face)
    # turret on top, facing out
    b.cyl('mid', (ox, oy, top + .25), .9, 1.0, .5, seg=8)
    b.box('dark', (ox, oy, top + .8), (1.6, 1.3, .7), rz=face)
    for sd in (-.28, .28):
        bx = ox + math.cos(face) * 1.4 - math.sin(face) * sd; by = oy + math.sin(face) * 1.4 + math.cos(face) * sd
        b.cyl('light', (bx, by, top + .85), .09, .1, 1.9, seg=8, rz=face, ry=math.pi / 2)
    b.box('emit', (ox + math.cos(face) * .81, oy + math.sin(face) * .81, top + .95), (.05, .8, .08), rz=face)
    # sky beam (owner 2026-11-13): emitter on the turret, a plain vertical beam, no rings round it
    b.cyl('mid', (ox, oy, top + 1.35), .35, .2, .4, seg=6)
    b.sphere('emit', (ox, oy, top + 1.7), .22)
    sky_beam(b, ox, oy, top + 1.7)

def module_tower(b, kind, px, py, z0, mk, face=0.0):
    rnd = random.Random(hash(kind) % 1000 + mk)
    hexplate(b, 'light', (px, py), 1.9, .25, z0 + .25)
    hex_edges(b, 'emit', (px, py), 1.85, z0 + .26, w=.05, hgt=.02)
    w = 1.9 if kind not in ('vault', 'hangar') else 2.3
    lh = 1.7; z = z0 + .25
    for k in range(mk):
        rz = face + (k % 2) * .15
        d = w * .86
        b.box('dark', (px, py, z + lh * .47), (w, d, lh * .94), rz=rz)
        ca, sa = math.cos(rz), math.sin(rz)
        for f in range(4):
            a = rz + f * math.pi / 2; hw = (w if f % 2 == 0 else d) / 2
            b.box('emit', (px + math.cos(a) * (hw + .01), py + math.sin(a) * (hw + .01), z + lh * .5), (.03, .05, lh * .6), rz=a)
            b.box('mid', (px + math.cos(a + .5) * (hw * .9), py + math.sin(a + .5) * (hw * .9), z + lh * rnd.uniform(.3, .7)), (.25, .4, .3), rz=a)
        b.box('light', (px, py, z + lh * .97), (w * 1.08, d * 1.08, .07), rz=rz)
        b.box('emit', (px, py, z + lh * .9), (w * 1.01, d * 1.01, .03), rz=rz)
        z += lh; w *= .92
    t = z
    s = .8 + mk * .15   # cap grows with Mk too
    if kind == 'lattice':
        b.cyl('mid', (px, py, t + .2), w * .4, w * .3, .4, seg=6)
        b.cyl('emit', (px, py, t + .4 + 1.2 * s), .5 * s, 0, 2.4 * s, seg=6)
        b.cyl('emit', (px, py, t + .4 + .25 * s), .5 * s, .5 * s, .02, seg=6)
        for i in range(3):
            a = i * math.tau / 3 + .3
            b.cyl('emit', (px + math.cos(a) * .9 * s, py + math.sin(a) * .9 * s, t + 1.2 * s), .14 * s, 0, .9 * s, seg=4, rx=.4 * math.cos(a), ry=.4 * math.sin(a))
    elif kind == 'shield':
        b.cyl('mid', (px, py, t + .15), w * .45, w * .4, .3, seg=8)
        bpy_dome = (px, py, t + .3)
        b.sphere('glass', bpy_dome, 1.0 * s)
        b.sphere('emit', (px, py, t + .3 + .55 * s), .28 * s)
        for i in range(3):
            a = i * math.tau / 3
            b.box('mid', (px + math.cos(a) * .7 * s, py + math.sin(a) * .7 * s, t + .5), (.2, .2, .9 * s), rz=a, rx=-.35 * math.sin(a), ry=.35 * math.cos(a))
    elif kind == 'vault':
        b.box('mid', (px, py, t + .45 * s), (w * 1.05, w * 1.05, .9 * s))
        b.box('dark', (px, py, t + .9 * s + .2), (w * .7, w * .7, .4))
        b.box('emit', (px, py, t + .45 * s), (w * 1.07, .06, .06))
        b.box('emit', (px, py, t + .45 * s), (.06, w * 1.07, .06))
        b.box('light', (px + w * .53, py, t + .45 * s), (.05, w * .4, .5 * s))
    elif kind == 'uplink':
        mh = 2.2 * s
        b.cyl('dark', (px, py, t + mh / 2), .1, .06, mh, seg=8)
        b.cyl('light', (px + .2, py, t + mh * .65), .95 * s, .15, .35, seg=16, ry=.7)
        b.cyl('emit', (px + .2, py, t + mh * .65 + .05), .7 * s, .7 * s, .02, seg=16, ry=.7)
        b.sphere('emit', (px, py, t + mh), .12)
    elif kind == 'collector':
        b.sphere('emit', (px, py, t + 1.0 * s), .38 * s)
        for i in range(3):
            a = i * math.tau / 3
            b.box('mid', (px + math.cos(a) * .55 * s, py + math.sin(a) * .55 * s, t + .9 * s), (.22, .3, 1.8 * s), rz=a, rx=.45 * math.sin(a), ry=-.45 * math.cos(a))
            b.box('emit', (px + math.cos(a) * .45 * s, py + math.sin(a) * .45 * s, t + 1.2 * s), (.05, .08, .8 * s), rz=a, rx=.45 * math.sin(a), ry=-.45 * math.cos(a))
    elif kind == 'sensor':
        b.cyl('dark', (px, py, t + .5), .14, .14, 1.0, seg=8)
        b.box('mid', (px, py, t + 1.1 + .3 * s), (1.9 * s, .16, 1.0 * s), rz=face + .6, rx=.55)
        b.box('emit', (px, py, t + 1.1 + .3 * s), (1.7 * s, .18, .07), rz=face + .6, rx=.55)
    elif kind == 'hangar':
        b.cyl('mid', (px, py, t + .15), 1.5 * s, 1.4 * s, .3, seg=6, rz=math.pi / 6)
        hex_edges(b, 'emit', (px, py), 1.45 * s, t + .31, w=.05, hgt=.02)
        b.box('dark', (px, py, t + 1.1), (.55, .55, .22))
        for i in range(3):
            a = i * math.tau / 3
            b.box('mid', (px + math.cos(a) * .4, py + math.sin(a) * .4, t + 1.1), (.5, .08, .05), rz=a)
            b.cyl('emit', (px + math.cos(a) * .65, py + math.sin(a) * .65, t + 1.12), .17, .17, .02, seg=12)
    elif kind == 'fabricator':
        for sd in (-1, 1):
            b.cyl('dark', (px + sd * .45, py, t + .7 * s), .28, .22, 1.4 * s, seg=8)
            b.cyl('emit', (px + sd * .45, py, t + 1.4 * s + .03), .2, .2, .06, seg=8)
        b.box('mid', (px, py + .5, t + .35), (1.2, .5, .7))
    elif kind == 'relay':
        nh = 4.5 * s
        b.cyl('dark', (px, py, t + nh / 2), .22, .03, nh, seg=6)
        oz = t + nh + .9
        b.cyl('emit', (px, py, oz + .45), .5, 0, .9, seg=6)
        b.cyl('emit', (px, py, oz - .45), 0, .5, .9, seg=6)
    return t

def person(x, y, rz=0):
    b = B()
    b.cyl('person', (x, y, .45), .13, .16, .9, seg=10)
    b.cyl('person', (x, y, 1.22), .2, .16, .64, seg=10)
    b.sphere('person', (x, y, 1.66), .12)
    b.build('person', bevel=False)


WALL_R = [14.0, 20.0, 27.0, 33.0]          # hex wall rings (corner radius); a new ring is built at T1, T3, T4, T5
RING_BUILT = [1, 3, 4, 5]
NRINGS = {1: 1, 2: 1, 3: 2, 4: 3, 5: 4}
PLAT = {t: WALL_R[NRINGS[t] - 1] + 1.6 for t in range(1, 6)}
PYLON_R = 6.2
GATE_EDGE = 4                               # edge facing the street (camera side) has the gate

def corner(R, k, ox, oy):
    a = math.pi / 6 + k * math.pi / 3
    return ox + R * math.cos(a), oy + R * math.sin(a)

def wall_segment(b, x1, y1, x2, y2, z, hw, hf):
    dx, dy = x2 - x1, y2 - y1; L = math.hypot(dx, dy); a = math.atan2(dy, dx)
    if L < .5: return
    cx, cy = (x1 + x2) / 2, (y1 + y2) / 2
    b.box('dark', (cx, cy, z + hw / 2), (L, 1.1, hw), rz=a)
    b.box('light', (cx, cy, z + hw + .06), (L, 1.35, .12), rz=a)
    nx, ny = -math.sin(a), math.cos(a)
    for sd in (-1, 1):
        b.box('emit', (cx + nx * sd * .56, cy + ny * sd * .56, z + hw * .55), (L * .96, .03, .05), rz=a)
        # armour ribs
        n = max(1, int(L / 2.2))
        for j in range(n):
            u = (j + .5) / n - .5
            b.box('mid', (cx + dx * u + nx * sd * .6, cy + dy * u + ny * sd * .6, z + hw * .45), (.35, .18, hw * .8), rz=a)
    # energy field on top between emitter posts
    if hf > 0:
        b.box('glass', (cx, cy, z + hw + .12 + hf / 2), (L, .06, hf), rz=a)
        b.box('emit_soft', (cx, cy, z + hw + .12 + hf), (L, .05, .05), rz=a)
        n = max(1, int(L / 3.5))
        for j in range(n + 1):
            u = j / n - .5
            px, py = cx + dx * u, cy + dy * u
            b.box('mid', (px, py, z + hw + .12 + hf / 2), (.22, .22, hf + .1), rz=a)
            b.sphere('emit', (px, py, z + hw + .3 + hf), .09)

def wall_ring(b, R, ox, oy, z, hw, hf, tower_r):
    for k in range(6):
        x1, y1 = corner(R, k, ox, oy); x2, y2 = corner(R, k + 1, ox, oy)
        dx, dy = x2 - x1, y2 - y1; L = math.hypot(dx, dy); ux, uy = dx / L, dy / L
        s0 = tower_r; s1 = L - tower_r
        if k == GATE_EDGE:
            g = 2.6
            wall_segment(b, x1 + ux * s0, y1 + uy * s0, x1 + ux * (L / 2 - g), y1 + uy * (L / 2 - g), z, hw, hf)
            wall_segment(b, x1 + ux * (L / 2 + g), y1 + uy * (L / 2 + g), x1 + ux * s1, y1 + uy * s1, z, hw, hf)
            for sd in (-1, 1):  # gate pillars
                gx, gy = x1 + ux * (L / 2 + sd * g), y1 + uy * (L / 2 + sd * g)
                b.box('mid', (gx, gy, z + (hw + hf + 1.2) / 2), (1.3, 1.6, hw + hf + 1.2), rz=math.atan2(dy, dx))
                b.box('emit', (gx - ux * sd * .66, gy - uy * sd * .66, z + (hw + hf + 1.2) / 2), (.04, 1.0, hw + hf), rz=math.atan2(dy, dx))
            b.box('glass', (x1 + ux * L / 2, y1 + uy * L / 2, z + (hw + hf) / 2 + .1), (2 * g - 1.3, .05, hw + hf), rz=math.atan2(dy, dx))
        else:
            wall_segment(b, x1 + ux * s0, y1 + uy * s0, x1 + ux * s1, y1 + uy * s1, z, hw, hf)

def grow_base(tier, ox=0.0, oy=0.0, mkshift=0, seed=3, people=True):
    rnd = random.Random(seed * 17 + tier)
    b = B()
    nr = NRINGS[tier]
    TER = [WALL_R[k] + 1.6 for k in range(nr)]
    def dz_at(r):
        for k in range(nr):
            if r <= TER[k] * .866 - .3: return .35 * (nr - k)
        return 0.0
    for k in range(nr):
        ztop = .35 * (nr - k)
        hexplate(b, 'mid' if k % 2 == 0 else 'dark', (ox, oy), TER[k], .35, ztop)
        hex_edges(b, 'emit', (ox, oy), TER[k] * .995, ztop + .015, w=.08, hgt=.03)
    b.ring('emit_soft', (ox, oy, .02), PLAT[tier] * 1.1, .08, .02, seg=140)
    dz = .35 * nr
    top = central_tower(b, tier, ox, oy, dz)
    # inner pylon towers round the main tower
    for j in range(PYLON_COUNT[tier]):
        a = PYLON_ORDER[j] * math.pi / 3
        n = tier + 1 - j // 2
        px, py = ox + math.cos(a) * PYLON_R, oy + math.sin(a) * PYLON_R
        pylon(b, rnd, px, py, dz, n, 2.4, a)
        bz = dz + .6 + 3.0 * n * .62
        bridge(b, ox + math.cos(a) * (W0 * .4), oy + math.sin(a) * (W0 * .4), px - math.cos(a) * 1.0, py - math.sin(a) * 1.0, bz)
    # walls and wall towers, ring by ring
    ntow = 1 + PYLON_COUNT[tier]
    for k in range(nr):
        R = WALL_R[k]; z = dz_at(R * .866 - .5)
        age = tier - RING_BUILT[k]                 # older rings have grown taller
        hw = 2.2 + .4 * min(age, 3) - .2 * k
        hf = 1.6 + .4 * min(age, 3)
        wall_ring(b, R, ox, oy, z, hw, hf, 1.6)
        corners = [0, 2, 4] if (k == 0 and tier == 1) else range(6)
        for c in corners:
            cx_, cy_ = corner(R, c, ox, oy); a = math.atan2(cy_ - oy, cx_ - ox)
            n = max(2, 1 + min(age, 2) + (k == 0))
            if k == 3:
                bastion(b, rnd, cx_, cy_, z, 3, a); ntow += 1
            else:
                wt = pylon(b, rnd, cx_, cy_, z, n, 2.6 - .1 * k, a); ntow += 1
                sky_beam(b, cx_, cy_, wt + 3.4)
        if tier == 5 and k == 2:                     # Capital: mid-wall towers on ring 3
            for e in range(6):
                if e == GATE_EDGE: continue
                x1, y1 = corner(R, e, ox, oy); x2, y2 = corner(R, e + 1, ox, oy)
                mx, my = (x1 + x2) / 2, (y1 + y2) / 2
                wt = pylon(b, rnd, mx, my, z, 2, 2.0, math.atan2(my - oy, mx - ox)); ntow += 1
                sky_beam(b, mx, my, wt + 3.4)
    # modules in the courtyards
    mods = []
    def slots():
        out = []
        c1 = SLOTS[tier][0]
        for i in range(c1):
            out.append((1, i, i / c1 * math.tau + .2, 9.6))
        if SLOTS[tier][1]:
            for i, deg in enumerate([0, 60, 180, 240]):
                out.append((2, i, math.radians(deg), 16.0))
        c3 = SLOTS[tier][2]
        if c3:
            angs = [e * 60 for e in range(6)] if c3 == 6 else [e * 60 + sd * 14 for e in range(6) for sd in (-1, 1)]
            for i, deg in enumerate(angs):
                out.append((3, i, math.radians(deg), 22.6))
        return out
    for ring, i, a, R in slots():
        kinds = {1: KINDS1, 2: KINDS2, 3: KINDS3}[ring]
        kind = kinds[i % len(kinds)]
        if kind == 'relay' and tier < 5: kind = 'shield'
        mk = max(1, min(4, tier - ring + 1 - (i % 3 == 2) + mkshift))
        if tier == 1: mk = 1 + (i % 2)
        px, py = ox + math.cos(a) * R, oy + math.sin(a) * R
        z0 = dz_at(R)
        ttop = module_tower(b, kind, px, py, z0, mk, face=a)
        if ring == 1:
            L = R - 3.6; ca = (ox + math.cos(a) * (3.6 + L / 2), oy + math.sin(a) * (3.6 + L / 2))
            b.box('emit_soft', (*ca, dz + .03), (L - 1.6, .1, .03), rz=a)
        mods.append((px, py, ttop, kind, mk))
    # banner poles: a pair per tier, outside the outer wall
    nb = 2 * tier; flags = []
    for i in range(nb):
        a = i / nb * math.tau + math.pi / nb + .15; r = PLAT[tier] * 1.05
        px, py = ox + math.cos(a) * r, oy + math.sin(a) * r; ph = 6 + tier * 1.2
        b.cyl('dark', (px, py, ph / 2), .09, .12, ph, seg=8)
        b.cyl('mid', (px, py, ph + .4), .18, .0, .8, seg=6)
        flags.append((px, py, ph, a))
    b.build('gbase%d_%d' % (tier, int(ox)))
    for j, (px, py, ph, a) in enumerate(flags):
        flag('gflag%d_%d_%d' % (tier, int(ox), j), (px, py, ph - 1.9), a + math.pi / 2, 1.4, 3.4)
    if people:
        gx, gy = corner(PLAT[tier] * .9, 4, ox, oy)
        ga, gb = corner(PLAT[tier] * .9, 5, ox, oy)
        mx, my = (gx + ga) / 2, (gy + gb) / 2
        person(mx * 1.12 - ox * .12 + 1.0, my * 1.12 - oy * .12)
        person(mx * 1.12 - ox * .12 - 1.2, my * 1.12 - oy * .12 - .8)
    print('TOWERS', tier, ntow + len(mods), 'big', ntow, 'top', round(top, 1))
    return top, mods

# ================= shots =================
OUT = '/tmp/claude-0/grow/out/'
os.makedirs(OUT, exist_ok=True)
def big_ground(size):
    ground(size)

if SHOT.startswith('tier'):
    T = int(SHOT[4:])
    big_ground(400)
    top, mods = grow_base(T)
    R = PLAT[T]
    std_lights(R / 12)
    D = R * 1.75 + top * .6
    a = math.radians(-62)
    cam((D * math.cos(a), D * math.sin(a), top * .42 + R * .5), (0, 0, top * .3), lens=30)
    render_setup(int(1080 * RS), int(1350 * RS), OUT + 'tier%d.png' % T)
elif SHOT == 'lineup' or SHOT == 'top':
    big_ground(900)
    x = -120.0; tops = []
    for t in range(1, 6):
        x += PLAT[t] + (6 if t > 1 else 0)
        top, _ = grow_base(t, ox=x, people=True)
        tops.append(top)
        x += PLAT[t]
    std_lights(4.0)
    if SHOT == 'lineup':
        cam((6, -262, 34), (6, 0, 13), lens=36)
        render_setup(int(2400 * RS), int(1100 * RS), OUT + 'lineup.png')
    else:
        cd = bpy.data.cameras.new('c'); cd.type = 'ORTHO'; cd.ortho_scale = 260
        o = bpy.data.objects.new('c', cd); sc.collection.objects.link(o); o.location = (-6, 0, 150); sc.camera = o
        light('SUN', (0, 0, 100), (0, 0, 0), 2.0, (0.8, 0.85, 1.0))
        render_setup(int(2400 * RS), int(560 * RS), OUT + 'top.png')
    print('TOPS', tops)
elif SHOT.startswith('anim'):
    T = int(SHOT[4:])
    big_ground(400)
    top, mods = grow_base(T)
    R = PLAT[T]
    std_lights(R / 12)
    D = R * 1.85 + top * .6
    render_setup(int(720 * RS), int(900 * RS), OUT + 'x.png')
    sc.render.use_persistent_data = True; sc.cycles.max_bounces = 2; sc.cycles.transparent_max_bounces = 10
    sc.cycles.use_adaptive_sampling = True; sc.cycles.adaptive_threshold = .05
    os.makedirs(OUT + 'anim%d' % T, exist_ok=True)
    NF = int(os.environ.get('NF', '120')); F0 = int(os.environ.get('F0', '0')); F1 = int(os.environ.get('F1', str(NF)))
    phase_nodes = [M[k].node_tree.nodes['phase'] for k in ('beamfade', 'beamcore')]
    for f in range(F0, F1):
        t = f / 24.0
        a = math.radians(-75 + 30 * f / NF)          # slow orbit
        cam((D * math.cos(a), D * math.sin(a), top * .42 + R * .5), (0, 0, top * .3), lens=30)
        for j, (objs, bx, by) in enumerate(BEAM_OBJS):
            # random per beam: two drifting sines on each axis with their own speeds, sizes and phases
            r = random.Random(900 + j)
            ang = math.atan2(by, bx); lean = math.radians(r.uniform(1, 5))
            ax1, ax2 = math.radians(r.uniform(3, 10)), math.radians(r.uniform(1, 4))
            f1, f2, f3, f4 = r.uniform(.06, .22), r.uniform(.25, .6), r.uniform(.05, .2), r.uniform(.3, .7)
            p1, p2, p3, p4 = [r.uniform(0, math.tau) for _ in range(4)]
            d = 1 if r.random() < .5 else -1
            rx = -lean * math.sin(ang) + ax1 * math.sin(math.tau * f1 * t * d + p1) + ax2 * math.sin(math.tau * f2 * t + p2)
            ry = lean * math.cos(ang) + ax1 * .8 * math.sin(math.tau * f3 * t + p3) + ax2 * math.cos(math.tau * f4 * t * d + p4)
            for ob in objs:
                ob.rotation_euler = (rx, ry, 0)
        for pn in phase_nodes:
            pn.outputs[0].default_value = t * 9.0       # pulses travel up
        sc.render.filepath = OUT + 'anim%d/f%04d.png' % (T, f)
        bpy.ops.render.render(write_still=True)
    print('DONE', SHOT); sys.exit(0)
elif SHOT == 'modules':
    big_ground(200)
    kinds = ['lattice', 'shield', 'vault', 'uplink', 'collector', 'sensor', 'hangar', 'fabricator']
    b = B()
    for i, k in enumerate(kinds):
        for mk in range(1, 5):
            x = (i - 3.5) * 6.2; y = (mk - 1) * 6.5
            module_tower(b, k, x, y, 0.0, mk, face=-math.pi / 2)
    b.build('mods')
    person(-21.5, -3.5)
    std_lights(1.6)
    cam((0, -46, 19), (0, 7, 2.6), lens=34)
    render_setup(int(2200 * RS), int(1150 * RS), OUT + 'modules.png')

bpy.ops.render.render(write_still=True)
print('DONE', SHOT)
