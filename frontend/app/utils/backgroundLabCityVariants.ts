import { sampleBusinessBusRoute, sampleBusinessBusFootprints, businessBusRoadAreas, type BusinessBusSection } from '~/utils/backgroundLabBusinessBus'
import { cityAirshipOrbit, paintCityAirship } from '~/utils/backgroundLabAirship'
import { paintCityAircraft } from '~/utils/backgroundLabAircraft'
import type { LabPointer } from '~/utils/backgroundLab'
import { paintCityFerrisWheel } from '~/utils/backgroundLabFerrisWheel'
import { paintCityCoaster } from '~/utils/backgroundLabCoaster'
import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'


type Context = CanvasRenderingContext2D
const TAU = Math.PI * 2
/** Physical basin dimensions stay in city coordinates; touch size belongs only to the hit region. */
export function businessCityFountainLayout() {
  const x = 6.75, y = -.05, radius = 50
  const groundRadius = Math.hypot(radius / 80, radius * .49 / 40)
  return { x, y, radius, groundRadius }
}
function painter(ctx: Context, w: number, h: number, pointer: LabPointer, worldWidth = 920) {
  const scale = w < 600 ? w * .9 / worldWidth : Math.min(w * .66 / worldWidth, h * .83 / 720)
  const centerX = w < 600 ? w * .5 : worldWidth >= 1080 ? Math.min(w * .69, w - worldWidth * .52 * scale - 18) : w * .69
  ctx.translate(centerX + pointer.x * 6, h * (w < 600 ? .67 : .63) + pointer.y * 4); ctx.scale(scale, scale)
  const p = (x: number, y: number, z = 0) => [(x - y) * 40, (x + y) * 20 - z]
  const poly = (points: number[][], color: string) => {
    ctx.beginPath(); points.forEach((point, index) => index ? ctx.lineTo(point[0]!, point[1]!) : ctx.moveTo(point[0]!, point[1]!)); ctx.closePath(); ctx.fillStyle = color; ctx.fill()
  }
  const line = (points: number[][], color: string, width = 1) => {
    ctx.beginPath(); points.forEach((point, index) => index ? ctx.lineTo(point[0]!, point[1]!) : ctx.moveTo(point[0]!, point[1]!)); ctx.strokeStyle = color; ctx.lineWidth = width; ctx.lineCap = 'round'; ctx.stroke()
  }
  const plane = (x: number, y: number, width: number, depth: number, z: number, color: string) => poly([p(x, y, z), p(x + width, y, z), p(x + width, y + depth, z), p(x, y + depth, z)], color)
  const block = (x: number, y: number, width: number, depth: number, z: number, height: number, warm = false, windows = true) => {
    poly([p(x, y + depth, z), p(x + width, y + depth, z), p(x + width, y + depth, z + height), p(x, y + depth, z + height)], warm ? '#b5896a' : worldWidth >= 1080 ? '#618276' : '#78988d')
    poly([p(x + width, y, z), p(x + width, y + depth, z), p(x + width, y + depth, z + height), p(x + width, y, z + height)], warm ? '#d0ad8d' : worldWidth >= 1080 ? '#a4bcae' : '#b1c3b6')
    plane(x, y, width, depth, z + height, warm ? '#e8d7ba' : '#dfe6d8')
    if (windows) for (let floor = 13; floor < height - 7; floor += 17) {
      for (let column = .15; column < width - .1; column += .29) poly([p(x + column, y + depth + .005, z + floor), p(x + column + .14, y + depth + .005, z + floor), p(x + column + .14, y + depth + .005, z + floor + 7), p(x + column, y + depth + .005, z + floor + 7)], '#f0d9a5')
      line([p(x + .1, y + depth + .006, z + floor), p(x + width - .1, y + depth + .006, z + floor)], '#cfe0ca88', 2)
      for (let column = .14; column < depth - .1; column += .3) poly([p(x + width + .005, y + column, z + floor), p(x + width + .005, y + column + .15, z + floor), p(x + width + .005, y + column + .15, z + floor + 7), p(x + width + .005, y + column, z + floor + 7)], '#668d80')
    }
  }
  const vehicle = (x: number, y: number, color: string, large = false) => {
    const length = large ? 1.07 : .62, breadth = large ? .43 : .32
    plane(x + .06, y + .07, length, breadth, 0, '#52705b23')
    block(x, y, length, breadth, 3, large ? 16 : 11, false, false)
    poly([p(x, y + breadth, 4), p(x + length, y + breadth, 4), p(x + length, y + breadth, large ? 19 : 14), p(x, y + breadth, large ? 19 : 14)], color)
    plane(x + .09, y + .04, length - .18, breadth - .08, large ? 19 : 14, color)
    block(x + length * .2, y + .02, length * .55, breadth - .04, large ? 19 : 14, large ? 9 : 6, false, false)
    poly([p(x + length * .23, y + breadth - .01, 17), p(x + length * .73, y + breadth - .01, 17), p(x + length * .73, y + breadth - .01, large ? 26 : 19), p(x + length * .23, y + breadth - .01, large ? 26 : 19)], '#567b75')
    poly([p(x + length * .77, y + .025, large ? 19 : 14), p(x + length * .77, y + breadth - .02, large ? 19 : 14), p(x + length * .77, y + breadth - .02, large ? 27 : 20), p(x + length * .77, y + .025, large ? 27 : 20)], '#7ea19a')
    for (const wheel of [.16, .8]) {
      const center = p(x + length * wheel, y + breadth + .01, 4)
      ctx.fillStyle = '#435d50'; ctx.beginPath(); ctx.ellipse(center[0]!, center[1]!, 4, 5, -.4, 0, TAU); ctx.fill()
      ctx.fillStyle = '#acb8a1'; ctx.beginPath(); ctx.arc(center[0]!, center[1]!, 1.8, 0, TAU); ctx.fill()
    }
  }
  const tree = (x: number, y: number, t: number) => {
    const center = p(x, y), shift = Math.sin(t * .7 + x + y) * 1.5
    line([center, [center[0]!, center[1]! - 17]], '#8c7b5f', 2)
    ctx.fillStyle = '#74966b'; ctx.beginPath(); ctx.ellipse(center[0]! + shift, center[1]! - 24, 11, 15, -.2, 0, TAU); ctx.fill()
    ctx.fillStyle = '#a5ba86'; ctx.beginPath(); ctx.ellipse(center[0]! - 3 + shift, center[1]! - 28, 7, 10, -.2, 0, TAU); ctx.fill()
  }
  return { scale, p, poly, line, plane, block, vehicle, tree }
}

function helicopter(ctx: Context, tools: ReturnType<typeof painter>, x: number, y: number, roof: number, t: number, interaction: LabInteraction | undefined, id: string) {
  const { p, plane, line, poly, scale } = tools
  plane(x - .82, y - .82, 1.64, 1.64, roof + 1, '#8fa48e')
  const pad = p(x, y, roof + 2)
  ctx.save(); ctx.translate(pad[0]!, pad[1]!); ctx.scale(1, .5)
  ctx.strokeStyle = '#f7efd7'; ctx.lineWidth = 3; ctx.beginPath(); ctx.arc(0, 0, 38, 0, TAU); ctx.stroke()
  line([[-11, -18], [-11, 18]], '#f5ebd4', 5); line([[11, -18], [11, 18]], '#f5ebd4', 5); line([[-11, 0], [11, 0]], '#f5ebd4', 5); ctx.restore()
  const state = labTarget(interaction, id, 'toggle')
  const lift = state.position
  const orbit = t * .5
  const cx = x + Math.sin(orbit) * lift * .95, cy = y + Math.cos(orbit) * lift * .65
  const base = p(cx, cy, roof + 24 + lift * 115)
  const shadow = p(cx, cy, roof + 3)
  ctx.fillStyle = `rgba(46,68,53,${.17 - lift * .08})`; ctx.beginPath(); ctx.ellipse(shadow[0]!, shadow[1]!, 29 + lift * 13, 11 + lift * 5, 0, 0, TAU); ctx.fill()
  const size = Math.max(1, 43 / (scale * 84))
  ctx.save(); ctx.translate(base[0]!, base[1]!); ctx.scale(size, size)
  // Tail boom, vertical fin, two skids, a volumetric fuselage and a glazed cockpit.
  poly([[-16, -6], [-70, -18], [-71, -10], [-15, 5]], '#b18b66')
  poly([[-70, -17], [-71, -39], [-58, -24], [-58, -13]], '#d7c4a1')
  line([[-28, 17], [30, 17]], '#617b69', 3); line([[-21, 11], [-23, 18]], '#9a896b', 2); line([[17, 10], [22, 18]], '#9a896b', 2)
  const body = ctx.createLinearGradient(-24, -20, 28, 16); body.addColorStop(0, id === 'citybiz-helicopter' ? '#f5cf69' : '#eee4cb'); body.addColorStop(.5, id === 'citybiz-helicopter' ? '#dc9638' : '#c7af88'); body.addColorStop(1, id === 'citybiz-helicopter' ? '#ad6632' : '#8d9c86')
  ctx.fillStyle = body; ctx.beginPath(); ctx.ellipse(0, -3, 33, 18, -.12, 0, TAU); ctx.fill()
  poly([[9, -20], [25, -17], [34, -7], [28, 4], [10, 1]], id === 'citybiz-helicopter' ? '#244e61' : '#6d9590')
  line([[11, -18], [12, 0], [29, 3]], '#e0ddc5', 1.4)
  line([[-3, -16], [-3, -30]], '#718777', 3)
  ctx.save(); ctx.translate(-3, -30); ctx.rotate(t * (1.5 + lift * 18) + state.phase)
  line([[-49, 0], [49, 0]], '#526c5d', 4); line([[0, -25], [0, 25]], '#8f9f85', 3); ctx.restore()
  ctx.save(); ctx.translate(-68, -21); ctx.rotate(t * (2 + lift * 24)); line([[-8, 0], [8, 0]], '#667d69', 2); ctx.restore()
  ctx.restore()
  registerLabHit(ctx, interaction, { id, label: '停机坪直升机：点击起飞绕行或降落', kind: 'toggle', x: base[0]!, y: base[1]!, radius: 49 * size })
  highlightLabHit(ctx, interaction, id, base[0]!, base[1]!, 49 * size)
}

function containsPoint(point: number[], polygon: number[][]) {
  let inside = false
  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    const a = polygon[i]!, b = polygon[j]!
    if ((a[1]! > point[1]!) !== (b[1]! > point[1]!) && point[0]! < (b[0]! - a[0]!) * (point[1]! - a[1]!) / (b[1]! - a[1]!) + a[0]!) inside = !inside
  }
  return inside
}
function businessBus(ctx: Context, tools: ReturnType<typeof painter>, sections: BusinessBusSection[], visible: boolean, interaction?: LabInteraction) {
  const { p, poly, scale } = tools
  for (const section of sections) {
    const world = (along: number, across: number, z = 0) => p(section.x + Math.cos(section.angle) * along - Math.sin(section.angle) * across, section.y + Math.sin(section.angle) * along + Math.cos(section.angle) * across, z)
    const l = section.length / 2, b = section.width / 2
    poly([world(-l, -b), world(l, -b), world(l, b), world(-l, b)], '#304e4933')
    const faces = [
      [world(-l, -b, 3), world(l, -b, 3), world(l, -b, 24), world(-l, -b, 24)],
      [world(-l, b, 3), world(l, b, 3), world(l, b, 24), world(-l, b, 24)],
      [world(-l, -b, 3), world(-l, b, 3), world(-l, b, 24), world(-l, -b, 24)],
      [world(l, -b, 3), world(l, b, 3), world(l, b, 24), world(l, -b, 24)],
    ]
    faces.forEach((face, index) => poly(face, index % 2 ? '#e3ebe1' : '#b7d0cf'))
    for (const side of [-1, 1]) {
      poly([world(-l, side * b, 7), world(l, side * b, 7), world(l, side * b, 13), world(-l, side * b, 13)], '#377a99')
      for (let pane = 0; pane < 3; pane++) {
        const start = -l + .08 + pane * .32
        poly([world(start, side * (b + .002), 15), world(start + .25, side * (b + .002), 15), world(start + .25, side * (b + .002), 22), world(start, side * (b + .002), 22)], '#28536a')
      }
      for (const wheel of [-.34, .34]) {
        const center = world(wheel, side * (b + .02), 4)
        ctx.fillStyle = '#294b43'; ctx.beginPath(); ctx.ellipse(center[0]!, center[1]!, 3.2, 4.2, .4, 0, TAU); ctx.fill()
      }
    }
    poly([world(-l, -b, 25), world(l, -b, 25), world(l, b, 25), world(-l, b, 25)], '#f4eee0')
    poly([world(l + .002, -b * .84, 15), world(l + .002, b * .84, 15), world(l + .002, b * .84, 23), world(l + .002, -b * .84, 23)], '#355c70')
  }
  const front = sections[0]!
  if (visible) {
    const center = p(front.x, front.y, 14)
    registerLabHit(ctx, interaction, { id: 'citybiz-shuttle', label: '蓝白铰接公交：点击发车，16秒沿城内街道环行回站', kind: 'release', duration: 16, x: center[0]!, y: center[1]!, radius: Math.max(45, 22 / scale) })
    highlightLabHit(ctx, interaction, 'citybiz-shuttle', center[0]!, center[1]!, Math.max(45, 22 / scale))
  }
}

export function paintBusinessCity(ctx: Context, w: number, h: number, t: number, pointer: LabPointer, interaction?: LabInteraction, options?: { aircraft?: boolean }) {
  ctx.save(); const tools = painter(ctx, w, h, pointer, 1240)
  const { p, line, plane, block, tree, vehicle } = tools
  plane(-7.8, -7.2, 15.6, 14.4, -12, '#c3cebc'); plane(-7.8, -7.2, 15.6, 14.4, -3, '#d6dfcd')
  plane(2.82, 2.35, 4.5, 3.92, -1, '#cad8b9')
  for (const lane of [-5.2, -2.6, 0, 2.6, 5.2]) {
    const verticalLength = lane === 5.2 ? 8.45 : 12.6
    const halfWalk = lane === 2.6 ? .2 : .43, halfRoad = lane === 2.6 ? .18 : .3
    plane(lane - halfWalk, -6.3, halfWalk * 2, verticalLength, -.5, '#ece9d8')
    plane(lane - halfRoad, -6.3, halfRoad * 2, verticalLength, 0, '#8fa79a')
    const horizontalLength = lane === 2.6 || lane === 5.2 ? 9.3 : 13.4
    plane(-6.7, lane - .43, horizontalLength, .86, -.5, '#ece9d8')
    plane(-6.7, lane - .3, horizontalLength, .6, 0, '#8fa79a')
    for (let mark = -5; mark < 5; mark++) {
      if (lane !== 5.2 || mark < 2) plane(lane - .012, mark + .1, .024, .28, 1, '#eee7c9')
      if ((lane !== 2.6 && lane !== 5.2) || mark < 2) plane(mark + .1, lane - .012, .28, .024, 1, '#eee7c9')
    }
  }
  // The eastern street turns along the leisure park's north edge, never under its track.
  plane(2.6, 1.66, 4.1, .68, -.5, '#ece9d8'); plane(2.6, 1.76, 4.1, .48, 0, '#8fa79a')
  // Connected southern station loop returns the articulated vehicle facing its departure direction.
  plane(-3.03, 5.2, .86, 1.88, -.5, '#ece9d8'); plane(-2.9, 5.2, .6, 1.88, 0, '#8fa79a')
  plane(-2.6, 6.27, 2.66, .86, -.5, '#ece9d8'); plane(-2.6, 6.4, 2.66, .6, 0, '#8fa79a')
  // The existing central boulevard continues to the grounded entrance stop.
  plane(-.43, 6.3, .86, .72, -.5, '#ece9d8')
  plane(-.3, 6.3, .6, .72, 0, '#8fa79a')
  // The enlarged footprint contains a western waterfront and an eastern civic promenade.
  plane(-7.65, -7.05, 1.9, 14.1, -1, '#a9c8bd')
  plane(-5.75, -7.05, .22, 14.1, 0, '#eee4ce')
  for (let wave = 0; wave < 12; wave++) {
    const y = -5.8 + wave
    line([p(-6.53, y, 0), p(-6.12 + Math.sin(t * .5 + wave) * .04, y, 0)], '#e6efda99', 1)
  }
  // Three overlapping wave bands make the wider river read as moving water.
  for (let band = 0; band < 3; band++) for (let ripple = 0; ripple < 7; ripple++) {
    const y = -6.7 + ripple * 1.95 + ((t * .12 + band * .31) % .8)
    const start = p(-7.5 + band * .28, y, 0), middle = p(-7.05 + band * .24, y + .18, 0), end = p(-6.55 + band * .22, y, 0)
    ctx.beginPath(); ctx.moveTo(start[0]!, start[1]!); ctx.quadraticCurveTo(middle[0]!, middle[1]! + Math.sin(t * .8 + ripple) * 2, end[0]!, end[1]!)
    ctx.strokeStyle = band % 2 ? '#759f9677' : '#d9eee0aa'; ctx.lineWidth = (band === 1 ? .9 : .7) / tools.scale; ctx.stroke()
  }
  // A broad southern promenade and a separated eastern square occupy the new land.
  plane(-5.45, 6.35, 2.35, .55, 0, '#e9e3ce')
  plane(.55, 6.35, 6.85, .55, 0, '#e9e3ce')
  plane(6.75, -5.4, .72, 10.8, 0, '#c7d7b4')
  // A separate eastern fountain plaza, between parking and the leisure park.
  plane(5.8, -1, 1.9, 1.9, .5, '#e9e3ce')
  for (let i = 0; i < 8; i++) {
    const treeX = -4.9 + i * 1.6
    if (treeX < -4 || treeX > 2.4) tree(treeX, 6.7, t)
  }
  const sun = labTarget(interaction, 'citybiz-sun', 'toggle')
  // Sunlight belongs to this model only: directional ground shadows reveal its change.
  ctx.globalAlpha = .035 + sun.position * .24
  for (const [x, y] of [[-3.9, -3.9], [-1.3, -1.3], [1.3, 1.3], [3.9, 3.9]]) {
    tools.poly([p(x!, y!, 0), p(x! + 2.1, y! + .5, 0), p(x! + 2.9 + sun.position * 1.1, y! + 1.6, 0), p(x! + .6, y! + 1.3, 0)], '#50685a')
  }
  ctx.globalAlpha = 1
  // A moored river launch moves along the new, wider water strip.
  const boat = labTarget(interaction, 'citybiz-boat', 'release')
  const drift = Math.sin(Math.PI * Math.min(8, boat.phase) / 8) ** 2
  const boatY = 4 - drift * 8.9
  const boatX = -6.85 + Math.sin(t * .8) * .045
  const boatCenter = p(boatX, boatY, 15)
  // Every ripple stays inside the water strip; the wake opens behind the launch.
  for (let wake = 0; wake < 5; wake++) {
    const distance = .25 + wake * .25
    const width = .18 + wake * .08
    line([p(boatX - width, boatY + 1.05 + distance, 1), p(boatX, boatY + 1.18 + distance, 1), p(boatX + width, boatY + 1.05 + distance, 1)], `rgba(238,247,226,${.72 - wake * .1})`, (.8 + drift * .7) / tools.scale)
  }
  ctx.save(); ctx.translate(boatCenter[0]!, boatCenter[1]!); ctx.scale(1.2, 1.2); ctx.rotate(Math.sin(t * 1.1) * .035 + drift * Math.sin(boat.phase * 2.8) * .105); ctx.translate(-boatCenter[0]!, -boatCenter[1]! + Math.sin(t * 1.4 + boat.phase * 1.3) * (1 + drift * 3) / tools.scale)

  tools.poly([p(boatX - .53, boatY - .95, 1), p(boatX + .53, boatY - .95, 1), p(boatX + .6, boatY + .6, 1), p(boatX, boatY + 1.17, 1), p(boatX - .6, boatY + .6, 1)], '#286e8c')
  tools.poly([p(boatX - .5, boatY - .92, 10), p(boatX + .5, boatY - .92, 10), p(boatX + .55, boatY + .55, 10), p(boatX, boatY + 1.1, 10), p(boatX - .55, boatY + .55, 10)], '#f4f1e5')
  plane(boatX - .38, boatY - .81, .76, .38, 11, '#bda683')
  block(boatX - .36, boatY - .35, .72, .8, 10, 13, false, false)
  tools.poly([p(boatX - .35, boatY + .46, 16), p(boatX + .35, boatY + .46, 16), p(boatX + .28, boatY + .39, 22), p(boatX - .28, boatY + .39, 22)], '#245d7c')
  tools.poly([p(boatX + .37, boatY - .31, 16), p(boatX + .37, boatY + .38, 16), p(boatX + .3, boatY + .32, 22), p(boatX + .3, boatY - .25, 22)], '#5a9eb4')
  plane(boatX - .39, boatY - .39, .78, .83, 24, '#f6f0df')
  line([p(boatX - .45, boatY + .5, 13), p(boatX, boatY + 1.02, 13), p(boatX + .45, boatY + .5, 13)], '#b8c7b5', 1 / tools.scale)
  ctx.restore()
  registerLabHit(ctx, interaction, { id: 'citybiz-boat', label: boat.active ? '水岸游艇：乘浪漂流中，稍后自动回泊' : '水岸游艇：轻摇停泊中，点击短时乘浪漂流', kind: 'release', x: boatCenter[0]!, y: boatCenter[1]!, radius: Math.max(41, 22 / tools.scale) })
  highlightLabHit(ctx, interaction, 'citybiz-boat', boatCenter[0]!, boatCenter[1]!, Math.max(41, 22 / tools.scale))
  // A short pedestrian bridge has a visible deck, low rails and two shore connections.
  plane(-6.7, 1.95, 1.18, .35, 9, '#d5c5a5')
  line([p(-6.7, 1.95, 16), p(-5.52, 1.95, 16)], '#a48e6c', 2)
  line([p(-6.7, 2.3, 16), p(-5.52, 2.3, 16)], '#a48e6c', 2)
  for (const x of [-6.63, -6.08, -5.59]) line([p(x, 1.95, 9), p(x, 1.95, 16)], '#a48e6c', 1.4)
  // Crossings sit at real junctions, outside the marked vehicle lanes.
  for (const junction of [-2.6, 0, 2.6]) for (let stripe = 0; stripe < 5; stripe++) {
    if (junction < 2.6) {
      plane(junction - .28 + stripe * .12, 5.54, .065, .25, 1, '#f6f0d5')
      plane(5.53, junction - .28 + stripe * .12, .25, .065, 1, '#f6f0d5')
    }
  }
  // Parking has individual bays and a mixture of full-bodied cars and a delivery van.
  plane(5.68, -4.55, .95, 3.5, 0, '#b5bead')
  for (let bay = 0; bay < 5; bay++) {
    const y = -4.4 + bay * .65
    line([p(5.76, y, 1), p(6.56, y, 1)], '#ede8cf', 1)
    if (bay !== 2) vehicle(5.81, y + .12, bay % 2 ? '#bba283' : '#d9dfca', bay === 4)
  }
  // The station sits beside the front entrance to the existing x=0 boulevard.
  plane(1.6, 5.25, .6, 1.75, 1, '#e7dfc7')
  for (const y of [5.4, 6.75]) line([p(2.08, y, 1), p(2.08, y, 25)], '#829484', 2)
  plane(1.6, 5.25, .6, 1.65, 26, '#c9d5bd')
  line([p(2, 5.6, 6), p(2, 6.45, 6)], '#a78a66', 4)
  for (let i = 0; i < 5; i++) {
    const carX = ((t * .35 + i * 2.1) % 10) - 5
    if (i < 3 || carX < 1.8) vehicle(carX, [-5.1, -2.5, .1, 2.7, 5.3][i]!, i % 2 ? '#e8ddc4' : '#9ab3a3')
  }
  const airshipOrbit = cityAirshipOrbit(t, interaction)
  const airshipShadow = p(airshipOrbit.x, airshipOrbit.y, 1)
  ctx.fillStyle = `rgba(55,75,57,${.11 - airshipOrbit.pulse * .035})`
  ctx.beginPath(); ctx.ellipse(airshipShadow[0]!, airshipShadow[1]!, 32 * airshipOrbit.depth, 11 * airshipOrbit.depth, 0, 0, TAU); ctx.fill()
  if (!airshipOrbit.near) {
    const center = p(airshipOrbit.x, airshipOrbit.y, airshipOrbit.z)
    paintCityAirship(ctx, center[0]!, center[1]!, tools.scale, t, airshipOrbit.depth, airshipOrbit.pulse, airshipOrbit.facing, interaction)
  }
  // Compact gardens retreat from the actual articulated-bus sweep, not merely its paint.
  for (const [x, y] of [[-3.9, 1.3], [3.9, -1.3]]) {
    plane(x! - .55, y! - .55, 1.1, 1.1, 1, '#c2d3ad')
    plane(x! - .06, y! - .5, .12, 1, 2, '#ece4ce')
    plane(x! - .5, y! - .06, 1, .12, 2, '#ece4ce')
    tree(x!, y! + .1, t)
    line([p(x! - .4, y! + .38, 6), p(x! - .13, y! + .38, 6)], '#a88b65', 3)
    line([p(x! + .13, y! - .38, 6), p(x! + .4, y! - .38, 6)], '#a88b65', 3)
  }
  plane(2.8, -5, 2.2, 2.2, 1, '#c4d4b2')
  plane(3.73, -4.86, .34, 4.25, 2, '#e8e0c8')
  plane(3.15, 3.15, 1.5, 1.5, 1, '#d1ddc2')
  plane(5.95, 6.1, 1.25, .25, 2, '#e5ddc5')
  const lots = []
  for (const y of [-3.9, -1.3, 1.3, 3.9]) for (const x of [-3.9, -1.3, 1.3, 3.9]) { if ((y !== 3.9 || Math.abs(x) !== 1.3) && !(x === 3.9 && y === -1.3) && !(x === -3.9 && y === 1.3)) lots.push({ x, y }) }
  for (const { x, y } of lots) if (!(x === 3.9 && (Math.abs(y) === 3.9 || y === 1.3))) plane(x - .91, y - .91, 1.82, 1.82, 1, '#e8e8d6')
  plane(-2.16, 3.04, 1.72, 1.72, 1, '#e7e2cc')
  // Draw road aprons after lot paving, before all raised objects: articulated corners stay on asphalt.
  for (const area of businessBusRoadAreas.slice(6)) plane(area.minX, area.minY, area.maxX - area.minX, area.maxY - area.minY, 1.2, '#8fa79a')
  plane(-.3, 6.3, .6, .9, 1.2, '#8fa79a')
  plane(-2.9, 6.3, .6, .9, 1.2, '#8fa79a')
  lots.sort((a, b) => a.x + a.y - b.x - b.y)
  const shuttle = labTarget(interaction, 'citybiz-shuttle', 'release')
  const busRoute = sampleBusinessBusRoute(shuttle.phase)
  const busSections = sampleBusinessBusFootprints(shuttle.phase)
  const busCenter = p(busRoute.x, busRoute.y, 14)
  const busDepth = busRoute.x + busRoute.y
  const busOccluded = lots.some(({ x, y }, i) => {
    if ((x === 3.9 && (Math.abs(y) === 3.9 || y === 1.3)) || x + y <= busDepth) return false
    const height = 57 + (i * 47 % 137), width = Math.min(1.1, i % 4 ? 1.3 : 1.65), depth = Math.min(1.1, i % 3 ? 1.38 : 1.05)
    const left = x - width / 2, back = y - depth / 2
    return containsPoint(busCenter, [p(left, back + depth), p(left + width, back + depth), p(left + width, back + depth, height), p(left, back + depth, height)])
      || containsPoint(busCenter, [p(left + width, back), p(left + width, back + depth), p(left + width, back + depth, height), p(left + width, back, height)])
  })
  const scene = [...lots.map((lot, i) => ({ ...lot, index: i, kind: 'building' })), ...busSections.map((section, i) => ({ x: section.x, y: section.y, index: i, kind: 'bus' })), { x: (busSections[0]!.x + busSections[1]!.x) / 2, y: (busSections[0]!.y + busSections[1]!.y) / 2, index: -3, kind: 'joint' }, { x: -1.3, y: 3.9, index: -2, kind: 'terminal' }].sort((a, b) => a.x + a.y - b.x - b.y)
  for (const item of scene) {
    const { x, y } = item, i = item.index
    if (item.kind === 'bus') { businessBus(ctx, tools, [busSections[i]!], i === 0 && !busOccluded && !shuttle.active, interaction); continue }
    if (item.kind === 'joint') {
      const front = busSections[0]!, rear = busSections[1]!
      line([p(front.x - Math.cos(front.angle) * .55, front.y - Math.sin(front.angle) * .55, 14), p(rear.x + Math.cos(rear.angle) * .55, rear.y + Math.sin(rear.angle) * .55, 14)], '#6b8a87', 6)
      continue
    }
    if (item.kind === 'terminal') {
      block(-2.05, 3.15, 1.5, 1.5, 0, 38, true, false)
      helicopter(ctx, tools, -1.3, 3.9, 39, t, interaction, 'citybiz-helicopter')
      continue
    }
    if (x === 3.9 && y === -3.9) {
      tree(x - .87, y + .6, t); tree(x + .91, y + .7, t)
      const wheelCenter = p(x, y, 0)
      paintCityFerrisWheel(ctx, wheelCenter[0]!, wheelCenter[1]!, tools.scale, t, interaction)
      continue
    }
    if (x === 3.9 && (y === 3.9 || y === 1.3)) continue
    const height = 57 + (i * 47 % 137), width = Math.min(1.1, i % 4 ? 1.3 : 1.65), depth = Math.min(1.1, i % 3 ? 1.38 : 1.05)
    block(x - width / 2, y - depth / 2, width, depth, 0, height, i % 5 === 0)
    if (i % 3 === 0) block(x - width * .35, y - depth * .35, width * .7, depth * .7, height, 28, i % 5 === 0)
    // The sun warms the actual tower faces, confined to architecture, never page text.
    ctx.globalAlpha = .14 + sun.position * .15
    tools.poly([p(x - width / 2, y + depth / 2, 0), p(x + width / 2, y + depth / 2, 0), p(x + width / 2, y + depth / 2, height), p(x - width / 2, y + depth / 2, height)], sun.position > .5 ? '#efb866' : '#7c9faa')
    ctx.globalAlpha = 1
    tree(x - .86, y + .7, t)
  }
  // The wider map has an actual tree-lined outside boundary and warm street lamps.
  for (let i = 0; i < 10; i++) {
    const avenueY = -5.8 + i * 1.22
    if ((avenueY < 2.1 || avenueY > 6.5) && (avenueY < -1.6 || avenueY > 1.2)) tree(6.3, avenueY, t)
    tree(-5.72, avenueY, t)
    if (i % 2 === 0) {
      const foot = p(-5.46, -5.4 + i * 1.22), top = p(-5.46, -5.4 + i * 1.22, 27)
      line([foot, top], '#8c9a85', 1.5); line([top, [top[0]! + 7, top[1]! + 1]], '#e3c99c', 2.5)
    }
  }
  // A second leisure ride occupies the vacated southeast lot and the open edge.
  // Small planting strips and a station approach preserve both 5.2 street axes.
  tree(7.35, 2.7, t); tree(7.35, 6.6, t)
  paintCityCoaster(ctx, 5, 4.5, tools.scale, t, interaction)
  for (let i = 0; i < 11; i++) {
    const travel = (t * .13 + i * .73) % 9.6
    const x = i < 7 ? -4.8 + travel : [-2.2, .55, 3, 5.58][i - 7]!
    const y = i < 7 ? -.41 : i >= 9 ? 1.48 + Math.sin(t * .24 + i) * .08 : 4.75 + Math.sin(t * .24 + i) * .7
    const foot = p(x, y, 1), gait = Math.sin(t * 4 + i) * 1.5
    line([[foot[0]! - 1.5, foot[1]! - 5], [foot[0]! - 2 + gait, foot[1]!]], '#5f7660', 1.2)
    line([[foot[0]! + 1, foot[1]! - 5], [foot[0]! + 2 - gait, foot[1]!]], '#5f7660', 1.2)
    line([[foot[0]!, foot[1]! - 5], [foot[0]!, foot[1]! - 10]], i % 2 ? '#b1926a' : '#6d8d7a', 3)
    ctx.fillStyle = '#d7ba8f'; ctx.beginPath(); ctx.arc(foot[0]!, foot[1]! - 12, 2, 0, TAU); ctx.fill()
  }
  // The basin and every water style occupy their own plaza, with a visible screen gap from the ride.
  const fountain = labTarget(interaction, 'citybiz-fountain', 'release')
  const fountainStyle = (Math.max(1, fountain.clicks) - 1) % 4
  const fountainNames = ['扇形水幕', '交错拱桥', '多束涌泉', '层级水花']
  const fountainPower = fountain.clicks ? 1 + Math.sin(Math.PI * Math.min(8, fountain.phase) / 8) ** 2 * .28 : 0
  const fountainLayout = businessCityFountainLayout()
  const fountainCenter = p(fountainLayout.x, fountainLayout.y, 4)
  const fountainRadius = fountainLayout.radius
  ctx.save(); ctx.translate(fountainCenter[0]!, fountainCenter[1]!)
  const stone = ctx.createLinearGradient(0, -fountainRadius * .5, 0, fountainRadius * .62)
  stone.addColorStop(0, '#eee5d1'); stone.addColorStop(.58, '#c9bfa7'); stone.addColorStop(1, '#aab6a1')
  ctx.fillStyle = stone; ctx.beginPath(); ctx.ellipse(0, 4, fountainRadius, fountainRadius * .49, 0, 0, TAU); ctx.fill()
  ctx.strokeStyle = '#f6efdb'; ctx.lineWidth = 1 / tools.scale
  ctx.beginPath(); ctx.ellipse(0, 0, fountainRadius * .94, fountainRadius * .42, 0, 0, TAU); ctx.stroke()
  const water = ctx.createRadialGradient(-fountainRadius * .2, -5, 0, 0, 0, fountainRadius)
  water.addColorStop(0, '#b5d6c8'); water.addColorStop(.7, '#89b6ad'); water.addColorStop(1, '#739d97')
  ctx.fillStyle = water; ctx.beginPath(); ctx.ellipse(0, -1, fountainRadius * .82, fountainRadius * .34, 0, 0, TAU); ctx.fill()
  const jet = (start: number, end: number, height: number, seed: number) => {
    const y = -fountainRadius * .035
    const draw = () => {
      ctx.beginPath(); ctx.moveTo(start, y)
      ctx.bezierCurveTo(start + (end - start) * .2, -height, start + (end - start) * .72, -height * .86, end, y)
    }
    ctx.strokeStyle = '#4e9da591'; ctx.lineWidth = 1.7 / tools.scale; draw(); ctx.stroke()
    ctx.strokeStyle = '#effff2d9'; ctx.lineWidth = .85 / tools.scale; draw(); ctx.stroke()
    // Each stream lands in one small expanding ripple, with a short curved splash.
    const progress = (t * .65 + seed * .21) % 1
    ctx.strokeStyle = `rgba(232,255,237,${.62 * (1 - progress)})`; ctx.lineWidth = .75 / tools.scale
    ctx.beginPath(); ctx.ellipse(end, y + 1, fountainRadius * (.035 + progress * .1), fountainRadius * (.012 + progress * .035), 0, 0, TAU); ctx.stroke()
    ctx.strokeStyle = '#d6f8e599'; ctx.lineWidth = .8 / tools.scale
    ctx.beginPath(); ctx.moveTo(end - fountainRadius * .06, y); ctx.quadraticCurveTo(end - fountainRadius * .03, y - fountainRadius * .1, end, y); ctx.quadraticCurveTo(end + fountainRadius * .035, y - fountainRadius * .08, end + fountainRadius * .065, y); ctx.stroke()
  }
  if (fountainPower) {
    const r = fountainRadius, breathe = 1 + Math.sin(t * 1.5) * .025
    if (fountainStyle === 0) for (let i = -3; i <= 3; i++) jet(i * r * .025, i * r * .22, r * (1.55 - Math.abs(i) * .17) * fountainPower * breathe, i + 3)
    else if (fountainStyle === 1) for (let i = 0; i < 3; i++) {
      jet(-r * (.69 - i * .12), r * (.48 - i * .11), r * (1.15 + i * .16) * fountainPower, i)
      jet(r * (.69 - i * .12), -r * (.48 - i * .11), r * (1.15 + i * .16) * fountainPower, i + 3)
    }
    else if (fountainStyle === 2) for (let i = -2; i <= 2; i++) jet(i * r * .23, i * r * .23 + r * .09, r * (1.75 - Math.abs(i) * .28) * fountainPower * breathe, i + 2)
    else {
      jet(-r * .035, r * .06, r * 1.9 * fountainPower, 0)
      for (let level = 0; level < 3; level++) for (const side of [-1, 1]) jet(side * r * .03, side * r * (.29 + level * .19), r * (1.45 - level * .34) * fountainPower, level + side + 3)
    }
  }
  ctx.restore()
  registerLabHit(ctx, interaction, { id: 'citybiz-fountain', label: `广场喷泉：${fountain.clicks ? fountainNames[fountainStyle] : '未开启'}，点击切换为${fountainNames[fountain.clicks % 4]}`, kind: 'release', x: fountainCenter[0]!, y: fountainCenter[1]!, radius: Math.max(fountainRadius, 22 / tools.scale) })
  highlightLabHit(ctx, interaction, 'citybiz-fountain', fountainCenter[0]!, fountainCenter[1]!, Math.max(fountainRadius, 22 / tools.scale))
  // A restrained warm change follows only the ground footprint and architecture area.
  ctx.globalAlpha = .065 + sun.position * .14
  tools.poly([p(-7.8, -7.2), p(7.8, -7.2), p(7.8, 7.2), p(-7.8, 7.2)], sun.position > .5 ? '#edbc70' : '#8bafbf')
  ctx.globalAlpha = 1
  if (airshipOrbit.near) {
    const center = p(airshipOrbit.x, airshipOrbit.y, airshipOrbit.z)
    paintCityAirship(ctx, center[0]!, center[1]!, tools.scale, t, airshipOrbit.depth, airshipOrbit.pulse, airshipOrbit.facing, interaction)
  }
  const modelOriginX = (w < 600 ? w * .5 : Math.min(w * .69, w - 1240 * .52 * tools.scale - 18)) + pointer.x * 6
  const modelOriginY = h * (w < 600 ? .67 : .63) + pointer.y * 4
  const sunCenter = [(w * .87 - modelOriginX) / tools.scale, (h * (w < 600 ? .51 : .3) - modelOriginY) / tools.scale]
  const sunRadius = (w < 600 ? 16 : 19) / tools.scale
  ctx.save(); ctx.translate(sunCenter[0]!, sunCenter[1]!)
  for (let ray = 0; ray < 8; ray++) {
    const angle = ray / 8 * TAU
    line([[Math.cos(angle) * sunRadius * 1.18, Math.sin(angle) * sunRadius * 1.18], [Math.cos(angle) * sunRadius * (1.4 + sun.position * .2), Math.sin(angle) * sunRadius * (1.4 + sun.position * .2)]], '#c5a477', 2)
  }
  const sunlight = ctx.createRadialGradient(-sunRadius * .3, -sunRadius * .3, 0, 0, 0, sunRadius)
  sunlight.addColorStop(0, '#fff3bf'); sunlight.addColorStop(1, sun.position > .5 ? '#e5a349' : '#c6b78d')
  ctx.fillStyle = sunlight; ctx.beginPath(); ctx.arc(0, 0, sunRadius, 0, TAU); ctx.fill(); ctx.restore()
  registerLabHit(ctx, interaction, { id: 'citybiz-sun', label: '城市日照：点击切换暖阳与柔光', kind: 'toggle', x: sunCenter[0]!, y: sunCenter[1]!, radius: Math.max(sunRadius * 1.5, 22 / tools.scale) })
  highlightLabHit(ctx, interaction, 'citybiz-sun', sunCenter[0]!, sunCenter[1]!, Math.max(sunRadius * 1.5, 22 / tools.scale))
  ctx.restore()
  if (options?.aircraft !== false) paintCityAircraft(ctx, w, h, t, pointer, interaction)
}

export function paintHarborCity(ctx: Context, w: number, h: number, t: number, pointer: LabPointer, interaction?: LabInteraction) {
  ctx.save(); const tools = painter(ctx, w, h, pointer)
  const { p, poly, line, plane, block, tree, vehicle } = tools
  plane(-5.6, -5.6, 11.2, 11.2, -9, '#c3cebc'); plane(-5.6, -5.6, 11.2, 11.2, -1, '#d8dfcd')
  plane(-1.2, -5.6, 2.5, 11.2, 0, '#9fc3bd')
  plane(-1.45, -5.6, .23, 11.2, 1, '#e8e3ce'); plane(1.31, -5.6, .23, 11.2, 1, '#e8e3ce')
  for (let i = 0; i < 18; i++) {
    const y = -5.3 + i * .6, x = -.85 + i % 3 * .55 + Math.sin(t * .4 + i) * .06
    line([p(x, y, .5), p(x + .26, y, .5)], '#e4efdb99', 1)
  }
  for (const lane of [-4.5, -2, 2.2, 4.8]) plane(-5.5, lane - .24, 11, .48, 1, '#a8b5a7')
  plane(-4.15, -5.5, .55, 11, 1, '#a8b5a7'); plane(3.55, -5.5, .55, 11, 1, '#a8b5a7')
  for (let i = 0; i < 4; i++) vehicle(((t * .28 + i * 2.7) % 10) - 5, i % 2 ? -4.45 : 4.85, i % 2 ? '#c6a17a' : '#e6dfcb')
  const buildings = [
    [-4.9, -3.3, 1.2, 1.3, 134], [-2.9, -3.4, 1.1, 1.4, 196], [1.9, -3.5, 1.25, 1.3, 224], [4.3, -3.4, 1.05, 1.35, 145],
    [-4.9, .1, 1.25, 1.45, 109], [-2.9, -.1, 1.1, 1.6, 163], [1.9, -.1, 1.2, 1.5, 171], [4.35, .2, 1.03, 1.4, 97],
    [-4.9, 3.1, 1.15, 1.2, 84], [4.35, 3.2, 1.05, 1.2, 115],
  ].sort((a, b) => a[0]! + a[1]! - b[0]! - b[1]!)
  for (let i = 0; i < buildings.length; i++) {
    const [x, y, width, depth, height] = buildings[i]!
    plane(x! - .18, y! - .18, width! + .36, depth! + .36, 2, '#e8e7d3')
    block(x!, y!, width!, depth!, 0, height!, i % 4 === 0)
    if (i % 2) block(x! + .15, y! + .15, width! - .3, depth! - .3, height!, 22, false)
    tree(x! - .3, y! + depth!, t)
  }
  // A recognisable two-leaf bascule bridge divides the two skyline silhouettes.
  const bridge = labTarget(interaction, 'harbor-bridge', 'toggle')
  const angle = bridge.position * 1.05
  for (const side of [-1, 1]) {
    const anchor = side < 0 ? -1.4 : 1.5
    block(anchor - .12, 2.35, .24, .75, -4, 21, true, false)
    const reach = 1.44 * Math.cos(angle), elevation = 14 + Math.sin(angle) * 83
    const end = anchor - side * reach
    poly([p(anchor, 2.35, 14), p(anchor, 3.1, 14), p(end, 3.1, elevation), p(end, 2.35, elevation)], '#b8b59d')
    line([p(anchor, 2.35, 20), p(end, 2.35, elevation + 6)], '#ead9b4', 3)
    line([p(anchor, 3.1, 20), p(end, 3.1, elevation + 6)], '#ead9b4', 3)
  }
  const bridgeCenter = p(.05, 2.72, 30 + Math.sin(angle) * 40)
  registerLabHit(ctx, interaction, { id: 'harbor-bridge', label: '港湾开合桥：点击升起或放下桥面', kind: 'toggle', x: bridgeCenter[0]!, y: bridgeCenter[1]!, radius: Math.max(59, 22 / tools.scale) })
  highlightLabHit(ctx, interaction, 'harbor-bridge', bridgeCenter[0]!, bridgeCenter[1]!, Math.max(59, 22 / tools.scale))
  for (let i = 0; i < 9; i++) { tree(-1.7, -5 + i * 1.2, t); tree(1.7, -5 + i * 1.2, t) }
  // A river taxi is another articulated volumetric vehicle, with deck and glazed cabin.
  const boatY = (t * .14 % 9.3) - 4.65
  plane(-.63, boatY, .85, 1.3, 4, '#c9b18a'); block(-.52, boatY + .25, .61, .65, 4, 16, true, false)
  poly([p(-.51, boatY + .9, 8), p(.08, boatY + .9, 8), p(.08, boatY + .9, 17), p(-.51, boatY + .9, 17)], '#6c9790')
  block(-3.15, 3.1, 1.6, 1.6, 0, 32, true, false)
  helicopter(ctx, tools, -2.35, 3.9, 33, t, interaction, 'harbor-helicopter')
  ctx.restore()
}
