import { labTarget, registerLabHit, highlightLabHit, type LabInteraction } from '~/utils/backgroundLabInteraction'

/** An upright miniature wheel; each cabin hangs vertically regardless of the wheel angle. */
export function paintCityFerrisWheel(ctx: CanvasRenderingContext2D, x: number, y: number, scale: number, time: number, interaction?: LabInteraction) {
  const motor = labTarget(interaction, 'citybiz-ferris-wheel', 'drive')
  const radius = Math.max(72, 30 / scale), centerY = -radius - 26
  const angle = time * .08 + motor.phase * .13
  const stroke = (points: number[][], color: string, width: number) => {
    ctx.beginPath(); points.forEach((p, i) => i ? ctx.lineTo(p[0]!, p[1]!) : ctx.moveTo(p[0]!, p[1]!)); ctx.strokeStyle = color; ctx.lineWidth = width; ctx.lineCap = 'round'; ctx.stroke()
  }
  const poly = (points: number[][], fill: string) => {
    ctx.beginPath(); points.forEach((p, i) => i ? ctx.lineTo(p[0]!, p[1]!) : ctx.moveTo(p[0]!, p[1]!)); ctx.closePath(); ctx.fillStyle = fill; ctx.fill()
  }
  ctx.save(); ctx.translate(x, y)
  ctx.fillStyle = '#54705d20'; ctx.beginPath(); ctx.ellipse(8, 12, radius * .92, 16, .1, 0, Math.PI * 2); ctx.fill()
  poly([[-radius * .72, -2], [radius * .62, -2], [radius * .86, 15], [-radius * .49, 15]], '#e8dfc4')
  poly([[-radius * .49, 15], [radius * .86, 15], [radius * .86, 21], [-radius * .49, 21]], '#b1bea4')
  // Rear rim sits behind the support, with links showing the depth of the two wheel faces.
  ctx.strokeStyle = '#8da18c'; ctx.lineWidth = Math.max(3, 1.2 / scale)
  ctx.beginPath(); ctx.ellipse(-8, centerY - 4, radius * .8, radius, 0, 0, Math.PI * 2); ctx.stroke()
  stroke([[-radius * .48, 9], [0, centerY], [radius * .48, 9]], '#6b8b79', Math.max(6, 2.3 / scale))
  stroke([[-radius * .48 + 2, 8], [2, centerY + 2], [radius * .48 + 2, 8]], '#d9dbc1', 1.5 / scale)
  for (let i = 0; i < 12; i++) {
    const a = angle + i / 12 * Math.PI * 2, px = Math.cos(a) * radius * .8, py = centerY + Math.sin(a) * radius
    stroke([[-8, centerY - 4], [px - 8, py - 4], [px, py]], '#a9bfa7', Math.max(1, .6 / scale))
    stroke([[0, centerY], [px, py]], '#c6a675', Math.max(1, .7 / scale))
  }
  ctx.strokeStyle = '#ba9a68'; ctx.lineWidth = Math.max(3, 1.4 / scale)
  ctx.beginPath(); ctx.ellipse(0, centerY, radius * .8, radius, 0, 0, Math.PI * 2); ctx.stroke()
  ctx.strokeStyle = '#f1e6c8'; ctx.lineWidth = .7 / scale
  ctx.beginPath(); ctx.ellipse(-1, centerY - 1, radius * .8, radius, 0, 0, Math.PI * 2); ctx.stroke()
  for (let i = 0; i < 8; i++) {
    const a = angle + i / 8 * Math.PI * 2, px = Math.cos(a) * radius * .8, py = centerY + Math.sin(a) * radius
    const width = Math.max(12, 5.5 / scale), height = width * .86
    stroke([[px, py], [px, py + 6]], '#8b9e85', .7 / scale)
    // Local cabin coordinates are never rotated with the rim, preserving gravity.
    poly([[px - width / 2, py + 5], [px + width / 2, py + 5], [px + width / 2, py + 5 + height], [px - width / 2, py + 5 + height]], i % 2 ? '#b18f69' : '#8cae9b')
    poly([[px + width / 2, py + 5], [px + width / 2 + 3, py + 3], [px + width / 2 + 3, py + height + 3], [px + width / 2, py + height + 5]], '#678a79')
    ctx.fillStyle = '#dbe9d8'; ctx.fillRect(px - width * .32, py + 7, width * .64, height * .44)
    stroke([[px - width * .55, py + 5], [px + width * .55, py + 5]], '#eee2c0', .8 / scale)
  }
  ctx.fillStyle = '#d8bc87'; ctx.beginPath(); ctx.arc(0, centerY, Math.max(7, 3 / scale), 0, Math.PI * 2); ctx.fill()
  ctx.fillStyle = '#6c8e78'; ctx.beginPath(); ctx.arc(0, centerY, Math.max(3, 1.2 / scale), 0, Math.PI * 2); ctx.fill()
  const label = Math.abs(motor.position) + Math.abs(motor.velocity) > .03 ? '摩天轮：短时加速中，随后恢复慢转' : '摩天轮：缓慢转动，点击短时加速'
  registerLabHit(ctx, interaction, { id: 'citybiz-ferris-wheel', label, kind: 'drive', x: 0, y: centerY, radius: Math.max(25, 22 / scale) })
  highlightLabHit(ctx, interaction, 'citybiz-ferris-wheel', 0, centerY, Math.max(25, 22 / scale))
  ctx.restore()
}
