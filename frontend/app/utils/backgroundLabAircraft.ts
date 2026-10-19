import type { LabPointer } from '~/utils/backgroundLab'
import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'

/** Velocity boost integrates into distance, so returning to cruise never jumps or reverses. */
export function cityAircraftFlight(w: number, h: number, time: number, boostPhase = 0, boostPosition = 0, boostVelocity = 0) {
  const mobile = w < 600
  const response = Math.min(1, Math.max(Math.max(0, boostPosition) * 1.8, Math.abs(boostVelocity) * .1))
  const leg = (time / 38 + .22 + boostPhase * .032) % 2
  const direction = leg < 1 ? 1 : -1
  const route = leg < 1 ? leg : 2 - leg
  return {
    x: -72 + (w + 144) * route,
    y: h * (mobile ? direction > 0 ? .585 : .62 : direction > 0 ? .19 : .285) + Math.sin(time * .14) * 3 - response * (mobile ? 3 : 5),
    direction, response, scale: mobile ? .48 : .58,
  }
}

export const cityAircraftSmokeColors = ['#dc454b', '#fffdf4', '#287bc7'] as const
interface AircraftSmokeSample { x: number, y: number, t: number, direction: number, strength: number, segment: number }
export interface AircraftSmokeEmitter { samples: AircraftSmokeSample[], lastTime: number | null, segment: number, direction: number, burstStart: number | null, lastClick: number, burstIndex: number }
const smokeEmitters = new WeakMap<LabInteraction, AircraftSmokeEmitter>()
export function createAircraftSmokeEmitter(): AircraftSmokeEmitter { return { samples: [], lastTime: null, segment: 0, direction: 1, burstStart: null, lastClick: 0, burstIndex: -1 } }

/** History stores the actual CSS-space aft outlet; old smoke never mirrors with the airplane. */
export function stepAircraftSmoke(emitter: AircraftSmokeEmitter, flight: ReturnType<typeof cityAircraftFlight>, time: number, clicks = 0) {
  const source = { x: flight.x - flight.direction * 49 * flight.scale, y: flight.y }
  const delta = emitter.lastTime === null ? 0 : time - emitter.lastTime
  const maximum = flight.scale < .5 ? 96 : 160, lifetime = 2.1
  if (delta < 0) { emitter.samples = []; emitter.segment++; emitter.burstStart = null; emitter.burstIndex = -1 }
  if (clicks !== emitter.lastClick) {
    emitter.lastClick = clicks; emitter.burstStart = time; emitter.burstIndex = -1; emitter.segment++
  }
  if (emitter.burstStart === null && flight.response > .04) emitter.burstStart = time
  const elapsed = emitter.burstStart === null ? Infinity : Math.max(0, time - emitter.burstStart)
  const burstIndex = Number.isFinite(elapsed) ? 0 : -1
  const burstOpen = elapsed < .9 && flight.response > .04
  emitter.samples = emitter.samples.filter(sample => time - sample.t >= 0 && time - sample.t < lifetime)
  const last = emitter.samples.at(-1)
  const discontinuity = flight.direction !== (last?.direction ?? emitter.direction) || delta > .25 || (last && Math.hypot(source.x - last.x, source.y - last.y) > 150)
  if (delta > 0 && discontinuity) emitter.segment++
  if (delta > 0 && delta <= .25 && burstOpen && (!last || time - last.t >= 1 / 75)) {
    if (burstIndex !== emitter.burstIndex) { emitter.segment++; emitter.burstIndex = burstIndex }
    if (last && time - last.t > .12 && !discontinuity) emitter.segment++
    emitter.samples.push({ ...source, t: time, direction: flight.direction, strength: flight.response, segment: emitter.segment })
  }
  if (emitter.samples.length > maximum) emitter.samples.splice(0, emitter.samples.length - maximum)
  emitter.lastTime = time; emitter.direction = flight.direction
  const groups = new Map<number, AircraftSmokeSample[]>()
  for (const sample of emitter.samples) {
    const group = groups.get(sample.segment) ?? []
    group.push(sample); groups.set(sample.segment, group)
  }
  const plumes = [...groups.values()].filter(group => group.length > 1).flatMap(group => cityAircraftSmokeColors.map((color, index) => ({
    color, segment: group[0]!.segment,
    emitting: burstOpen && group[0]!.segment === emitter.segment && time - group.at(-1)!.t < .04,
    points: group.map(sample => {
      const age = time - sample.t, u = age / lifetime
      const fan = Math.min(1, age / .12)
      return {
        sourceX: sample.x, sourceY: sample.y, age, direction: sample.direction,
        x: sample.x - sample.direction * age * (flight.scale < .5 ? 70 : 62) * sample.strength,
        y: sample.y + Math.sin(sample.t * .7 + age * .9) * age * 2 + age * age * .8 + (index - 1) * (1.5 + fan * 5.4 + age * 2.2),
        width: (3.8 + sample.strength * 1.2) * (1 + age * .55),
        alpha: Math.min(.84, sample.strength * .9) * (1 - u) ** 1.35,
      }
    }),
  })))
  return { source, maximum, lifetime, burstOpen, burstIndex, samples: emitter.samples.map(sample => ({ ...sample })), plumes }
}

/** Rendering-only arc-length interpolation fills sparse history without emitting extra samples. */
export function resampleAircraftSmoke(points: ReturnType<typeof stepAircraftSmoke>['plumes'][number]['points'], spacing = 2.5, maximum = 320) {
  if (points.length < 2) return points.map(point => ({ ...point }))
  const cumulative = [0]
  for (let i = 1; i < points.length; i++) cumulative.push(cumulative[i - 1]! + Math.hypot(points[i]!.x - points[i - 1]!.x, points[i]!.y - points[i - 1]!.y))
  const length = cumulative.at(-1)!
  if (length < .001) return [{ ...points[0]! }]
  const step = Math.max(spacing, length / (maximum - 1))
  const result: typeof points = []
  let index = 1
  for (let distance = 0; distance < length && result.length < maximum - 1; distance += step) {
    while (index < points.length - 1 && cumulative[index]! < distance) index++
    const a = points[index - 1]!, b = points[index]!
    const u = (distance - cumulative[index - 1]!) / Math.max(.001, cumulative[index]! - cumulative[index - 1]!)
    result.push({ ...a,
      x: a.x + (b.x - a.x) * u, y: a.y + (b.y - a.y) * u,
      sourceX: a.sourceX + (b.sourceX - a.sourceX) * u, sourceY: a.sourceY + (b.sourceY - a.sourceY) * u,
      age: a.age + (b.age - a.age) * u, width: a.width + (b.width - a.width) * u, alpha: a.alpha + (b.alpha - a.alpha) * u,
    })
  }
  result.push({ ...points.at(-1)! })
  return result
}

/** Alive groups share a per-colour draw cap; newest releases keep their original dense texture. */
export function aircraftSmokeDrawPlan(plumes: ReturnType<typeof stepAircraftSmoke>['plumes'], maximum = 320) {
  const remaining = new Map<string, number>()
  const plan: typeof plumes = []
  for (const plume of [...plumes].sort((a, b) => b.segment - a.segment)) {
    const budget = remaining.get(plume.color) ?? maximum
    if (budget < 2) continue
    const points = resampleAircraftSmoke(plume.points, 2.5, budget)
    remaining.set(plume.color, budget - points.length)
    plan.push({ ...plume, points })
  }
  return plan.sort((a, b) => a.segment - b.segment)
}

const smokeSprites = new Map<string, HTMLCanvasElement | null>()
let smokeSpriteFailureReported = false
/** Cached, overlapping translucent lobes form irregular rolling cloud edges, not bead outlines. */
function smokeSprite(color: string) {
  if (smokeSprites.has(color)) return smokeSprites.get(color) ?? null
  if (typeof document === 'undefined') return null
  const canvas = document.createElement('canvas'); canvas.width = 128; canvas.height = 128
  const brush = canvas.getContext('2d')
  if (!brush) {
    smokeSprites.set(color, null)
    if (!smokeSpriteFailureReported) {
      smokeSpriteFailureReported = true
      console.warn('Airshow smoke texture unavailable: the browser could not create a Canvas 2D context.')
    }
    return null
  }
  const hex = color.slice(1), r = parseInt(hex.slice(0, 2), 16), g = parseInt(hex.slice(2, 4), 16), b = parseInt(hex.slice(4, 6), 16)
  for (let lobe = 0; lobe < 9; lobe++) {
    const angle = lobe * 2.39996, distance = lobe ? 8 + lobe % 3 * 4 : 0
    const x = 64 + Math.cos(angle) * distance, y = 64 + Math.sin(angle) * distance
    const radius = 28 + lobe % 4 * 4
    if (color === cityAircraftSmokeColors[1]) {
      const edge = brush.createRadialGradient(x, y, 0, x, y, radius * 1.1)
      edge.addColorStop(0, '#28415000'); edge.addColorStop(.55, '#28415008'); edge.addColorStop(1, '#28415000')
      brush.fillStyle = edge; brush.fillRect(0, 0, 128, 128)
    }
    const cloud = brush.createRadialGradient(x - 3, y - 4, 0, x, y, radius)
    cloud.addColorStop(0, `rgba(${r},${g},${b},.29)`); cloud.addColorStop(.38, `rgba(${r},${g},${b},.22)`); cloud.addColorStop(.73, `rgba(${r},${g},${b},.095)`); cloud.addColorStop(1, `rgba(${r},${g},${b},0)`)
    brush.fillStyle = cloud; brush.fillRect(0, 0, 128, 128)
  }
  smokeSprites.set(color, canvas)
  return canvas
}

function paintAircraftSmoke(ctx: CanvasRenderingContext2D, pose: ReturnType<typeof cityAircraftFlight>, time: number, interaction?: LabInteraction) {
  if (!interaction) return
  let emitter = smokeEmitters.get(interaction)
  if (!emitter) { emitter = createAircraftSmokeEmitter(); smokeEmitters.set(interaction, emitter) }
  const reduced = typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
  const clicks = labTarget(interaction, 'citybiz-airplane', 'drive').clicks
  const smoke = stepAircraftSmoke(emitter, reduced ? { ...pose, response: 0 } : pose, time, clicks)
  ctx.save()
  for (const plume of aircraftSmokeDrawPlan(smoke.plumes)) {
    const sprite = smokeSprite(plume.color)
    if (!sprite) continue
    for (const point of plume.points) {
      // History spacing is dense; irregular soft clouds overlap into a single volumetric plume.
      const variation = .5 + Math.sin(point.sourceX * .055 + point.sourceY * .043) * .5
      const radius = point.width * (.93 + variation * .3)
      const curl = Math.sin(point.age * 1.7 + variation * 6) * point.age * .85
      ctx.save(); ctx.translate(point.x, point.y + curl); ctx.rotate(variation * Math.PI * 2 + point.age * (variation - .5) * .45)
      ctx.globalAlpha = point.alpha * .52
      ctx.drawImage(sprite, -radius * 1.42, -radius, radius * 2.84, radius * 2)
      ctx.restore()
    }
  }
  // Short bright engine jet stays continuous and attached to the true aft outlet.
  if (pose.response > .04 && !reduced) {
    const end = smoke.source.x - pose.direction * (12 + pose.response * 17)
    const glow = ctx.createLinearGradient(smoke.source.x, 0, end, 0)
    glow.addColorStop(0, '#ffffffee'); glow.addColorStop(.3, '#d9f5ffcf'); glow.addColorStop(1, '#80c3df00')
    ctx.fillStyle = glow; ctx.beginPath(); ctx.moveTo(smoke.source.x, smoke.source.y - 2.5 * pose.response)
    ctx.quadraticCurveTo(smoke.source.x - pose.direction * 12, smoke.source.y - 5 * pose.response, end, smoke.source.y)
    ctx.quadraticCurveTo(smoke.source.x - pose.direction * 12, smoke.source.y + 5 * pose.response, smoke.source.x, smoke.source.y + 2.5 * pose.response); ctx.closePath(); ctx.fill()
    ctx.strokeStyle = '#ffffffcc'; ctx.lineWidth = 1.5; ctx.beginPath(); ctx.moveTo(smoke.source.x, smoke.source.y); ctx.lineTo(smoke.source.x - pose.direction * 12, smoke.source.y); ctx.stroke()
  }
  ctx.restore()
}

/** Quiet edge-to-edge patrol; a click adds damped forward thrust and one short airshow smoke release with three rolling coloured plumes. */
export function paintCityAircraft(ctx: CanvasRenderingContext2D, w: number, h: number, time: number, _pointer: LabPointer, interaction?: LabInteraction) {
  const flight = labTarget(interaction, 'citybiz-airplane', 'drive')
  const mobile = w < 600
  const pose = cityAircraftFlight(w, h, time, flight.phase, flight.position, flight.velocity)
  const { x, y, direction, scale } = pose
  paintAircraftSmoke(ctx, pose, time, interaction)
  ctx.save(); ctx.translate(x, y); ctx.scale(direction * scale, scale)
  const poly = (points: number[][], fill: string) => {
    ctx.beginPath(); points.forEach((p, i) => i ? ctx.lineTo(p[0]!, p[1]!) : ctx.moveTo(p[0]!, p[1]!)); ctx.closePath(); ctx.fillStyle = fill; ctx.fill()
  }
  // Rear wing and tail plane establish a tilted, three-dimensional view.
  poly([[-5, -5], [-31, -42], [-20, -44], [24, -3]], '#2e7ba3')
  poly([[-42, -5], [-59, -23], [-48, -25], [-27, -2]], '#86bbd0')
  poly([[-51, -2], [-55, -25], [-47, -27], [-35, 1]], '#dd713e')
  poly([[-41, 5], [-48, 12], [-25, 10], [-15, 4]], '#2e6e99')
  const body = ctx.createLinearGradient(0, -13, 0, 13)
  body.addColorStop(0, '#ffffff'); body.addColorStop(.42, '#f2f5f4'); body.addColorStop(1, '#7f9eaf')
  ctx.fillStyle = body; ctx.beginPath(); ctx.moveTo(-49, -4); ctx.bezierCurveTo(-14, -14, 28, -14, 48, -4); ctx.quadraticCurveTo(61, 3, 47, 10); ctx.bezierCurveTo(18, 17, -18, 11, -49, 3); ctx.closePath(); ctx.fill()
  ctx.strokeStyle = '#254d69'; ctx.lineWidth = .9 / scale; ctx.stroke()
  poly([[-41, 5], [38, 8], [40, 11], [-33, 8]], '#27709f')
  // Cockpit wraps around the nose; a row of windows stays visible behind it.
  poly([[34, -6], [45, -3], [48, 2], [33, 1]], '#153f5c')
  for (let i = 0; i < 6; i++) { ctx.fillStyle = '#285e80'; ctx.beginPath(); ctx.ellipse(23 - i * 8, 2, 2.3, 2.5, 0, 0, Math.PI * 2); ctx.fill() }
  poly([[-9, 4], [-25, 43], [-12, 46], [24, 7]], '#438ab2')
  poly([[-25, 43], [-12, 46], [-10, 50], [-25, 47]], '#225b83')
  poly([[-9, 8], [-4, 19], [11, 20], [17, 10]], '#3877a4')
  ctx.fillStyle = '#1c4560'; ctx.beginPath(); ctx.ellipse(11, 17, 7, 4, -.15, 0, Math.PI * 2); ctx.fill()
  ctx.strokeStyle = '#f2e7c7'; ctx.lineWidth = 1.5; ctx.beginPath(); ctx.moveTo(-19, 40); ctx.lineTo(15, 8); ctx.stroke()
  ctx.restore()
  const boosted = Math.abs(flight.position) + Math.abs(flight.velocity) > .03
  const label = boosted ? '巡航飞机：正在沿航向加速，机尾红白蓝航空烟羽随后自然消散' : '巡航飞机：点击加速，机尾喷气并喷出红白蓝航空烟羽'
  if (x < -28 || x > w + 28) return
  registerLabHit(ctx, interaction, { id: 'citybiz-airplane', label, kind: 'drive', x, y, radius: mobile ? 24 : 28 })
  highlightLabHit(ctx, interaction, 'citybiz-airplane', x, y, mobile ? 24 : 28)
}
