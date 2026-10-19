import type { LabPointer } from '~/utils/backgroundLab'
import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'
type C = CanvasRenderingContext2D
const tau = Math.PI * 2
const mix = (a: number, b: number, q: number) => a + (b - a) * Math.max(0, Math.min(1, q))
function stroke(c: C, points: number[][], color: string, width: number) {
  c.beginPath(); points.forEach(([x, y], i) => i ? c.lineTo(x!, y!) : c.moveTo(x!, y!)); c.strokeStyle = color; c.lineWidth = width; c.lineCap = 'round'; c.lineJoin = 'round'; c.stroke()
}
function sphere(c: C, x: number, y: number, r: number, white = false) {
  const g = c.createRadialGradient(x - r * .3, y - r * .4, 0, x, y, r); g.addColorStop(0, white ? '#fffef6' : '#ffe9b3'); g.addColorStop(.55, white ? '#e2e7d9' : '#c98d50'); g.addColorStop(1, white ? '#89998b' : '#805232')
  c.save(); c.shadowColor = '#31403335'; c.shadowOffsetY = 7; c.shadowBlur = 10; c.fillStyle = g; c.beginPath(); c.arc(x, y, r, 0, tau); c.fill(); c.restore()
}
function setup(c: C, w: number, h: number, pointer: LabPointer) {
  const mobile = w < 650, s = Math.min(w * (mobile ? .92 : .55) / 620, h * (mobile ? .49 : .79) / 470)
  c.fillStyle = '#eee7d8'; c.fillRect(0, 0, w, h); c.save(); c.translate(w * (mobile ? .5 : .69) + pointer.x * 3, h * (mobile ? .62 : .53) + pointer.y * 3); c.scale(s, s)
  return s
}
function target(c: C, interaction: LabInteraction | undefined, id: string, kind: 'release' | 'toggle', label: string, x: number, y: number, radius: number) {
  registerLabHit(c, interaction, { id, kind, label, x, y, radius }); highlightLabHit(c, interaction, id, x, y, radius)
}
function base(c: C, x: number, width: number) {
  c.save(); c.shadowColor = '#34432c30'; c.shadowBlur = 25; c.shadowOffsetY = 16; c.fillStyle = '#bd9771'; c.beginPath(); c.roundRect(x, 161, width, 33, 12); c.fill(); c.restore()
  stroke(c, [[x + 12, 164], [x + width - 12, 164]], '#efd1a6', 5)
}
/** A vertically stacked marble tower; a click releases a ball into alternating gravity shelves. */
export function paintMarbleTower(c: C, w: number, h: number, _t: number, pointer: LabPointer, interaction?: LabInteraction) {
  const s = setup(c, w, h, pointer), r = Math.max(24, 23 / s)
  const release = labTarget(interaction, 'track-tower-release', 'release'), gate = labTarget(interaction, 'track-tower-gate', 'toggle')
  const p = release.phase >= 8 ? 0 : release.phase, open = gate.active
  base(c, -225, 450)
  for (const x of [-160, 160]) { stroke(c, [[x, 160], [x, -200]], '#8b9f8a', 17); stroke(c, [[x - 4, 152], [x - 4, -200]], '#dee4d2', 4) }
  // Tall central ceramic arch and distinct descending landings.
  c.strokeStyle = '#c0cbb7'; c.lineWidth = 15; c.beginPath(); c.moveTo(-160, -175); c.bezierCurveTo(-160, -272, 160, -272, 160, -175); c.stroke()
  const shelves = [[-130, -150, 118, -125], [118, -40, -115, -12], [-115, 74, 106, 95]]
  for (const [x1, y1, x2, y2] of shelves) {
    stroke(c, [[x1!, y1! + r], [x2!, y2! + r]], '#77563b', 13); stroke(c, [[x1!, y1! + r - 4], [x2!, y2! + r - 4]], '#e1b37c', 4)
    for (const x of [-150, 150]) sphere(c, x, y1! + r + 4, 6)
  }
  // Oversized ceramic diverter is independently controllable.
  c.save(); c.translate(0, 7); c.rotate(open ? -.3 : .3); stroke(c, [[-48, 0], [48, 0]], '#526f55', 18); stroke(c, [[-45, -5], [45, -5]], '#c7d0b6', 4); c.restore(); sphere(c, 0, 7, 11, true)
  target(c, interaction, 'track-tower-gate', 'toggle', '分流挡板：切换落球出口', 0, 7, Math.max(32, 24 / s))
  let x = -130, y = -150
  if (p > 0 && p < 1.65) { x = mix(-130, 118, p / 1.65); y = mix(-150, -125, p / 1.65) }
  else if (p >= 1.65 && p < 2.2) { x = 118; y = mix(-125, -40, ((p - 1.65) / .55) ** 2) }
  else if (p >= 2.2 && p < 3.6) { const q = (p - 2.2) / 1.4; x = mix(118, -115, q); y = mix(-40, -12, q) - (open ? Math.sin(q * Math.PI) * 28 : 0) }
  else if (p >= 3.6 && p < 4.2) { x = -115; y = mix(-12, 74, ((p - 3.6) / .6) ** 2) }
  else if (p >= 4.2 && p < 5.7) { x = mix(-115, open ? 0 : 106, (p - 4.2) / 1.5); y = mix(74, 95, (p - 4.2) / 1.5) }
  else if (p >= 5.7 && p < 6.4) { x = open ? 0 : 106; y = mix(95, 130, (p - 5.7) / .7) }
  else if (p >= 6.4) { x = mix(open ? 0 : 106, -130, (p - 6.4) / 1.6); y = mix(130, -150, (p - 6.4) / 1.6) }
  stroke(c, [[106, 143], [-202, 143], [-202, -150], [-130, -150]], '#9eac96', 7)
  for (const cx of [0, 106]) { c.fillStyle = '#f6f1df'; c.beginPath(); c.ellipse(cx, 143, 43, 14, 0, 0, Math.PI); c.fill() }
  sphere(c, x, y, r); target(c, interaction, 'track-tower-release', 'release', '落球塔：点击铜球放行', x, y, r + 4)
  c.fillStyle = '#52654f'; c.font = '12px sans-serif'; c.fillText(open ? '挡板向左 · 中央收集杯' : '挡板向右 · 外侧收集杯', -96, 224); c.restore()
}
/** A wide railway-like switchyard: choose a branch, then launch the marble onto a rocker relay. */
export function paintSwitchyard(c: C, w: number, h: number, _t: number, pointer: LabPointer, interaction?: LabInteraction) {
  const s = setup(c, w, h, pointer), r = Math.max(24, 23 / s)
  const release = labTarget(interaction, 'track-yard-release', 'release'), switcher = labTarget(interaction, 'track-yard-switch', 'toggle')
  const p = release.phase >= 8 ? 0 : release.phase, upper = switcher.active
  base(c, -285, 570)
  for (const x of [-235, -70, 95, 245]) { stroke(c, [[x, 160], [x, -100]], '#a0af96', 11); sphere(c, x, 146, 6) }
  const rail = (points: number[][]) => { stroke(c, points, '#76553e', 14); stroke(c, points.map(([x, y]) => [x!, y! - 4]), '#edc58d', 4) }
  rail([[-240, -80], [-90, -65], [80, -150], [238, -80]])
  rail([[-90, -65], [80, 25], [238, -80]])
  rail([[238, -80], [265, -20], [205, 110], [-240, 110]])
  // The lever visibly rotates towards the selected rail, rather than changing the entire scene.
  const angle = upper ? -.45 : .45
  c.save(); c.translate(-90, -65); c.rotate(angle); stroke(c, [[0, 0], [70, 0]], '#415e49', 19); sphere(c, 62, 0, Math.max(20, 22 / s), true); c.restore()
  target(c, interaction, 'track-yard-switch', 'toggle', '道岔：点击切换上层 / 下层轨道', -90 + Math.cos(angle) * 62, -65 + Math.sin(angle) * 62, Math.max(26, 24 / s))
  let x = -240, y = -80
  if (p > 0 && p < 1.5) { x = mix(-240, -90, p / 1.5); y = mix(-80, -65, p / 1.5) }
  else if (p >= 1.5 && p < 3.1) { x = mix(-90, 80, (p - 1.5) / 1.6); y = mix(-65, upper ? -150 : 25, (p - 1.5) / 1.6) }
  else if (p >= 3.1 && p < 4.4) { x = mix(80, 238, (p - 3.1) / 1.3); y = mix(upper ? -150 : 25, -80, (p - 3.1) / 1.3) }
  else if (p >= 4.4 && p < 5) { x = mix(238, 205, (p - 4.4) / .6); y = mix(-80, 88, (p - 4.4) / .6) }
  else if (p >= 5 && p < 7.2) { x = mix(205, -240, (p - 5) / 2.2); y = 88 - Math.sin((p - 5) / 2.2 * Math.PI) * 12 }
  else if (p >= 7.2) { x = -240; y = mix(88, -80, (p - 7.2) / .8) }
  for (let i = 0; i < 3; i++) {
    const cx = 115 - i * 135, trigger = 5.35 + i * .6
    const bounce = p > trigger && p < trigger + .8 ? Math.sin((p - trigger) / .8 * Math.PI) : 0
    c.fillStyle = '#768c70'; c.beginPath(); c.moveTo(cx - 18, 155); c.lineTo(cx, 112); c.lineTo(cx + 18, 155); c.closePath(); c.fill()
    c.save(); c.translate(cx, 113); c.rotate(bounce * -.35); stroke(c, [[-49, 0], [49, 0]], '#617e5d', 12); c.restore()
    sphere(c, cx - 33, 85 - bounce * 65, Math.max(19, 19 / s), true)
  }
  stroke(c, [[-263, 110], [-263, -80]], '#647c60', 8)
  sphere(c, x, y, r); target(c, interaction, 'track-yard-release', 'release', '接力球：点击放行，沿所选路线接力', x, y, r + 4)
  c.fillStyle = '#52654f'; c.font = '12px sans-serif'; c.fillText(upper ? '上层快线 → 回程跷跷板接力' : '下层缓线 → 回程跷跷板接力', -110, 224); c.restore()
}
