import type { LabPointer } from '~/utils/backgroundLab'
import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'
type Context = CanvasRenderingContext2D
const lerp = (a: number, b: number, p: number) => a + (b - a) * Math.max(0, Math.min(1, p))
function line(ctx: Context, points: number[][], color: string, width = 3) {
  ctx.beginPath(); points.forEach(([x, y], i) => i ? ctx.lineTo(x!, y!) : ctx.moveTo(x!, y!)); ctx.strokeStyle = color; ctx.lineWidth = width; ctx.lineCap = 'round'; ctx.lineJoin = 'round'; ctx.stroke()
}
function ball(ctx: Context, x: number, y: number, r: number, copper = true) {
  const g = ctx.createRadialGradient(x - r * .35, y - r * .45, 0, x, y, r)
  g.addColorStop(0, copper ? '#fff0cf' : '#ffffff'); g.addColorStop(.4, copper ? '#dca879' : '#eeeae0'); g.addColorStop(1, copper ? '#835438' : '#a5ada3')
  ctx.save(); ctx.shadowColor = '#403c3a35'; ctx.shadowBlur = 9; ctx.shadowOffsetY = 5; ctx.fillStyle = g; ctx.beginPath(); ctx.arc(x, y, r, 0, Math.PI * 2); ctx.fill(); ctx.restore()
}
/** A stored phase drives a visible marble run, rocker, bell and return elevator. */
export function paintKineticPlayground(ctx: Context, w: number, h: number, _t: number, pointer: LabPointer, interaction?: LabInteraction) {
  const release = labTarget(interaction, 'toy-release', 'release'), bell = labTarget(interaction, 'toy-bell', 'drive')
  const p = release.phase >= 8 ? 0 : release.phase, mobile = w < 650
  const scale = Math.min(w * (mobile ? .94 : .56) / 620, h * (mobile ? .5 : .79) / 440)
  const r = Math.max(23, 24 / scale), bellSize = Math.max(39, 25 / scale)
  ctx.fillStyle = '#eee7d8'; ctx.fillRect(0, 0, w, h)
  ctx.save(); ctx.translate(w * (mobile ? .5 : .69) + pointer.x * 4, h * (mobile ? .61 : .53) + pointer.y * 3); ctx.scale(scale, scale)
  const slab = ctx.createLinearGradient(0, 128, 0, 183); slab.addColorStop(0, '#e8cda6'); slab.addColorStop(1, '#a97d55')
  ctx.save(); ctx.shadowColor = '#343b3540'; ctx.shadowBlur = 35; ctx.shadowOffsetY = 24; ctx.fillStyle = slab; ctx.beginPath(); ctx.roundRect(-285, 133, 570, 40, 16); ctx.fill(); ctx.restore()
  for (let i = 0; i < 5; i++) line(ctx, [[-265, 145 + i * 4], [265, 145 + i * 4]], '#66432718', 1)
  // Deep frames carry three visibly separated tiers, with bolted ceramic feet.
  for (const [x, top] of [[-234, -158], [-115, -144], [98, -158], [247, -180]]) {
    line(ctx, [[x!, 133], [x!, top!]], '#9daa99', 15); line(ctx, [[x! - 4, 126], [x! - 4, top!]], '#e6e6d5', 4)
    line(ctx, [[x! - 25, 132], [x! + 25, 132]], '#526957', 8); ball(ctx, x!, 123, 4)
  }
  line(ctx, [[-234, -105], [98, 116]], '#a4af9d', 5); line(ctx, [[98, -100], [-234, 118]], '#a4af9d', 5)
  // A rounded switchback carries the marble down two terraces.
  const route: number[][] = []
  for (let i = 0; i <= 30; i++) route.push([lerp(-230, 75, i / 30), lerp(-170, -155, i / 30)])
  for (let i = 1; i <= 24; i++) { const a = -Math.PI / 2 + i / 24 * Math.PI; route.push([75 + Math.cos(a) * 65, -105 + Math.sin(a) * 50]) }
  for (let i = 1; i <= 24; i++) route.push([lerp(75, -130, i / 24), lerp(-55, -30, i / 24)])
  for (let i = 1; i <= 20; i++) { const a = -Math.PI / 2 - i / 20 * Math.PI; route.push([-130 + Math.cos(a) * 48, 15 + Math.sin(a) * 45]) }
  for (let i = 1; i <= 12; i++) route.push([lerp(-130, -90, i / 12), lerp(60, 75, i / 12)])
  const rail = route.map(([x, y]) => [x!, y! + r])
  line(ctx, rail.map(([x, y]) => [x! + 4, y! + 8]), '#3b413c22', 16)
  line(ctx, rail, '#76543b', 13); line(ctx, rail.map(([x, y]) => [x!, y! - 4]), '#e3bb83', 5)
  line(ctx, route.map(([x, y]) => [x!, y! + r + 15]), '#aa8060', 3)
  for (let i = 6; i < rail.length; i += 11) { const [x, y] = rail[i]!; line(ctx, [[x! - 6, y!], [x! + 6, y! + 14]], '#d2ab7d', 3) }
  // The raised return belt remains behind the copper switchback.
  line(ctx, [[210, 94], [254, 94], [254, -215], [-230, -215], [-230, -170]], '#506954', 10)
  line(ctx, [[254, 94], [254, -215], [-230, -215]], '#b3c2a6', 3)
  for (const [x, y] of [[254, 94], [254, -215], [-230, -215]]) { ball(ctx, x!, y!, 17, false); ball(ctx, x!, y!, 6) }
  // A tilting receiver makes the transfer causality visible before the launch.
  const kick = p > 3.25 && p < 4.15 ? Math.sin((p - 3.25) / .9 * Math.PI) * .34 : 0
  ctx.fillStyle = '#799475'; ctx.beginPath(); ctx.moveTo(-76, 135); ctx.lineTo(-42, 70); ctx.lineTo(-7, 135); ctx.closePath(); ctx.fill()
  ctx.save(); ctx.translate(-42, 88); ctx.rotate(-kick); line(ctx, [[-62, 0], [95, 0]], '#3e5948', 17); line(ctx, [[-60, -6], [93, -6]], '#a6b89b', 4)
  for (const x of [-50, 85]) { ctx.fillStyle = '#f6f0df'; ctx.beginPath(); ctx.ellipse(x, -9, r + 4, 10, 0, 0, Math.PI * 2); ctx.fill() }
  ctx.restore(); ball(ctx, -42, 88, 12)
  const ring = p > 4.65 && p < 6 ? Math.sin((p - 4.65) * 22) * Math.exp(-(p - 4.65) * 3) * .26 : 0
  line(ctx, [[247, -150], [170, -150], [170, -105]], '#526d55', 7)
  ctx.save(); ctx.translate(170, -105); ctx.rotate(bell.position * .3 + ring)
  line(ctx, [[0, 0], [0, 16]], '#94714d', 3)
  const metal = ctx.createLinearGradient(-bellSize, 0, bellSize, 0); metal.addColorStop(0, '#855e3a'); metal.addColorStop(.4, '#f3d19a'); metal.addColorStop(1, '#ac7843')
  ctx.fillStyle = metal; ctx.beginPath(); ctx.moveTo(-bellSize * .35, 13); ctx.bezierCurveTo(-bellSize * .8, 18, -bellSize * .5, 49, -bellSize, 57); ctx.quadraticCurveTo(0, 76, bellSize, 57); ctx.bezierCurveTo(bellSize * .5, 49, bellSize * .8, 18, bellSize * .35, 13); ctx.closePath(); ctx.fill(); ball(ctx, 0, 60, 7)
  registerLabHit(ctx, interaction, { id: 'toy-bell', kind: 'drive', label: '大铜钟：点击敲响', x: 0, y: 40, radius: bellSize }); highlightLabHit(ctx, interaction, 'toy-bell', 0, 40, bellSize); ctx.restore()
  ctx.fillStyle = '#f6f0e1'; ctx.beginPath(); ctx.moveTo(174, 65); ctx.quadraticCurveTo(211, 146, 247, 65); ctx.closePath(); ctx.fill(); line(ctx, [[174, 65], [247, 65]], '#adb6a0', 4)
  let bx = -230, by = -170
  if (p > 0 && p < 3.4) {
    const index = Math.min(route.length - 1.001, (p / 3.4) ** .85 * (route.length - 1)), i = Math.floor(index), q = index - i
    bx = lerp(route[i]![0]!, route[i + 1]![0]!, q); by = lerp(route[i]![1]!, route[i + 1]![1]!, q)
  }
  else if (p >= 3.4 && p < 3.9) { bx = -90; by = 75 + Math.sin((p - 3.4) * Math.PI * 2) * 7 }
  else if (p >= 3.9 && p < 5.15) { const q = (p - 3.9) / 1.25; bx = lerp(-90, 210, q); by = lerp(75, 65, q) - Math.sin(q * Math.PI) * 145 }
  else if (p >= 5.15 && p < 5.65) { bx = lerp(210, 254, (p - 5.15) / .5); by = lerp(65, 94, (p - 5.15) / .5) }
  else if (p >= 5.65 && p < 6.7) { bx = 254; by = lerp(94, -215, (p - 5.65) / 1.05) }
  else if (p >= 6.7 && p < 7.8) { bx = lerp(254, -230, (p - 6.7) / 1.1); by = -215 }
  else if (p >= 7.8) { bx = -230; by = lerp(-215, -170, (p - 7.8) / .2) }
  const hop = p >= 3.4 && p < 4.6 ? Math.sin((p - 3.4) / 1.2 * Math.PI) * 115 : 0
  ball(ctx, 43 + hop * .23, 61 - hop, Math.max(21, 21 / scale), false)
  ball(ctx, bx, by, r)
  registerLabHit(ctx, interaction, { id: 'toy-release', kind: 'release', label: '铜球：点击启动回旋球道', x: bx, y: by, radius: r + 5 }); highlightLabHit(ctx, interaction, 'toy-release', bx, by, r + 5)
  ctx.fillStyle = '#61715a'; ctx.font = '11px ui-monospace, monospace'; ctx.fillText('01 / SWITCHBACK', -258, 199); ctx.fillText('02 / TRANSFER', -68, 199); ctx.fillText('03 / ELEVATOR', 114, 199)
  ctx.restore()
}
