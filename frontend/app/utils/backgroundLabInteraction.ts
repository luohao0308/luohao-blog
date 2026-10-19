export type LabHitKind = 'pendulum' | 'drive' | 'toggle' | 'release'
export interface LabTargetState { kind: LabHitKind, position: number, velocity: number, phase: number, active: boolean, clicks: number, duration?: number }
export interface LabHitRegion { id: string, label: string, kind: LabHitKind, x: number, y: number, radius: number, anchorX?: number, anchorY?: number, ropeLength?: number, duration?: number }
export interface LabInteraction {
  ratio: number
  hits: LabHitRegion[]
  states: Record<string, LabTargetState>
  hovered: string | null
  dragging: string | null
  actions: number
  lastHit: string
}
export function createLabInteraction(): LabInteraction { return { ratio: 1, hits: [], states: {}, hovered: null, dragging: null, actions: 0, lastHit: '' } }
export function labTarget(interaction: LabInteraction | undefined, id: string, kind: LabHitKind): LabTargetState {
  const make = () => ({ kind, position: 0, velocity: 0, phase: 0, active: false, clicks: 0 })
  if (!interaction) return make()
  const state = interaction.states[id]
  if (!state || state.kind !== kind) return interaction.states[id] = make()
  return state
}
/** Convert the artist's local coordinates into canvas CSS pixels, including the current DPR/view transform. */
export function registerLabHit(ctx: CanvasRenderingContext2D, interaction: LabInteraction | undefined, local: LabHitRegion) {
  if (!interaction) return
  const m = ctx.getTransform(), ratio = interaction.ratio
  const point = (x: number, y: number) => ({ x: (m.a * x + m.c * y + m.e) / ratio, y: (m.b * x + m.d * y + m.f) / ratio })
  const p = point(local.x, local.y), scale = Math.hypot(m.a, m.b) / ratio
  const anchor = local.anchorX !== undefined && local.anchorY !== undefined ? point(local.anchorX, local.anchorY) : undefined
  interaction.hits.push({ ...local, ...p, radius: Math.max(22, local.radius * scale), anchorX: anchor?.x, anchorY: anchor?.y, ropeLength: local.ropeLength === undefined ? undefined : local.ropeLength * scale })
}
export function highlightLabHit(ctx: CanvasRenderingContext2D, interaction: LabInteraction | undefined, id: string, x: number, y: number, radius: number) {
  if (interaction?.hovered !== id && interaction?.dragging !== id) return
  ctx.save(); ctx.strokeStyle = '#b97d3a'; ctx.lineWidth = 2; ctx.setLineDash([3, 3]); ctx.beginPath(); ctx.arc(x, y, radius + 5, 0, Math.PI * 2); ctx.stroke(); ctx.restore()
}
export function hitLabRegion(interaction: LabInteraction, x: number, y: number) {
  return [...interaction.hits].reverse().find(hit => Math.hypot(x - hit.x, y - hit.y) <= hit.radius)
}
export function activateLabHit(interaction: LabInteraction, hit: LabHitRegion, reduced = false) {
  const state = labTarget(interaction, hit.id, hit.kind)
  state.clicks++; interaction.actions++; interaction.lastHit = hit.id
  if (hit.kind === 'pendulum') {
    if (reduced) { state.position = state.position ? 0 : .45; state.velocity = 0 }
    else state.velocity = Math.min(5, state.velocity + 2.6)
  }
  else if (hit.kind === 'drive') {
    if (reduced) state.phase += Math.PI / 3
    else state.velocity = Math.min(8, state.velocity + 5.5)
  }
  else if (hit.kind === 'toggle') { state.active = !state.active; if (reduced) { state.position = state.active ? 1 : 0; state.velocity = 0 } }
  else {
    const duration = hit.duration ?? 8
    state.duration = duration
    if (reduced) { state.phase = state.phase < duration * .125 ? duration * .275 : state.phase < duration * .375 ? duration * .45 : 0; state.active = false }
    else { state.active = true; state.phase = 0 }
  }
  return hit.label
}
export function stepLabInteraction(interaction: LabInteraction, elapsed: number) {
  const steps = Math.max(1, Math.ceil(elapsed / .016)), dt = Math.min(.06, elapsed) / steps
  for (let step = 0; step < steps; step++) for (const [id, state] of Object.entries(interaction.states)) {
    if (interaction.dragging === id) continue
    if (state.kind === 'release') {
      const duration = state.duration ?? 8
      if (state.active) { state.phase += dt; if (state.phase >= duration) { state.phase = duration; state.active = false } }
      continue
    }
    const stiffness = state.kind === 'toggle' ? 32 : state.kind === 'drive' ? 10 : 7
    const damping = state.kind === 'toggle' ? 10 : state.kind === 'drive' ? 4 : 1.35
    const target = state.kind === 'toggle' && state.active ? 1 : 0
    state.velocity += ((target - state.position) * stiffness - state.velocity * damping) * dt
    state.position += state.velocity * dt
    if (state.kind === 'pendulum') state.position = Math.max(-1.15, Math.min(1.15, state.position))
    if (state.kind === 'drive') state.phase += Math.max(0, state.position) * dt * 4
    if (Math.abs(state.position - target) < .0001 && Math.abs(state.velocity) < .0001) { state.position = target; state.velocity = 0 }
  }
}
