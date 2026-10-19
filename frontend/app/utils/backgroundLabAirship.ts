import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'
const TAU = Math.PI * 2
type Vec = { x: number, y: number, z: number }
/** Ellipsoid silhouette never collapses when the longitudinal axis faces the camera. */
export function airshipProjectedRadius(yaw: number, length = 25, radius = 9.5) {
  return Math.hypot(length * Math.cos(yaw), radius * Math.sin(yaw))
}
export function cityAirshipOrbit(t: number, interaction?: LabInteraction) {
  const state = labTarget(interaction, 'citybiz-airship', 'drive')
  const pulse = Math.min(1, Math.max(0, state.position) * 1.8)
  // Integrated drive distance preserves the new position as speed returns to cruise.
  const angle = t * .08 + state.phase * .13 - .45, radius = 4.5
  const x = radius * Math.cos(angle), y = radius * Math.sin(angle)
  const depth = 1 + (Math.cos(angle) + Math.sin(angle)) / Math.SQRT2 * .14
  // Keep the existing argument name, now a true yaw angle rather than a signed flattening scale.
  const facing = Math.atan2(Math.cos(angle) - Math.sin(angle), -Math.sin(angle) - Math.cos(angle))
  return { x, y, angle, z: 270 + pulse * 8 + Math.sin(t * .42) * 3, near: x + y >= 0, depth, pulse, facing }
}
/** Latitudinal hull panels and hardware share one depth-sorted three-dimensional projection. */
export function paintCityAirship(ctx: CanvasRenderingContext2D, x: number, y: number, scale: number, t: number, depth: number, pulse: number, facing: number, interaction?: LabInteraction) {
  const state = labTarget(interaction, 'citybiz-airship', 'drive'), unit = 50 * depth / scale
  const cos = Math.cos(facing), sin = Math.sin(facing)
  const project = (p: Vec) => {
    const d = -p.x * sin + p.z * cos
    return { x: (p.x * cos + p.z * sin) * unit, y: (p.y - d * .14) * unit, depth: d + p.y * .14 }
  }
  const panels: { points: ReturnType<typeof project>[], color: string, depth: number }[] = []
  const add = (vertices: Vec[], color: string) => {
    const points = vertices.map(project)
    panels.push({ points, color, depth: points.reduce((sum, p) => sum + p.depth, 0) / points.length })
  }
  const hull = (a: number, b: number): Vec => ({ x: Math.cos(a) * .49, y: Math.sin(a) * Math.sin(b) * .19, z: Math.sin(a) * Math.cos(b) * .19 })
  // Fine mesh: shading follows rotated surface normals and a stable upper-left light source.
  for (let lat = 0; lat < 18; lat++) for (let lon = 0; lon < 36; lon++) {
    const a = lat / 18 * Math.PI, b = lon / 36 * TAU, da = Math.PI / 18, db = TAU / 36
    const normal = { x: Math.cos(a + da / 2) * .4, y: Math.sin(a + da / 2) * Math.sin(b + db / 2), z: Math.sin(a + da / 2) * Math.cos(b + db / 2) }
    const light = Math.max(0, Math.min(1, .5 - normal.y * .35 - (normal.x * cos + normal.z * sin) * .19 + (-normal.x * sin + normal.z * cos) * .14))
    const stripe = lat === 5 || lat === 12 ? -5 : 0
    const color = `rgb(${Math.round(142 + light * 102 + stripe)},${Math.round(119 + light * 105 + stripe)},${Math.round(77 + light * 111 + stripe)})`
    add([hull(a, b), hull(a + da, b), hull(a + da, b + db), hull(a, b + db)], color)
  }
  // Tail stabilizers are actual planes in two perpendicular orientations, not mirrored stickers.
  add([{ x: -.32, y: -.04, z: 0 }, { x: -.53, y: -.3, z: 0 }, { x: -.52, y: .04, z: 0 }], '#58796a')
  add([{ x: -.32, y: .04, z: 0 }, { x: -.52, y: .22, z: 0 }, { x: -.52, y: -.02, z: 0 }], '#75917b')
  add([{ x: -.32, y: 0, z: -.02 }, { x: -.53, y: 0, z: -.29 }, { x: -.53, y: 0, z: .29 }], '#567868')
  const box = (x0: number, x1: number, y0: number, y1: number, z0: number, z1: number) => {
    add([{ x: x0, y: y0, z: z0 }, { x: x1, y: y0, z: z0 }, { x: x1, y: y0, z: z1 }, { x: x0, y: y0, z: z1 }], '#e5d7b1')
    for (const z of [z0, z1]) add([{ x: x0, y: y0, z }, { x: x1, y: y0, z }, { x: x1, y: y1, z }, { x: x0, y: y1, z }], z > 0 ? '#557f78' : '#46685e')
    for (const xx of [x0, x1]) add([{ x: xx, y: y0, z: z0 }, { x: xx, y: y1, z: z0 }, { x: xx, y: y1, z: z1 }, { x: xx, y: y0, z: z1 }], '#9d8256')
  }
  box(-.15, .17, .2, .29, -.065, .065)
  for (const xx of [-.12, .12]) box(xx - .008, xx + .008, .13, .22, -.01, .01)
  // Exhaust belongs to the two real engine outlets and rotates with their local aft axis.
  // Finite translucent layers fade with the same damped drive response; no particle loop/state.
  if (pulse > .003) for (const z of [-.16, .16]) {
    const length = .12 + pulse * .55
    for (let layer = 0; layer < 7; layer++) {
      const u = layer / 7, v = (layer + 1) / 7
      const width = .018 + u * .05
      const flicker = Math.sin(t * 25 + layer * .8) * .008 * pulse
      add([
        { x: -.28 - u * length, y: .18 - width, z: z + flicker },
        { x: -.28 - v * length, y: .18 - width * .85, z: z + flicker },
        { x: -.28 - v * length, y: .18 + width * .85, z: z + flicker },
        { x: -.28 - u * length, y: .18 + width, z: z + flicker },
      ], `rgba(119,201,222,${pulse * (1 - u) * .57})`)
      add([
        { x: -.28 - u * length, y: .172, z: z - .012 },
        { x: -.28 - v * length, y: .172, z: z - .009 },
        { x: -.28 - v * length, y: .188, z: z + .009 },
        { x: -.28 - u * length, y: .188, z: z + .012 },
      ], `rgba(246,221,157,${pulse * (1 - u) * .8})`)
    }
  }
  // Propeller rotates in its own transverse plane; projection preserves its depth relationship.
  const spin = t * (6 + pulse * 24) + state.phase
  for (const z of [-.16, .16]) {
    box(-.25, -.13, .15, .205, z - .025, z + .025)
    const blade = (offset: number): Vec => ({ x: -.26, y: .18 + Math.cos(spin + offset) * .1, z: z + Math.sin(spin + offset) * .1 })
    add([blade(0), blade(.12), blade(Math.PI), blade(Math.PI + .12)], '#345d4d')
  }
  ctx.save(); ctx.translate(x, y)
  panels.sort((a, b) => a.depth - b.depth)
  for (const panel of panels) {
    ctx.beginPath(); panel.points.forEach((p, i) => i ? ctx.lineTo(p.x, p.y) : ctx.moveTo(p.x, p.y)); ctx.closePath(); ctx.fillStyle = panel.color; ctx.fill()
  }
  ctx.restore()
  registerLabHit(ctx, interaction, { id: 'citybiz-airship', label: pulse > .03 ? '城市飞艇：尾部喷气加速中，随后恢复巡航' : '城市飞艇：环城巡航中，点击尾部喷气加速', kind: 'drive', x, y, radius: 27 / scale })
  highlightLabHit(ctx, interaction, 'citybiz-airship', x, y, 27 / scale)
}
