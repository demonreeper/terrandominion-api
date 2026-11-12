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

    return ob


# ================= Tether Node (hex pylon) =================
def lin(hx):
    c = tuple(((int(hx[i:i+2], 16) / 255) ** 2.2) for i in (0, 2, 4)); m_ = max(c); return tuple(x / m_ for x in c)

RES = {'crystal': lin('59d9ff'), 'alloy': lin('ffb840'), 'data': lin('b885ff'), 'credit': lin('66f28c')}
FACS = {'free': '3a8cff', 'lumen': '4fd8ff', 'forge': 'ffb52e', 'veil': 'b57cff', 'verdant': '4cf08a', 'slip': 'ff5d6c', 'bastion': 'dbe8f5', 'fa': '5b78ff', 'fb': 'ff5fd2', 'fc': 'd4ff3a'}

def fmats(key):
    global FAC
    col = lin(FACS[key]); old = FAC; FAC = col
    M['dark@' + key] = metal('dark@' + key, (0.05, 0.055, 0.07), .4, glow=3.0)
    M['emit@' + key] = emit('emit@' + key, col, 4)
    M['hot@' + key] = emit('hot@' + key, tuple(.55 + .45 * c for c in col), 9)
    M['soft@' + key] = emit('soft@' + key, col, 2.2, .5)
    M['beam@' + key] = emit('beam@' + key, col, 2.5, .3)
    M['glass@' + key] = emit('glass@' + key, col, 1.0, .12)
    M['plate@' + key] = img_mat('plate@' + key, 'hexgrid.png', emit_strength=1.6, col=col, alpha_mul=.8)
    if key not in ('free', 'fa', 'fb', 'fc'):
        M['banner@' + key] = img_mat('banner@' + key, 'banner_%s.png' % key)
    FAC = old
    return col

for r, c in RES.items():
    M['r_' + r] = emit('r_' + r, c, 5)
    M['rs_' + r] = emit('rs_' + r, c, 2.2, .45)
M['dmg'] = emit('dmg', (1, .12, .08), 3)
M['dmg_dim'] = emit('dmg_dim', (1, .12, .08), .8, .5)

def hex_ring(b, mat, c, R, w, h=.03, rz=0.0, tilt=(0, 0)):
    for i in range(6):
        a0 = rz + i * math.pi / 3; a1 = a0 + math.pi / 3
        p0 = Vector((R * math.cos(a0), R * math.sin(a0), 0)); p1 = Vector((R * math.cos(a1), R * math.sin(a1), 0))
        mid = (p0 + p1) / 2; ln = (p1 - p0).length
        ang = math.atan2(p1.y - p0.y, p1.x - p0.x)
        rot = Euler((tilt[0], tilt[1], 0)).to_matrix()
        mm = rot @ mid
        mtx = Matrix.Translation(Vector(c) + mm) @ rot.to_4x4() @ Euler((0, 0, ang)).to_matrix().to_4x4() @ Matrix.Diagonal((ln + w, w, h, 1))
        bmesh.ops.create_cube(b.get(mat), size=1.0, matrix=mtx)

def seg(b, mat, p0, p1, r, sides=6):
    p0 = Vector(p0); p1 = Vector(p1); d = p1 - p0
    q = d.to_track_quat('Z', 'Y')
    mtx = Matrix.Translation((p0 + p1) / 2) @ q.to_matrix().to_4x4()
    bmesh.ops.create_cone(b.get(mat), cap_ends=True, segments=sides, radius1=r, radius2=r, depth=d.length, matrix=mtx)

def pylon(key, tier, ox=0.0, oy=0.0, health=1.0, banners=True, beam=True):
    col = fmats(key); b = B(); o = Vector((ox, oy, 0)); rnd = random.Random(tier * 7 + len(key))
    D, E, H, S, BM, G = 'dark@' + key, 'emit@' + key, 'hot@' + key, 'soft@' + key, 'beam@' + key, 'glass@' + key
    PR = 2.0 if tier == 1 else 2.6
    # --- hex plate: stepped base, inset, rim lights, corner brackets
    b.cyl('mid', o + Vector((0, 0, .09)), PR + .25, PR + .1, .18, seg=6, rz=math.pi / 6)
    b.cyl(D, o + Vector((0, 0, .21)), PR, PR - .12, .1, seg=6, rz=math.pi / 6)
    b.cyl('mid', o + Vector((0, 0, .29)), PR * .55, PR * .5, .1, seg=6, rz=math.pi / 6)
    hex_ring(b, E, o + Vector((0, 0, .265)), PR * .97, .04, .02, rz=math.pi / 6)
    hex_ring(b, S, o + Vector((0, 0, .265)), PR * .72, .03, .015, rz=math.pi / 6)
    for i in range(6):
        a = i * math.pi / 3 + math.pi / 6; R = PR + .17
        cx, cy = ox + R * math.cos(a), oy + R * math.sin(a)
        b.box('light', (cx, cy, .2), (.28, .28, .26), rz=a)
        b.box(H, (ox + (R + .15) * math.cos(a), oy + (R + .15) * math.sin(a), .2), (.05, .12, .05), rz=a)
        # survey ticks between corners
        for t in range(1, 4):
            at = a + t * math.pi / 12
            b.box(E, (ox + (PR + .02) * math.cos(at), oy + (PR + .02) * math.sin(at), .27), (.18, .025, .02), rz=at)
    # --- buttress fins (three, leaning in)
    for i in range(3):
        a = i * math.tau / 3 + .3
        for k in range(4):
            t = k / 3
            r_ = .62 + .55 * (1 - t); z = .45 + 1.1 * t
            b.box(D, (ox + r_ * math.cos(a), oy + r_ * math.sin(a), z), (.38, .12, .38), rz=a, ry=-.5)
        b.box(E, (ox + .95 * math.cos(a), oy + .95 * math.sin(a), .75), (.03, .14, .5), rz=a, ry=-.6)
    # --- tapered hex spire in three stacked segments with glowing bands and slits
    z = .34; r = .62 if tier == 1 else .7; segs = [(1.15, .86), (1.0, .84), (.85, .8)] if tier == 1 else [(1.25, .86), (1.1, .86), (.95, .84), (.7, .8)]
    for i, (h, k) in enumerate(segs):
        r2 = r * k
        b.cyl(D if i % 2 == 0 else 'mid', o + Vector((0, 0, z + h / 2)), r, r2, h, seg=6)
        for f in range(6):
            a = f * math.pi / 3 + math.pi / 6; rr = (r + r2) / 2 * .87 + .01
            b.box(E, (ox + rr * math.cos(a), oy + rr * math.sin(a), z + h * .5), (.025, .07, h * .55), rz=a)
            if rnd.random() < .5:
                b.box('light', (ox + (rr + .02) * math.cos(a + .25), oy + (rr + .02) * math.sin(a + .25), z + h * rnd.uniform(.2, .8)), (.06, .14, .1), rz=a)
        z += h
        b.cyl(E, o + Vector((0, 0, z + .03)), r2 + .05, r2 + .05, .06, seg=6)
        b.cyl('light', o + Vector((0, 0, z + .1)), r2 + .02, r2 * .92, .08, seg=6)
        z += .12; r = r2 * .95
    top = z
    # crown prongs holding the core
    for i in range(3):
        a = i * math.tau / 3
        p0 = o + Vector((r * .8 * math.cos(a), r * .8 * math.sin(a), top)); p1 = o + Vector((.42 * math.cos(a), .42 * math.sin(a), top + .7))
        seg(b, 'mid', p0, p1, .055, 6)
        b.sphere(H, p1, .05)
    # --- floating cut core
    cz = top + 1.05
    b.cyl(H, o + Vector((0, 0, cz + .22)), .3, 0.0, .45, seg=6)
    b.cyl(H, o + Vector((0, 0, cz - .17)), 0.0, .3, .35, seg=6)
    b.sphere(G, o + Vector((0, 0, cz)), .55)
    # orbit rings (more with tier)
    hex_ring(b, E, o + Vector((0, 0, cz)), .8, .03, .03, rz=.2, tilt=(.62, .15))
    if tier >= 2:
        hex_ring(b, E, o + Vector((0, 0, cz)), 1.0, .03, .03, rz=.6, tilt=(-.5, .6))
        hex_ring(b, S, o + Vector((0, 0, cz - .2)), 1.25, .02, .02, rz=0, tilt=(.3, -.35))
    # --- sky beam with rising signal rings
    if beam:
        b.cyl(BM, o + Vector((0, 0, cz + 8)), .14, .02, 15, seg=12)
        b.cyl(E, o + Vector((0, 0, cz + 6)), .02, .008, 11, seg=8)
    # --- perimeter posts, lattice (tier 2), banners
    posts = []
    n = 3 if tier == 1 else 6
    for i in range(n):
        a = i * math.tau / n + math.pi / 6; R = PR + .55
        p = o + Vector((R * math.cos(a), R * math.sin(a), 0)); posts.append((p, a))
        ph = 1.6 if tier == 1 else 2.0
        b.cyl('mid', p + Vector((0, 0, .1)), .16, .14, .2, seg=6)
        b.cyl(D, p + Vector((0, 0, ph / 2)), .06, .05, ph, seg=6)
        b.cyl(E, p + Vector((0, 0, ph + .06)), .07, .07, .12, seg=6)
        b.sphere(H, p + Vector((0, 0, ph + .18)), .045)
    if tier >= 2:
        for i in range(n):
            p0, _ = posts[i]; p1, _ = posts[(i + 1) % n]
            for zz in (.7, 1.35):
                seg(b, S, p0 + Vector((0, 0, zz)), p1 + Vector((0, 0, zz)), .018, 4)
    objs = b.build('tn_' + key)
    # hex-cell glow inset on the plate
    bpy.ops.mesh.primitive_circle_add(vertices=6, radius=PR * .95, fill_type='NGON', location=(ox, oy, .262), rotation=(0, 0, math.pi / 6))
    pl = bpy.context.object; bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.uv.cube_project(cube_size=1.2); bpy.ops.object.mode_set(mode='OBJECT')
    pl.data.materials.append(M['plate@' + key])
    if banners and key in M_BANNER_KEYS:
        for i, (p, a) in enumerate(posts):
            if tier == 1 or i % 2 == 0:
                bn = flag('bn', p + Vector((0, 0, (1.6 if tier == 1 else 2.0) - .1)), a + math.pi / 2, .55, 1.0)
                bn.data.materials.clear(); bn.data.materials.append(M['banner@' + key])
                bn.location = p + Vector((.0, .0, 1.05 if tier == 1 else 1.45))
    # health ring on the ground
    hb = B(); RR = PR + 1.15; N_ = 72
    for i in range(N_):
        if i % 12 in (0,): continue
        a = i / N_ * math.tau + math.pi / 2
        mat = E if i / N_ < health else 'dmg_dim'
        hb.box(mat, (ox + RR * math.cos(a), oy + RR * math.sin(a), .02), (.09, math.tau * RR / N_ * .8, .02), rz=a)
    hb.build('hp_' + key, bevel=False)
    return o, top, cz

def node_at(kind, p, rnd):
    b = B(); p = Vector(p)
    E, S = 'r_' + kind, 'rs_' + kind
    b.cyl('mid', p + Vector((0, 0, .06)), .6, .55, .12, seg=6)
    hex_ring(b, E, p + Vector((0, 0, .13)), .5, .03, .015)
    hex_ring(b, S, p + Vector((0, 0, .015)), .95, .03, .015)
    if kind == 'crystal':
        for i in range(7):
            a = rnd.uniform(0, math.tau); r = 0 if i == 0 else rnd.uniform(.12, .32); h = (1.3 if i == 0 else rnd.uniform(.45, .9))
            tilt = 0 if i == 0 else .35
            b.cyl(E if i % 2 == 0 else S, p + Vector((r * math.cos(a), r * math.sin(a), .12 + h / 2)), .12 if i == 0 else .08, 0.0, h, seg=6, rx=tilt * math.sin(a), ry=-tilt * math.cos(a))
    elif kind == 'alloy':
        for i in range(5):
            b.box('light', p + Vector((0, 0, .2 + i * .14)), (.62 - i * .06, .4 - i * .03, .1), rz=i * .25)
            b.box(E, p + Vector((0, 0, .26 + i * .14)), (.64 - i * .06, .42 - i * .03, .02), rz=i * .25)
    elif kind == 'data':
        c = p + Vector((0, 0, .75)); s = .3
        for sx in (-1, 1):
            for sy in (-1, 1):
                b.box(E, c + Vector((sx * s, sy * s, 0)), (.03, .03, 2 * s))
                b.box(E, c + Vector((sx * s, 0, sy * s)), (.03, 2 * s, .03))
                b.box(E, c + Vector((0, sx * s, sy * s)), (2 * s, .03, .03))
        b.box(S, c, (.3, .3, .3), rz=.6, rx=.6)
        for i in range(4):
            a = i * math.tau / 4 + .4
            b.box(E, c + Vector((.62 * math.cos(a), .62 * math.sin(a), .15 * math.sin(3 * a))), (.09, .09, .09), rz=a)
    elif kind == 'credit':
        for i in range(8):
            a = i * math.tau / 8
            b.cyl(E if i % 2 else S, p + Vector((.42 * math.cos(a), .42 * math.sin(a), .7)), .14, .14, .025, seg=12, ry=math.pi / 2, rz=a + math.pi / 2)
        b.cyl(E, p + Vector((0, 0, .7)), .17, .17, .04, seg=6, rx=math.pi / 2)
    b.build('node_' + kind, bevel=False)

def link(kind, a, bpt, lift=1.6, packets=4, rnd=None):
    b = B(); a = Vector(a); bpt = Vector(bpt); mid = (a + bpt) / 2 + Vector((0, 0, lift))
    pts = [(1 - t) ** 2 * a + 2 * (1 - t) * t * mid + t ** 2 * bpt for t in [i / 40 for i in range(41)]]
    for i in range(40):
        seg(b, 'r_' + kind, pts[i], pts[i + 1], .022, 6)
        seg(b, 'rs_' + kind, pts[i], pts[i + 1], .06, 6)
    for k in range(packets):
        t = (k + .5 + (rnd.uniform(-.2, .2) if rnd else 0)) / packets
        q = (1 - t) ** 2 * a + 2 * (1 - t) * t * mid + t ** 2 * bpt
        b.cyl('r_' + kind, q, .09, .09, .04, seg=6, rx=math.pi / 2)
    b.build('link_' + kind, bevel=False)

def links_for(o, cz, kinds, rnd, dist=7.5, start=0.4):
    for i, k in enumerate(kinds):
        a = start + i * math.tau / max(4, len(kinds)) + rnd.uniform(-.25, .25)
        d = dist + rnd.uniform(-1.2, 1.5)
        np_ = o + Vector((d * math.cos(a), d * math.sin(a), 0))
        node_at(k, np_, rnd)
        link(k, o + Vector((0, 0, cz - .9)), np_ + Vector((0, 0, .8)), lift=.6 + d * .08, rnd=rnd)

def dome(o, R, col, strength=2.0):
    bpy.ops.mesh.primitive_uv_sphere_add(radius=R, segments=48, ring_count=24, location=o); d = bpy.context.object
    d.scale = (1, 1, 1.1)
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.select_all(action='DESELECT'); bpy.ops.object.mode_set(mode='OBJECT')
    for v in d.data.vertices: v.select = v.co.z < -0.01
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.delete(type='VERT'); bpy.ops.object.mode_set(mode='OBJECT')
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.uv.sphere_project(); bpy.ops.object.mode_set(mode='OBJECT')
    m, nt, out = node_mat('dome')
    tc = nt.nodes.new('ShaderNodeTexCoord'); mp = nt.nodes.new('ShaderNodeMapping'); mp.inputs['Scale'].default_value = (10, 5, 1)
    tx = nt.nodes.new('ShaderNodeTexImage'); tx.image = bpy.data.images.load(TEX + 'hexgrid.png')
    lw = nt.nodes.new('ShaderNodeLayerWeight'); lw.inputs[0].default_value = .35
    e = nt.nodes.new('ShaderNodeEmission'); e.inputs[0].default_value = (*col, 1)
    mx = nt.nodes.new('ShaderNodeMath'); mx.operation = 'MULTIPLY_ADD'; mx.inputs[2].default_value = .01
    nt.links.new(tx.outputs['Color'], mx.inputs[0]); nt.links.new(lw.outputs['Facing'], mx.inputs[1])
    st = nt.nodes.new('ShaderNodeMath'); st.operation = 'MULTIPLY'; st.inputs[1].default_value = strength
    nt.links.new(mx.outputs[0], st.inputs[0]); nt.links.new(st.outputs[0], e.inputs[1])
    tr = nt.nodes.new('ShaderNodeBsdfTransparent'); ad = nt.nodes.new('ShaderNodeAddShader')
    nt.links.new(tr.outputs[0], ad.inputs[0]); nt.links.new(e.outputs[0], ad.inputs[1]); nt.links.new(ad.outputs[0], out.inputs[0])
    nt.links.new(tc.outputs['UV'], mp.inputs[0]); nt.links.new(mp.outputs[0], tx.inputs[0]); m.blend_method = 'BLEND'
    d.data.materials.append(m)

def tn_lights(scale=.35, col=None):
    global FAC
    if col: FAC = col
    std_lights(scale)

TIERS = {1: (4.0, 7, 3), 2: (5.5, 10, 6), 3: (7, 13, 9), 4: (8.5, 17, 13), 5: (10, 22, 18)}

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

def tower(b, rnd, tier, ox, oy, z0):
    pr, th, _ = TIERS[tier]
    lv = tier + 2; lh = th / lv; w = 2.6 + tier * .45
    # foundation skirt
    b.cyl('mid', (ox, oy, z0 + .5), w * 1.0, w * .72, 1.0, seg=4, rz=math.pi / 4)
    z = z0 + 1.0
    for k in range(lv):
        rz = (k % 2) * math.pi / 12
        d = w * .86
        b.box('dark', (ox, oy, z + lh * .47), (w, d, lh * .94), rz=rz)
        face_greebles(b, rnd, ox, oy, z, w, d, lh, rz)
        # corner buttresses
        for cx in (-1, 1):
            for cy in (-1, 1):
                x, y = cx * w * .5, cy * d * .5
                ca, sa = math.cos(rz), math.sin(rz)
                b.box('mid', (ox + x * ca - y * sa, oy + x * sa + y * ca, z + lh * .5), (w * .16, w * .16, lh * 1.02), rz=rz)
        # cantilevered blocks and side masses (jagged silhouette)
        for j in range(3):
            a2 = rnd.uniform(0, math.tau); r2 = w * rnd.uniform(.45, .62)
            cw = w * rnd.uniform(.22, .4); chh = lh * rnd.uniform(.3, .8)
            b.box(rnd.choice(['dark', 'mid']), (ox + math.cos(a2) * r2, oy + math.sin(a2) * r2, z + rnd.uniform(.1, .6) * lh), (cw, cw * .8, chh), rz=a2)
        if k < lv - 1:
            for sd in (-1, 1):
                sw2 = w * .38
                b.box('dark', (ox + sd * w * .62, oy + w * .1, z + lh * .4), (sw2, sw2 * 1.1, lh * .8), rz=rz)
                b.box('emit', (ox + sd * (w * .62 + sw2 * .5 + .01), oy + w * .1, z + lh * .4), (.02, sw2 * .7, .04), rz=rz)
        # ledge + glow band
        b.box('light', (ox, oy, z + lh * .97), (w * 1.1, d * 1.1, .09), rz=rz)
        b.box('emit', (ox, oy, z + lh * .9), (w * 1.02, d * 1.02, .03), rz=rz)
        z += lh; w *= .84
    # crown
    b.box('mid', (ox, oy, z + .3), (w * .9, w * .8, .6))
    for i in range(3 + tier // 2):
        a = i * 2.1; r = w * .3 * (i % 2)
        hgt = rnd.uniform(1.5, 3.2) + tier * .4
        b.cyl('dark', (ox + r * math.cos(a), oy + r * math.sin(a), z + .6 + hgt / 2), .06 + .03 * (i == 0), .02, hgt, seg=8)
        b.sphere('emit', (ox + r * math.cos(a), oy + r * math.sin(a), z + .6 + hgt), .06)
    b.sphere('emit', (ox, oy, z + 1.0), .32)
    return z

def module(b, rnd, c, s):
    x, y, z = c
    b.box('dark', (x, y, z), (s * .78, s * .78, s * .78))
    for f in range(4):
        a = f * math.pi / 2
        b.box('mid', (x + math.cos(a) * s * .4, y + math.sin(a) * s * .4, z), (s * .05 if f % 2 == 0 else s * .55, s * .55 if f % 2 == 0 else s * .05, s * .55))
        b.box('emit', (x + math.cos(a) * s * .43, y + math.sin(a) * s * .43, z + s * .2), (s * .03 if f % 2 == 0 else s * .45, s * .45 if f % 2 == 0 else s * .03, s * .04))
    b.box('light', (x, y, z + s * .41), (s * .5, s * .5, .05))
    for i in range(4):
        b.box('mid', (x + rnd.uniform(-.25, .25) * s, y + rnd.uniform(-.25, .25) * s, z + s * .45), (s * .12, s * .12, s * .1))
    # glowing frame (12 edges)
    e = s * .5; t = .035
    for sx in (-1, 1):
        for sy in (-1, 1):
            b.box('emit', (x + sx * e, y + sy * e, z), (t, t, s))
            b.box('emit', (x + sx * e, y, z + sy * e), (t, s, t))
            b.box('emit', (x, y + sx * e, z + sy * e), (s, t, t))
    b.box('glass', (x, y, z), (s * 1.0, s * 1.0, s * 1.0))


# ================= Unity export =================
# Every asset: pivot at ground centre, metres, separate child meshes by role:
#   Hull (baked PBR textures), Glow (solid emissive, white: tint in Unity), GlowSoft (transparent glow),
#   Beam (transparent beam), Glass (transparent halo), PlateGrid (hex-cell inset), Banner_n (banner cloth).
EXP = '/tmp/claude-0/bl3/export/'
os.makedirs(EXP, exist_ok=True)
ASSET = os.environ.get('ASSET', 'tn_t1')
TEXRES = int(os.environ.get('TEXRES', '2048'))

def role(matname):
    n = matname
    if n.startswith(('dark', 'mid', 'light')): return 'Hull'
    if n.startswith(('emit_soft', 'soft@', 'rs_')): return 'GlowSoft'
    if n.startswith(('beam',)): return 'Beam'
    if n.startswith(('glass',)): return 'Glass'
    if n.startswith(('plate@',)): return 'PlateGrid'
    if n.startswith(('banner',)): return 'Banner'
    if n.startswith(('emit', 'hot@', 'r_', 'white')): return 'Glow'
    return 'Hull'

def flat_mat(name, col, strength, alpha):
    m, nt, out = node_mat(name)
    p = nt.nodes.new('ShaderNodeBsdfPrincipled')
    p.inputs['Base Color'].default_value = (*col, 1); p.inputs['Emission Color'].default_value = (*col, 1)
    p.inputs['Emission Strength'].default_value = strength; p.inputs['Alpha'].default_value = alpha
    if alpha < 1: m.blend_method = 'BLEND'
    nt.links.new(p.outputs[0], out.inputs[0]); return m

def split_by_role():
    """Apply modifiers, split every mesh into per-role objects, join each role."""
    groups = {}
    for ob in list(sc.objects):
        if ob.type != 'MESH': continue
        bpy.context.view_layer.objects.active = ob
        for md in list(ob.modifiers):
            if md.type == 'BEVEL': md.segments = 1; md.harden_normals = True
            bpy.ops.object.select_all(action='DESELECT'); ob.select_set(True)
            bpy.ops.object.modifier_apply(modifier=md.name)
        # apply transforms so everything is in asset space
        bpy.ops.object.select_all(action='DESELECT'); ob.select_set(True)
        bpy.ops.object.transform_apply(location=True, rotation=True, scale=True)
        r = role(ob.data.materials[0].name if ob.data.materials else 'dark')
        if r == 'Banner':
            groups.setdefault('Banner_%d' % len([k for k in groups if k.startswith('Banner')]), []).append(ob)
        else:
            groups.setdefault(r, []).append(ob)
    out = {}
    for r, obs in groups.items():
        bpy.ops.object.select_all(action='DESELECT')
        for o in obs: o.select_set(True)
        bpy.context.view_layer.objects.active = obs[0]
        if len(obs) > 1: bpy.ops.object.join()
        o = bpy.context.view_layer.objects.active; o.name = r; o.data.name = ASSET + '_' + r
        out[r] = o
    return out

def bake_hull(o):
    # drop faces nobody sees (undersides on the ground)
    import bmesh as _bm
    bm = _bm.new(); bm.from_mesh(o.data)
    dead = [f for f in bm.faces if f.normal.z < -.99 and f.calc_center_median().z < .05]
    _bm.ops.delete(bm, geom=dead, context='FACES'); bm.to_mesh(o.data); bm.free()
    # UVs
    bpy.ops.object.select_all(action='DESELECT'); o.select_set(True); bpy.context.view_layer.objects.active = o
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.select_all(action='SELECT')
    bpy.ops.uv.smart_project(angle_limit=math.radians(66), island_margin=.004, scale_to_bounds=True)
    bpy.ops.object.mode_set(mode='OBJECT')
    imgs = {}
    for k, cs in (('BaseColor', 'sRGB'), ('Roughness', 'Non-Color'), ('Normal', 'Non-Color'), ('Emission', 'sRGB')):
        im = bpy.data.images.new('%s_%s' % (ASSET, k), TEXRES, TEXRES, alpha=False, float_buffer=False)
        im.colorspace_settings.name = cs; imgs[k] = im
    # put an image node (active) in every material on the hull
    for m in o.data.materials:
        nt = m.node_tree
        n = nt.nodes.get('BAKE') or nt.nodes.new('ShaderNodeTexImage'); n.name = 'BAKE'
        nt.nodes.active = n
    sc.render.engine = 'CYCLES'; sc.cycles.device = 'CPU'; sc.cycles.samples = 8
    sc.render.bake.margin = 6; sc.render.bake.use_clear = True
    for k, typ in (('BaseColor', 'DIFFUSE'), ('Roughness', 'ROUGHNESS'), ('Normal', 'NORMAL'), ('Emission', 'EMIT')):
        for m in o.data.materials: m.node_tree.nodes['BAKE'].image = imgs[k]
        kw = {}
        if typ == 'DIFFUSE': sc.render.bake.use_pass_direct = False; sc.render.bake.use_pass_indirect = False; sc.render.bake.use_pass_color = True
        bpy.ops.object.bake(type=typ, **kw)
        imgs[k].filepath_raw = EXP + ASSET + '/%s_%s.png' % (ASSET, k); imgs[k].file_format = 'PNG'; imgs[k].save()
    # Unity metallic map: R = metallic, A = smoothness (1 - roughness)
    import numpy as np
    rp = np.empty(TEXRES * TEXRES * 4, dtype=np.float32); imgs['Roughness'].pixels.foreach_get(rp); rp = rp.reshape(TEXRES, TEXRES, 4)
    ms = np.zeros_like(rp); ms[..., 0] = .65; ms[..., 1] = .65; ms[..., 2] = .65; ms[..., 3] = 1 - rp[..., 0]
    mi = bpy.data.images.new(ASSET + '_MetallicSmoothness', TEXRES, TEXRES, alpha=True); mi.colorspace_settings.name = 'Non-Color'
    mi.pixels.foreach_set(ms.astype(np.float32).ravel()); mi.filepath_raw = EXP + ASSET + '/%s_MetallicSmoothness.png' % ASSET; mi.file_format = 'PNG'; mi.save()
    # one clean export material referencing the baked maps
    m, nt, out = node_mat('TR_Hull_' + ASSET)
    p = nt.nodes.new('ShaderNodeBsdfPrincipled'); p.inputs['Metallic'].default_value = .65
    def tx(im):
        t = nt.nodes.new('ShaderNodeTexImage'); t.image = im; return t
    nt.links.new(tx(imgs['BaseColor']).outputs[0], p.inputs['Base Color'])
    nt.links.new(tx(imgs['Roughness']).outputs[0], p.inputs['Roughness'])
    nm = nt.nodes.new('ShaderNodeNormalMap'); nt.links.new(tx(imgs['Normal']).outputs[0], nm.inputs['Color']); nt.links.new(nm.outputs[0], p.inputs['Normal'])
    nt.links.new(tx(imgs['Emission']).outputs[0], p.inputs['Emission Color']); p.inputs['Emission Strength'].default_value = 1
    nt.links.new(p.outputs[0], out.inputs[0])
    o.data.materials.clear(); o.data.materials.append(m)

ROLE_MAT = {}
def role_mats():
    W = (1, 1, 1)
    ROLE_MAT['Glow'] = flat_mat('TR_Glow', W, 4, 1)
    ROLE_MAT['GlowSoft'] = flat_mat('TR_GlowSoft', W, 2, .5)
    ROLE_MAT['Beam'] = flat_mat('TR_Beam', W, 2.5, .3)
    ROLE_MAT['Glass'] = flat_mat('TR_Glass', W, 1, .12)
    m, nt, out = node_mat('TR_PlateGrid')
    p = nt.nodes.new('ShaderNodeBsdfPrincipled'); t = nt.nodes.new('ShaderNodeTexImage'); t.image = bpy.data.images.load(TEX + 'hexgrid.png')
    nt.links.new(t.outputs['Color'], p.inputs['Base Color']); nt.links.new(t.outputs['Color'], p.inputs['Emission Color']); nt.links.new(t.outputs['Alpha'], p.inputs['Alpha'])
    p.inputs['Emission Strength'].default_value = 1.6; m.blend_method = 'BLEND'; nt.links.new(p.outputs[0], out.inputs[0]); ROLE_MAT['PlateGrid'] = m
    m, nt, out = node_mat('TR_Banner')
    p = nt.nodes.new('ShaderNodeBsdfPrincipled'); t = nt.nodes.new('ShaderNodeTexImage'); t.image = bpy.data.images.load(TEX + 'banner.png')
    nt.links.new(t.outputs['Color'], p.inputs['Base Color']); nt.links.new(t.outputs['Alpha'], p.inputs['Alpha']); m.blend_method = 'BLEND'
    nt.links.new(p.outputs[0], out.inputs[0]); ROLE_MAT['Banner'] = m

def finish():
    os.makedirs(EXP + ASSET, exist_ok=True)
    parts = split_by_role()
    role_mats()
    for r, o in parts.items():
        if r == 'Hull': bake_hull(o)
        else:
            o.data.materials.clear(); o.data.materials.append(ROLE_MAT['Banner' if r.startswith('Banner') else r])
    # parent under one root empty named after the asset
    root = bpy.data.objects.new(ASSET, None); sc.collection.objects.link(root)
    for o in parts.values(): o.parent = root
    tris = sum(sum(len(p.vertices) - 2 for p in o.data.polygons) for o in parts.values())
    bpy.ops.object.select_all(action='SELECT')
    bpy.ops.export_scene.fbx(filepath=EXP + ASSET + '/%s.fbx' % ASSET, use_selection=True, object_types={'EMPTY', 'MESH'},
                             apply_unit_scale=True, apply_scale_options='FBX_SCALE_ALL', axis_forward='-Z', axis_up='Y',
                             bake_space_transform=True, mesh_smooth_type='FACE', path_mode='COPY', embed_textures=False, add_leaf_bones=False)
    bpy.ops.export_scene.gltf(filepath=EXP + ASSET + '/%s.glb' % ASSET, export_format='GLB', use_selection=True)
    print('PARTS', {r: len(o.data.polygons) for r, o in parts.items()}, 'TRIS', tris)

# ---------- assets ----------
for k in list(FACS): FACS[k] = 'ffffff'
rnd = random.Random(5)
if ASSET in ('tn_t1', 'tn_t2'):
    M_BANNER_KEYS = ('lumen',)
    tier = int(ASSET[-1])
    o, top, cz = pylon('lumen', tier, beam=True)
    for ob in list(sc.objects):
        if ob.name.startswith('hp_'): bpy.data.objects.remove(ob)
elif ASSET.startswith('base_tower_t'):
    t = int(ASSET[-1]); b = B(); tower(b, random.Random(4 * 31 + t), t, 0, 0, .9); b.build(ASSET)
elif ASSET.startswith('base_platform_t'):
    t = int(ASSET[-1]); pr, th, nm = TIERS[t]; b = B()
    b.cyl('mid', (0, 0, .3), pr * 1.08, pr * 1.0, .6, seg=6, rz=math.pi / 6)
    b.cyl('dark', (0, 0, .75), pr * .8, pr * .76, .3, seg=6, rz=math.pi / 6)
    for k in range(6):
        a = k * math.pi / 3
        b.box('light', (math.cos(a) * pr * .93, math.sin(a) * pr * .93, .62), (pr * .9, .25, .05), rz=a + math.pi / 2)
    hex_ring(b, 'emit', (0, 0, .62), pr * .9, .05, .02, rz=math.pi / 6)
    hex_ring(b, 'emit', (0, 0, .91), pr * .62, .04, .02, rz=math.pi / 6)
    b.ring('emit_soft', (0, 0, .02), pr * 1.3, .07, .02, seg=72)
    b.ring('emit_soft', (0, 0, .02), pr * 1.55, .05, .02, seg=72)
    for k in range(3):
        b.box('mid', (0, -pr * 1.0 - .35 + k * .35, .1 + k * .2), (2.2, .4, .2 + k * .4))
    b.build(ASSET)
elif ASSET == 'base_module':
    b = B(); module(b, rnd, (0, 0, .5), 1.0); b.build(ASSET)
elif ASSET == 'base_module_pad':
    b = B(); b.cyl('light', (0, 0, .05), .75, .7, .1, seg=6); b.ring('emit', (0, 0, .11), .6, .04, .02, seg=24); b.build(ASSET)
elif ASSET == 'base_arm':
    b = B(); b.box('mid', (.5, 0, 0), (1.0, .16, .16)); b.box('emit', (.5, 0, .1), (1.0, .03, .03)); b.build(ASSET)
elif ASSET == 'base_turret':
    b = B()
    b.cyl('mid', (0, 0, .25), .45, .55, .5, seg=8); b.box('dark', (0, 0, .7), (.9, .75, .45))
    for sd in (-.18, .18): b.cyl('light', (.85, sd, .75), .06, .07, 1.2, seg=8, ry=math.pi / 2)
    b.box('emit', (.46, 0, .8), (.04, .5, .06)); b.build(ASSET)
elif ASSET == 'base_banner_pole':
    b = B(); ph = 6.0
    b.cyl('dark', (0, 0, ph / 2), .07, .09, ph, seg=8); b.cyl('mid', (0, 0, ph + .35), .15, .0, .7, seg=6); b.box('mid', (0, 0, ph - .1), (.1, .1, .1))
    b.build(ASSET)
    M['banner'] = M['banner@lumen'] if 'banner@lumen' in M else img_mat('banner', 'banner.png')
    fo = flag('banner', (0, 0, ph - 1.55), math.pi / 2, 1.1, 2.8); fo.data.materials.clear(); fo.data.materials.append(img_mat('banner_x', 'banner.png'))
elif ASSET in ('base_drone', 'base_drone_quad'):
    NR = 4 if ASSET.endswith('quad') else 3
    # patrol drone: base_drone has three rotors (in the game), base_drone_quad four, ~1.3 m wide, pivot at body centre
    b = B()
    b.cyl('dark', (0, 0, 0), .26, .2, .16, seg=6, rz=math.pi / 6)
    b.cyl('mid', (0, 0, .11), .18, .1, .08, seg=6, rz=math.pi / 6)
    b.cyl('dark', (0, 0, -.12), .16, .1, .1, seg=6, rz=math.pi / 6)
    hex_ring(b, 'emit', (0, 0, .0), .265, .02, .03, rz=math.pi / 6)
    b.cyl('emit', (0, 0, -.18), .07, .05, .03, seg=12)
    for i in range(NR):
        a = i * math.tau / NR + (math.pi / 2 if NR == 3 else math.pi / 4)
        ca, sa = math.cos(a), math.sin(a)
        b.box('mid', (ca * .36, sa * .36, .01), (.34, .07, .05), rz=a)
        b.box('emit', (ca * .36, sa * .36, .04), (.28, .015, .012), rz=a)
        px, py = ca * .58, sa * .58
        b.cyl('dark', (px, py, .02), .2, .2, .07, seg=16)
        b.cyl('mid', (px, py, .02), .17, .17, .072, seg=16)
        b.ring('emit', (px, py, .06), .185, .02, .012, seg=32)
        b.cyl('light', (px, py, .03), .03, .03, .05, seg=8)
        for k in range(3):  # three blades, each from the hub outward (no crossing bars)
            ak = a + k * math.tau / 3 + .3
            b.box('emit_soft', (px + math.cos(ak) * .08, py + math.sin(ak) * .08, .035), (.15, .035, .006), rz=ak)
        b.box('emit', (ca * .26, sa * .26, -.05), (.04, .05, .02), rz=a)
    b.cyl('dark', (0, -.05, .2), .012, .006, .2, seg=6)
    b.sphere('emit', (0, -.05, .3), .018)
    b.build(ASSET)
finish()
print('DONE', ASSET)
