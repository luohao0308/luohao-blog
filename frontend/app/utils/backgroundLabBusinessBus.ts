/** Shared world geometry for the articulated bus and independent collision checks. */
export interface BusinessBusSection { x: number, y: number, angle: number, length: number, width: number, corners: { x: number, y: number }[] }
const waypoints = [{ x: .06, y: 5.0 }, { x: .06, y: 0 }, { x: -5.2, y: 0 }, { x: -5.2, y: -5.2 }, { x: .06, y: -5.2 }, { x: .06, y: 0 }, { x: -2.6, y: 0 }, { x: -2.6, y: 6.65 }, { x: .06, y: 6.65 }, { x: .06, y: 5.0 }]
const route: { x: number, y: number, distance: number }[] = []
function append(x: number, y: number) {
  const previous = route.at(-1)
  route.push({ x, y, distance: (previous?.distance ?? 0) + (previous ? Math.hypot(x - previous.x, y - previous.y) : 0) })
}
append(waypoints[0]!.x, waypoints[0]!.y)
for (let i = 1; i < waypoints.length - 1; i++) {
  const before = waypoints[i - 1]!, at = waypoints[i]!, after = waypoints[i + 1]!
  const incomingLength = Math.hypot(at.x - before.x, at.y - before.y), outgoingLength = Math.hypot(after.x - at.x, after.y - at.y)
  const r = i === waypoints.length - 2 ? .2 : .4
  const start = { x: at.x - (at.x - before.x) / incomingLength * r, y: at.y - (at.y - before.y) / incomingLength * r }
  const end = { x: at.x + (after.x - at.x) / outgoingLength * r, y: at.y + (after.y - at.y) / outgoingLength * r }
  append(start.x, start.y)
  for (let step = 1; step <= 24; step++) {
    const u = step / 24
    append((1 - u) ** 2 * start.x + 2 * (1 - u) * u * at.x + u * u * end.x, (1 - u) ** 2 * start.y + 2 * (1 - u) * u * at.y + u * u * end.y)
  }
}
append(waypoints.at(-1)!.x, waypoints.at(-1)!.y)
function sampleDistance(distance: number) {
  if (distance < 0) return { x: .06, y: 5.0 - distance, angle: -Math.PI / 2 }
  const end = route.at(-1)!
  const d = Math.min(end.distance, distance)
  const index = route.findIndex(point => point.distance >= d)
  const first = route[Math.max(0, index - 1)]!, last = route[Math.max(1, index)]!
  const blend = (d - first.distance) / Math.max(.0001, last.distance - first.distance)
  return { x: first.x + (last.x - first.x) * blend, y: first.y + (last.y - first.y) * blend, angle: Math.atan2(last.y - first.y, last.x - first.x) }
}
export function sampleBusinessBusRoute(phase: number) {
  const progress = Math.max(0, Math.min(1, phase / 16))
  const distance = progress * route.at(-1)!.distance
  return { ...sampleDistance(distance), progress, distance, length: 2.45, width: .42 }
}
export function sampleBusinessBusFootprints(phase: number): BusinessBusSection[] {
  const { distance } = sampleBusinessBusRoute(phase)
  return [0, 1.25].map(offset => {
    const point = sampleDistance(distance - offset), length = 1.1, width = .42
    const dx = Math.cos(point.angle), dy = Math.sin(point.angle)
    const corners = [[-.5, -.5], [.5, -.5], [.5, .5], [-.5, .5]].map(([a, b]) => ({ x: point.x + dx * a! * length - dy * b! * width, y: point.y + dy * a! * length + dx * b! * width }))
    return { ...point, length, width, corners }
  })
}

/** Actual grey road union used by the model, including widened turning aprons. */
export const businessBusRoadAreas = [
  { minX: -.3, maxX: .3, minY: -6.3, maxY: 7.2 },
  { minX: -5.5, maxX: -4.9, minY: -6.3, maxY: 6.3 },
  { minX: -2.9, maxX: -2.3, minY: -6.3, maxY: 7.2 },
  { minX: -6.7, maxX: 6.7, minY: -.3, maxY: .3 },
  { minX: -6.7, maxX: 6.7, minY: -5.5, maxY: -4.9 },
  { minX: -2.6, maxX: .06, minY: 6.4, maxY: 7.0 },
  ...[[.06, 0], [-5.2, 0], [-5.2, -5.2], [.06, -5.2], [-2.6, 0], [-2.6, 6.65], [.06, 6.65]].flatMap(([x, y]) => [
    { minX: x! - .42, maxX: x! + .42, minY: y! - 1.18, maxY: Math.min(7.2, y! + 1.18) },
    { minX: x! - 1.18, maxX: x! + 1.18, minY: y! - .42, maxY: Math.min(7.2, y! + .42) },
  ]),
  ...[[.06, 0], [-5.2, 0], [-5.2, -5.2], [.06, -5.2], [-2.6, 0], [-2.6, 6.65], [.06, 6.65]].map(([x, y]) => ({ minX: x! - .66, maxX: x! + .66, minY: y! - .66, maxY: Math.min(7.2, y! + .66) })),
]
