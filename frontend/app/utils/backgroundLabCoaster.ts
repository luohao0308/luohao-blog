import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'
type Context = CanvasRenderingContext2D
type Point = { x: number, y: number, z: number }
// Control points are expressed in the rotated world footprint, allowing vertically
// separated switchbacks to occupy the same compact city block.
const nodes: Point[] = [
  { x: -1.25, y: -1.35, z: 25 }, { x: -.8, y: -1.6, z: 92 },
  { x: .1, y: -1.5, z: 204 }, { x: 1.1, y: -1.1, z: 181 },
  { x: 1.7, y: -.4, z: 42 }, { x: 1, y: .1, z: 28 },
  { x: .1, y: -.3, z: 91 }, { x: -.7, y: .35, z: 149 },
  { x: .05, y: 1.05, z: 87 }, { x: 1, y: .9, z: 30 },
  { x: 1.5, y: .4, z: 43 }, { x: 1, y: -.2, z: 79 },
  { x: .1, y: .1, z: 111 }, { x: -.9, y: .9, z: 83 },
  { x: -1.65, y: .65, z: 55 }, { x: -1.65, y: -.65, z: 30 },
]
function curve(u: number): Point {
  const v = ((u % 1 + 1) % 1) * nodes.length, i = Math.floor(v), t = v - i
  const a = nodes[(i + nodes.length - 1) % nodes.length]!, b = nodes[i]!, c = nodes[(i + 1) % nodes.length]!, d = nodes[(i + 2) % nodes.length]!
  const axis = (key: keyof Point) => .5 * (2 * b[key] + (-a[key] + c[key]) * t + (2 * a[key] - 5 * b[key] + 4 * c[key] - d[key]) * t * t + (-a[key] + 3 * b[key] - 3 * c[key] + d[key]) * t * t * t)
  // Convert to the renderer's local basis; its existing rotation restores these world coordinates.
  const originalX = axis('x'), originalY = axis('y')
  const spread = (originalX - originalY) * 1.38, depth = (originalX + originalY) * .48
  const x = (depth + spread) * .5 + .06, y = (depth - spread) * .5 - .16
  return { x: (x + y) / Math.SQRT2, y: (y - x) / Math.SQRT2, z: axis('z') * .96 }
}
function line(c: Context, points: number[][], color: string, width: number) {
  c.beginPath(); points.forEach(([x, y], i) => i ? c.lineTo(x!, y!) : c.moveTo(x!, y!)); c.strokeStyle = color; c.lineWidth = width; c.lineJoin = 'round'; c.lineCap = 'round'; c.stroke()
}
/** Shared geometry contract for dispatch timing, track bounds and return-to-station checks. */
export function cityCoasterMotion(phase: number) {
  const p = Math.max(0, Math.min(14, phase))
  const beats = [[0, 0], [3, .115], [4.6, .14], [5.7, .315], [7.7, .44], [8.6, .58], [11.7, .90], [14, 1]]
  const segment = beats.findIndex((beat, i) => i > 0 && p <= beat[0]!)
  const from = beats[Math.max(0, segment - 1)]!, to = beats[Math.max(1, segment)]!
  const q = (p - from[0]!) / (to[0]! - from[0]!)
  // Match the previous segment's entry velocity, then decrease continuously to zero.
  const previous = beats[Math.max(0, segment - 2)]!
  const tangent = (from[1]! - previous[1]!) / (from[0]! - previous[0]!) * (to[0]! - from[0]!) / (to[1]! - from[1]!)
  const progress = segment === beats.length - 1 ? (tangent - 2) * q ** 3 + (3 - 2 * tangent) * q ** 2 + tangent * q : q
  const u = from[1]! + (to[1]! - from[1]!) * progress
  return { u, point: curve(u) }
}
/** World-space footprint after the 45-degree basis rotation, including support feet and rails. */
export function cityCoasterGroundBounds() {
  const points = Array.from({ length: 1001 }, (_, i) => {
    const p = curve(i / 1000)
    return { x: (p.x - p.y) / Math.SQRT2, y: (p.x + p.y) / Math.SQRT2 }
  })
  // .25 world includes support feet, sleeper overhang and the horizontal half-car envelope.
  const padding = .25
  return { minX: Math.min(...points.map(p => p.x)) - padding, maxX: Math.max(...points.map(p => p.x)) + padding, minY: Math.min(...points.map(p => p.y)) - padding, maxY: Math.max(...points.map(p => p.y)) + padding }
}
/** Renderer-space dimensions for reproducible layout checks, before the city's CSS scale. */
export function cityCoasterGeometry() {
  const points = Array.from({ length: 8001 }, (_, i) => {
    const p = curve(i / 8000), x = (p.x - p.y) / Math.SQRT2, y = (p.x + p.y) / Math.SQRT2
    return { x: (x - y) * 40, y: (x + y) * 20 - p.z }
  })
  const width = Math.max(...points.map(p => p.x)) - Math.min(...points.map(p => p.x))
  const height = Math.max(...points.map(p => p.y)) - Math.min(...points.map(p => p.y))
  return { width, height, aspect: height / width, bounds: cityCoasterGroundBounds(), duration: 14 }
}
/** Nonadjacent track on the camera side must cover a distant car at projected crossings. */
export function cityCoasterRailForeground(railU: number, carU: number, occupiedU: number[] = [carU]) {
  const distance = (a: number, b: number) => { const d = Math.abs(((a % 1 + 1) % 1) - ((b % 1 + 1) % 1)); return Math.min(d, 1 - d) }
  if (occupiedU.some(u => distance(railU, u) < .045)) return false
  const rail = curve(railU), car = curve(carU)
  // In this local basis, world x+y equals local x * sqrt(2).
  return rail.x > car.x + .005
}
/** A city-world sculpture with a stored, fourteen-second dispatch cycle. No private event loop. */
export function paintCityCoaster(ctx: Context, cx: number, cy: number, scale: number, _t: number, interaction?: LabInteraction) {
  const run = labTarget(interaction, 'citybiz-coaster', 'release')
  // Rotate the footprint in world space, preserving vertical supports and gravity.
  const project = (p: Point) => {
    const x = (p.x - p.y) / Math.SQRT2, y = (p.x + p.y) / Math.SQRT2
    return [(x - y) * 40, (x + y) * 20 - p.z]
  }
  const screen = (u: number) => project(curve(u))
  // Slow lift hill, brisk drop and broad final braking curve, with continuous phase boundaries.
  const travel = cityCoasterMotion(run.phase).u
  ctx.save(); ctx.translate((cx - cy) * 40, (cx + cy) * 20); /* Caller already applies world scale. */
  // Narrow shadows retain the existing streets beneath the installation.
  const shadow: number[][] = []
  for (let i = 0; i <= 100; i++) { const q = curve(i / 100); shadow.push(project({ ...q, z: 0 })) }
  line(ctx, shadow, '#354b3c13', 13)
  const drawSupport = (u: number) => {
    const q = curve(u), top = project(q), foot = project({ ...q, z: 0 })
    line(ctx, [[foot[0]! - 9, foot[1]!], top, [foot[0]! + 9, foot[1]!]], '#436e5b', Math.max(3.5, 1 / scale))
    line(ctx, [[foot[0]! - 8, foot[1]!], [foot[0]! + 8, foot[1]!]], '#c9c7ad', 5)
    if (q.z > 65) line(ctx, [[foot[0]! - 6, foot[1]! - 24], [top[0]! + 3, top[1]! + 30]], '#718f76', Math.max(1.5, 1 / scale))
  }
  for (let i = 0; i < 28; i++) drawSupport(i / 28)
  // Two offset running rails with perpendicular sleepers following each local tangent.
  const left: number[][] = [], right: number[][] = []
  for (let i = 0; i <= 320; i++) {
    const u = i / 320, a = screen(u), b = screen(u + .001), dx = b[0]! - a[0]!, dy = b[1]! - a[1]!, length = Math.hypot(dx, dy) || 1
    const nx = -dy / length * 6, ny = dx / length * 6
    left.push([a[0]! + nx, a[1]! + ny]); right.push([a[0]! - nx, a[1]! - ny])
    if (i % 4 === 0) line(ctx, [[a[0]! + nx * 1.5, a[1]! + ny * 1.5], [a[0]! - nx * 1.5, a[1]! - ny * 1.5]], '#87684c', 2)
  }
  line(ctx, left, '#a96739', Math.max(3.2, 1 / scale)); line(ctx, right, '#a96739', Math.max(3.2, 1 / scale)); line(ctx, left.map(([x, y]) => [x!, y! - 1]), '#f4d29a', 1)
  // Only the station has a solid, shallow deck. Its real dimensions remain touch-visible.
  const station = screen(0), stationWidth = 58, stationDepth = 10
  ctx.fillStyle = '#9ba8a1'; ctx.beginPath(); ctx.roundRect(station[0]! - stationWidth / 2, station[1]! + 6, stationWidth, stationDepth, 3); ctx.fill()
  ctx.fillStyle = '#d5dcd5'; ctx.beginPath(); ctx.roundRect(station[0]! - stationWidth / 2, station[1]! + 3, stationWidth, stationDepth - 3, 3); ctx.fill()
  const drawCar = (u: number, lead: boolean) => {
    const a = screen(u), b = screen(u + .002), dx = b[0]! - a[0]!, dy = b[1]! - a[1]!, facing = dx < 0 ? -1 : 1
    const pitch = Math.atan2(dy * facing, Math.abs(dx))
    ctx.save(); ctx.translate(a[0]!, a[1]! - 5); ctx.rotate(pitch); ctx.scale(facing * 1.5, 1.5)
    ctx.fillStyle = '#3b5144'; ctx.fillRect(-9, 1, 18, 5)
    for (const x of [-6, 6]) { ctx.fillStyle = '#354038'; ctx.beginPath(); ctx.arc(x, 5, 2.5, 0, Math.PI * 2); ctx.fill() }
    ctx.fillStyle = lead ? '#d86532' : '#e6b044'; ctx.beginPath(); ctx.roundRect(-11, -7, 22, 10, 3); ctx.fill()
    ctx.fillStyle = lead ? '#f3a565' : '#f8d78a'; ctx.beginPath(); ctx.moveTo(-11, -7); ctx.lineTo(-7, -12); ctx.lineTo(12, -12); ctx.lineTo(11, -7); ctx.closePath(); ctx.fill()
    ctx.fillStyle = lead ? '#963f23' : '#aa6b29'; ctx.beginPath(); ctx.moveTo(11, -7); ctx.lineTo(12, -12); ctx.lineTo(14, -3); ctx.lineTo(11, 3); ctx.closePath(); ctx.fill()
    ctx.fillStyle = '#32596b'; ctx.fillRect(-7, -6, 5, 5); ctx.fillRect(3, -6, 5, 5)
    line(ctx, [[-10, -7], [9, -7]], '#f2dab0', 1.5); ctx.restore()
  }
  for (let i = 2; i > 0; i--) line(ctx, [screen(travel - i * .03), screen(travel - (i - 1) * .03)], '#586a55', Math.max(2, 1 / scale))
  for (let i = 2; i >= 0; i--) drawCar(travel - i * .03, i === 0)
  // Restore only camera-side, nonadjacent rails inside each train car's screen region.
  // This resolves track-over-car crossings without repainting its own supporting track.
  const occupied = [travel, travel - .03, travel - .06]
  for (const carU of occupied) {
    const car = screen(carU), half = 31
    ctx.save(); ctx.beginPath(); ctx.rect(car[0]! - half, car[1]! - 34, half * 2, 55); ctx.clip()
    // Camera-side trestles also occlude a distant car, before the corresponding rails.
    for (let i = 0; i < 28; i++) {
      const u = i / 28
      if (cityCoasterRailForeground(u, carU, occupied)) drawSupport(u)
    }
    for (let i = 0; i < 320; i++) {
      const u = (i + .5) / 320
      if (!cityCoasterRailForeground(u, carU, occupied)) continue
      const a = screen(i / 320), b = screen((i + 1) / 320)
      if (Math.max(a[0]!, b[0]!) < car[0]! - half - 10 || Math.min(a[0]!, b[0]!) > car[0]! + half + 10 || Math.max(a[1]!, b[1]!) < car[1]! - 44 || Math.min(a[1]!, b[1]!) > car[1]! + 31) continue
      if (i % 4 === 0) {
        const l = left[i]!, r = right[i]!, cx = a[0]!, cy = a[1]!
        line(ctx, [[cx + (l[0]! - cx) * 1.5, cy + (l[1]! - cy) * 1.5], [cx + (r[0]! - cx) * 1.5, cy + (r[1]! - cy) * 1.5]], '#87684c', 2)
      }
      line(ctx, [left[i]!, left[i + 1]!], '#a96739', Math.max(3.2, 1 / scale))
      line(ctx, [right[i]!, right[i + 1]!], '#a96739', Math.max(3.2, 1 / scale))
      line(ctx, [left[i]!, left[i + 1]!].map(([x, y]) => [x!, y! - 1]), '#f4d29a', 1)
    }
    ctx.restore()
  }
  // The compact station sits beneath the train; no oversized deck or human-like dispatch pole.
  const radius = Math.max(22 / scale, stationWidth * .5)
  const label = run.active ? '过山车车程中：点击重新发车' : '过山车等待发车：点击站台，爬坡后俯冲回站'
  registerLabHit(ctx, interaction, { id: 'citybiz-coaster', kind: 'release', duration: 14, label, x: station[0]!, y: station[1]!, radius })
  highlightLabHit(ctx, interaction, 'citybiz-coaster', station[0]!, station[1]!, radius)
  ctx.restore()
}
