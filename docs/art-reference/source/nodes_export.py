# Unity export of the grounded resource nodes (mock-ups in nodes.py, approved by the owner 2026-11-13).
# Same layout as export.py: metres, pivot at the ground centre, Y up, one root per asset with child meshes by role:
#   Hull (baked BaseColor / MetallicSmoothness / Normal / Emission), Glow (solid, white: tinted in Unity with the
#   resource colour), GlowSoft (half-transparent glow, white). The soft glow pool of the mock-up is left out (the game
#   draws its own ring and halo).
#   blender -b -P nodes_export.py -- <kind|all> <outdir>      kind: crystal | alloy | data | credit
import bpy, bmesh, math, os, sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import nodes  # noqa: E402  (the mock-up builders; its renders only run as __main__)

ARGS = sys.argv[sys.argv.index('--') + 1:] if '--' in sys.argv else []
KIND = ARGS[0] if len(ARGS) > 0 else 'all'
OUT = ARGS[1] if len(ARGS) > 1 else os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'nodes', 'export')
TEXRES = 1024


def role(matname):
    n = matname
    if n.endswith('_pool'): return None
    if n in ('crystal_glass', 'data_soft'): return 'GlowSoft'
    if n.endswith(('_seam', '_glow', '_m')): return 'Glow'
    return 'Hull'


def flat_mat(name, strength, alpha):
    m, nt, out = nodes.mat(name)
    p = nt.nodes.new('ShaderNodeBsdfPrincipled')
    p.inputs['Base Color'].default_value = (1, 1, 1, 1); p.inputs['Emission Color'].default_value = (1, 1, 1, 1)
    p.inputs['Emission Strength'].default_value = strength; p.inputs['Alpha'].default_value = alpha
    if alpha < 1: m.blend_method = 'BLEND'
    nt.links.new(p.outputs[0], out.inputs[0]); return m


def split_by_role(asset):
    sc = bpy.context.scene; groups = {}
    for ob in list(sc.objects):
        if ob.type != 'MESH':
            continue
        r = role(ob.data.materials[0].name if ob.data.materials else 'hull')
        if r is None:
            bpy.data.objects.remove(ob); continue
        bpy.ops.object.select_all(action='DESELECT'); ob.select_set(True); bpy.context.view_layer.objects.active = ob
        if ob.parent is not None:
            bpy.ops.object.parent_clear(type='CLEAR_KEEP_TRANSFORM')
        for md in list(ob.modifiers):
            if md.type == 'BEVEL': md.segments = 1; md.harden_normals = True
            bpy.ops.object.modifier_apply(modifier=md.name)
        bpy.ops.object.transform_apply(location=True, rotation=True, scale=True)
        groups.setdefault(r, []).append(ob)
    for ob in list(sc.objects):
        if ob.type == 'EMPTY': bpy.data.objects.remove(ob)
    out = {}
    for r, obs in groups.items():
        bpy.ops.object.select_all(action='DESELECT')
        for o in obs: o.select_set(True)
        bpy.context.view_layer.objects.active = obs[0]
        if len(obs) > 1: bpy.ops.object.join()
        o = bpy.context.view_layer.objects.active; o.name = r; o.data.name = asset + '_' + r
        out[r] = o
    return out


def bake_hull(o, asset, folder):
    import numpy as np
    sc = bpy.context.scene
    bm = bmesh.new(); bm.from_mesh(o.data)
    dead = [f for f in bm.faces if f.normal.z < -.99 and f.calc_center_median().z < .05]
    bmesh.ops.delete(bm, geom=dead, context='FACES'); bm.to_mesh(o.data); bm.free()
    bpy.ops.object.select_all(action='DESELECT'); o.select_set(True); bpy.context.view_layer.objects.active = o
    bpy.ops.object.mode_set(mode='EDIT'); bpy.ops.mesh.select_all(action='SELECT')
    bpy.ops.uv.smart_project(angle_limit=math.radians(66), island_margin=.006, scale_to_bounds=True)
    bpy.ops.object.mode_set(mode='OBJECT')
    imgs = {}
    for k, cs in (('BaseColor', 'sRGB'), ('Roughness', 'Non-Color'), ('Normal', 'Non-Color'), ('Emission', 'sRGB')):
        im = bpy.data.images.new('%s_%s' % (asset, k), TEXRES, TEXRES, alpha=False)
        im.colorspace_settings.name = cs; imgs[k] = im
    for m in o.data.materials:
        nt = m.node_tree; n = nt.nodes.get('BAKE') or nt.nodes.new('ShaderNodeTexImage'); n.name = 'BAKE'; nt.nodes.active = n
    sc.render.engine = 'CYCLES'; sc.cycles.device = 'CPU'; sc.cycles.samples = 8
    sc.render.bake.margin = 6; sc.render.bake.use_clear = True
    for k, typ in (('BaseColor', 'DIFFUSE'), ('Roughness', 'ROUGHNESS'), ('Normal', 'NORMAL'), ('Emission', 'EMIT')):
        for m in o.data.materials: m.node_tree.nodes['BAKE'].image = imgs[k]
        if typ == 'DIFFUSE':
            sc.render.bake.use_pass_direct = False; sc.render.bake.use_pass_indirect = False; sc.render.bake.use_pass_color = True
        bpy.ops.object.bake(type=typ)
        imgs[k].filepath_raw = os.path.join(folder, '%s_%s.png' % (asset, k)); imgs[k].file_format = 'PNG'; imgs[k].save()
    # Unity metallic map: R = metallic, A = smoothness (1 - roughness).
    rp = np.empty(TEXRES * TEXRES * 4, dtype=np.float32); imgs['Roughness'].pixels.foreach_get(rp); rp = rp.reshape(TEXRES, TEXRES, 4)
    ms = np.zeros_like(rp); ms[..., 0] = ms[..., 1] = ms[..., 2] = .6; ms[..., 3] = 1 - rp[..., 0]
    mi = bpy.data.images.new(asset + '_MetallicSmoothness', TEXRES, TEXRES, alpha=True); mi.colorspace_settings.name = 'Non-Color'
    mi.pixels.foreach_set(ms.astype(np.float32).ravel()); mi.filepath_raw = os.path.join(folder, '%s_MetallicSmoothness.png' % asset)
    mi.file_format = 'PNG'; mi.save()
    os.remove(os.path.join(folder, '%s_Roughness.png' % asset))
    m, nt, out = nodes.mat('TR_Hull_' + asset)
    p = nt.nodes.new('ShaderNodeBsdfPrincipled'); p.inputs['Metallic'].default_value = .6

    def tx(im):
        t = nt.nodes.new('ShaderNodeTexImage'); t.image = im; return t
    nt.links.new(tx(imgs['BaseColor']).outputs[0], p.inputs['Base Color'])
    nm = nt.nodes.new('ShaderNodeNormalMap'); nt.links.new(tx(imgs['Normal']).outputs[0], nm.inputs['Color']); nt.links.new(nm.outputs[0], p.inputs['Normal'])
    nt.links.new(tx(imgs['Emission']).outputs[0], p.inputs['Emission Color']); p.inputs['Emission Strength'].default_value = 1
    nt.links.new(p.outputs[0], out.inputs[0])
    o.data.materials.clear(); o.data.materials.append(m)


def export(kind):
    asset = 'node_' + kind
    folder = os.path.join(OUT, asset); os.makedirs(folder, exist_ok=True)
    nodes.reset()
    nodes.BUILD[kind](0)
    parts = split_by_role(asset)
    mats = {'Glow': flat_mat('TR_Glow', 4, 1), 'GlowSoft': flat_mat('TR_GlowSoft', 2, .5)}
    for r, o in parts.items():
        if r == 'Hull': bake_hull(o, asset, folder)
        else: o.data.materials.clear(); o.data.materials.append(mats[r])
    root = bpy.data.objects.new(asset, None); bpy.context.scene.collection.objects.link(root)
    for o in parts.values(): o.parent = root
    tris = sum(sum(len(p.vertices) - 2 for p in o.data.polygons) for o in parts.values())
    bpy.ops.object.select_all(action='SELECT')
    bpy.ops.export_scene.fbx(filepath=os.path.join(folder, asset + '.fbx'), use_selection=True, object_types={'EMPTY', 'MESH'},
                             apply_unit_scale=True, apply_scale_options='FBX_SCALE_ALL', axis_forward='-Z', axis_up='Y',
                             bake_space_transform=True, mesh_smooth_type='FACE', path_mode='COPY', embed_textures=False, add_leaf_bones=False)
    print('EXPORTED', asset, {r: len(o.data.polygons) for r, o in parts.items()}, 'TRIS', tris)


for k in nodes.BUILD:
    if KIND in ('all', k):
        export(k)
