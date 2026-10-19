<script setup lang="ts">
import type { LabPointer } from '~/utils/backgroundLab'
import { activateLabHit, createLabInteraction, hitLabRegion, labTarget, stepLabInteraction, type LabHitRegion, type LabInteraction } from '~/utils/backgroundLabInteraction'
const props = defineProps<{ effect: string, playing: boolean, resetToken?: number, renderToken?: number, showLabels?: boolean, transparent?: boolean, painter: (ctx: CanvasRenderingContext2D, width: number, height: number, time: number, pointer: LabPointer, interaction?: LabInteraction) => void }>()
const emit = defineEmits<{ 'motion-preference': [reduced: boolean], 'interaction-label': [label: string] }>()
const canvas = ref<HTMLCanvasElement | null>(null)
const state = ref('starting')
const targets = shallowRef<LabHitRegion[]>([])
const hoveredId = ref<string | null>(null)
const paintWidth = ref(1)
const hoveredTarget = computed(() => targets.value.find(hit => hit.id === hoveredId.value))
const feedback = ref('')
let activateById: (id: string) => void = () => {}
let focusHit: (id: string | null) => void = () => {}
let cleanup: (() => void) | undefined
onMounted(() => {
  const surface = canvas.value, ctx = surface?.getContext('2d', { alpha: props.transparent ?? false })
  if (!surface || !ctx) { state.value = 'unsupported'; return }
  const stage = surface.closest<HTMLElement>('.lab-stage, .about-city-surface, .about-flight-surface') ?? surface.parentElement!
  const preference = matchMedia('(prefers-reduced-motion: reduce)')
  let width = 1, height = 1, ratio = 1, visible = true, frame: number | undefined
  let elapsed = 0, last = 0, lastDraw = 0, frames = 0
  let interaction = createLabInteraction()
  let pressed: { hit: LabHitRegion, x: number, y: number, time: number, moved: boolean, pointerId: number } | null = null
  const originalCursor = stage.style.cursor, originalTouch = stage.style.touchAction
  stage.style.touchAction = 'pan-y'
  const pointer: LabPointer = { x: 0, y: 0, strength: 0 }, target: LabPointer = { x: 0, y: 0, strength: 0 }
  const active = () => props.playing && !preference.matches && !document.hidden && visible
  function paint() {
    interaction.ratio = ratio; interaction.hits = []
    ctx!.setTransform(ratio, 0, 0, ratio, 0, 0)
    props.painter(ctx!, width, height, elapsed, pointer, interaction)
    targets.value = interaction.hits.map(hit => ({ ...hit }))
    surface!.dataset.labFrame = String(++frames); surface!.dataset.labPointer = pointer.strength.toFixed(3)
    surface!.dataset.labActions = String(interaction.actions); surface!.dataset.labLastHit = interaction.lastHit
    surface!.dataset.labActivity = Math.max(0, ...Object.values(interaction.states).map(value => Math.abs(value.position) + Math.abs(value.velocity))).toFixed(3)
  }
  function clearPress() {
    const held = pressed
    pressed = null; interaction.dragging = null; interaction.hovered = null; hoveredId.value = null
    emit('interaction-label', '')
    if (held && stage.hasPointerCapture(held.pointerId)) stage.releasePointerCapture(held.pointerId)
    stage.style.cursor = originalCursor
  }
  function sync() {
    if (!active()) clearPress()
    if (frame !== undefined) cancelAnimationFrame(frame)
    frame = undefined; last = 0; lastDraw = 0
    emit('motion-preference', preference.matches)
    state.value = preference.matches ? 'reduced' : document.hidden ? 'hidden' : !visible ? 'offscreen' : !props.playing ? 'paused' : 'running'
    if (active()) frame = requestAnimationFrame(tick)
    else paint()
  }
  function tick(now: number) {
    frame = undefined
    if (!active()) { sync(); return }
    const delta = last ? Math.min(.06, (now - last) / 1000) : 0
    last = now; elapsed += delta; stepLabInteraction(interaction, delta)
    if (now - lastDraw >= 1000 / 30) {
      pointer.x += (target.x - pointer.x) * .12; pointer.y += (target.y - pointer.y) * .12; pointer.strength += (target.strength - pointer.strength) * .12
      paint(); lastDraw = now
    }
    frame = requestAnimationFrame(tick)
  }
  function resize() {
    const box = surface!.getBoundingClientRect(); width = Math.max(1, box.width); height = Math.max(1, box.height); paintWidth.value = width
    ratio = Math.min(devicePixelRatio || 1, 1.5, Math.sqrt(1500000 / (width * height)))
    surface!.width = Math.max(1, Math.floor(width * ratio)); surface!.height = Math.max(1, Math.floor(height * ratio))
    paint(); sync()
  }
  function local(event: PointerEvent) { const box = surface!.getBoundingClientRect(); return { x: event.clientX - box.left, y: event.clientY - box.top } }
  function labelHit(hit: LabHitRegion | undefined) {
    const id = hit?.id ?? null
    if (interaction.hovered !== id) { interaction.hovered = id; hoveredId.value = id; emit('interaction-label', hit?.label ?? ''); if (!active() && props.playing) paint() }
    stage.style.cursor = !props.playing ? 'default' : hit ? hit.kind === 'pendulum' ? 'grab' : 'pointer' : originalCursor
  }
  function perform(hit: LabHitRegion) {
    if (!props.playing) return
    activateLabHit(interaction, hit, preference.matches)
    feedback.value = `已响应：${hit.label.split('：')[0]}`
    emit('interaction-label', hit.label); paint(); sync()
  }
  activateById = id => { const hit = interaction.hits.find(item => item.id === id); if (hit) perform(hit) }
  focusHit = id => labelHit(interaction.hits.find(hit => hit.id === id))
  function move(event: PointerEvent) {
    if (pressed && event.pointerId !== pressed.pointerId) return
    if (!props.playing) return
    const point = local(event)
    if (pressed && interaction.dragging) {
      const hit = pressed.hit, value = labTarget(interaction, hit.id, hit.kind)
      const angle = Math.max(-1.05, Math.min(1.05, Math.atan2(point.x - hit.anchorX!, point.y - hit.anchorY!)))
      const dt = Math.max(.016, (event.timeStamp - pressed.time) / 1000)
      value.velocity = Math.max(-6, Math.min(6, (angle - value.position) / dt)); value.position = angle
      pressed.moved ||= Math.hypot(point.x - pressed.x, point.y - pressed.y) > 4; pressed.time = event.timeStamp
      stage.style.cursor = 'grabbing'
      return
    }
    labelHit(hitLabRegion(interaction, point.x, point.y))
    if (!active()) return
    target.x = Math.max(-1, Math.min(1, point.x / width * 2 - 1)); target.y = Math.max(-1, Math.min(1, point.y / height * 2 - 1)); target.strength = 1
  }
  function leave(event?: PointerEvent) {
    if (pressed && event && event.pointerId !== pressed.pointerId) return
    target.x = 0; target.y = 0; target.strength = 0
    if (!interaction.dragging) { pressed = null; labelHit(undefined) }
  }
  function down(event: PointerEvent) {
    if (pressed || !props.playing || event.button !== 0 || (event.target as Element)?.closest('button, a, input')) return
    const point = local(event), hit = hitLabRegion(interaction, point.x, point.y)
    if (!hit) return
    event.preventDefault(); labelHit(hit)
    pressed = { hit: { ...hit }, x: point.x, y: point.y, time: event.timeStamp, moved: false, pointerId: event.pointerId }
    if (hit.kind === 'pendulum' && !preference.matches && hit.anchorX !== undefined && hit.anchorY !== undefined) {
      interaction.dragging = hit.id
      if (event.isTrusted) stage.setPointerCapture(event.pointerId)
      stage.style.cursor = 'grabbing'
    }
  }
  function up(event: PointerEvent) {
    if (!pressed || event.pointerId !== pressed.pointerId) return
    if (!props.playing || document.hidden || !visible) { clearPress(); return }
    const held = pressed
    pressed = null; interaction.dragging = null
    if (stage.hasPointerCapture(event.pointerId)) stage.releasePointerCapture(event.pointerId)
    if (!held) return
    const point = local(event)
    if (held.moved) {
      const value = labTarget(interaction, held.hit.id, held.hit.kind)
      if (event.timeStamp - held.time > 100) value.velocity = 0
      interaction.actions++; interaction.lastHit = held.hit.id
      feedback.value = '已释放平衡球'; paint()
    }
    else if (Math.hypot(point.x - held.x, point.y - held.y) < 12) perform(held.hit)
    labelHit(hitLabRegion(interaction, point.x, point.y))
  }
  function cancel(event: PointerEvent) {
    if (!pressed || event.pointerId !== pressed.pointerId) return
    clearPress(); leave()
  }
  function lost(event: PointerEvent) {
    if (pressed?.pointerId === event.pointerId) clearPress()
  }
  stage.addEventListener('pointermove', move, { passive: true }); stage.addEventListener('pointerleave', leave)
  stage.addEventListener('pointerdown', down); stage.addEventListener('pointerup', up); stage.addEventListener('pointercancel', cancel); stage.addEventListener('lostpointercapture', lost)
  document.addEventListener('visibilitychange', sync); preference.addEventListener('change', sync)
  const observer = new ResizeObserver(resize); observer.observe(surface)
  const intersection = new IntersectionObserver(entries => { visible = entries.some(entry => entry.isIntersecting); sync() }); intersection.observe(surface)
  const unwatch = watch([() => props.effect, () => props.playing, () => props.resetToken], ([effect, , reset], [old, , oldReset]) => {
    if (effect !== old || reset !== oldReset) {
      clearPress(); interaction = createLabInteraction(); elapsed = 0; feedback.value = ''; emit('interaction-label', ''); leave(); pointer.x = 0; pointer.y = 0; pointer.strength = 0
    }
    paint(); sync()
  })
  const unwatchRender = watch(() => props.renderToken, paint)
  resize()
  cleanup = () => {
    if (frame !== undefined) cancelAnimationFrame(frame)
    clearPress()
    unwatch(); unwatchRender(); observer.disconnect(); intersection.disconnect()
    document.removeEventListener('visibilitychange', sync); preference.removeEventListener('change', sync)
    stage.removeEventListener('pointermove', move); stage.removeEventListener('pointerleave', leave); stage.removeEventListener('pointerdown', down); stage.removeEventListener('pointerup', up); stage.removeEventListener('pointercancel', cancel); stage.removeEventListener('lostpointercapture', lost)
    stage.style.cursor = originalCursor; stage.style.touchAction = originalTouch
  }
})
onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <div class="background-lab-layer">
    <canvas ref="canvas" class="background-lab-canvas" :data-effect="effect" :data-render-state="state" aria-hidden="true" />
    <button v-for="hit in targets" :key="hit.id" type="button" class="lab-hit-control" :data-lab-hit="hit.id" :aria-label="hit.label" :disabled="!playing" :style="{ left: `${hit.x}px`, top: `${hit.y}px` }" @click="activateById(hit.id)" @focus="focusHit(hit.id)" @blur="focusHit(null)">↵</button>
    <span v-if="hoveredTarget && playing && showLabels !== false" class="lab-hover-label" :style="{ left: `${Math.max(90, Math.min(paintWidth - 90, hoveredTarget.x))}px`, top: `${Math.max(8, hoveredTarget.y - hoveredTarget.radius - 46)}px` }">{{ hoveredTarget.label }}</span>
    <span class="sr-only" role="status" aria-live="polite">{{ feedback }}</span>
  </div>
</template>

<style scoped>
.background-lab-layer { position: absolute; inset: 0; pointer-events: none; }
.background-lab-canvas { display: block; position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
.lab-hover-label { position:absolute; z-index:4; width:max-content; max-width:180px; transform:translateX(-50%); border:1px solid #b99b64; border-radius:8px; padding:7px 10px; background:#fff9e9; color:#3d4e3e; box-shadow:0 4px 12px #2c3f3217; font-size:11px; line-height:1.5; pointer-events:none; }
.lab-hit-control { position: absolute; width: 44px; height: 44px; margin: -22px 0 0 -22px; border-radius: 50%; border: 2px solid #b88747; background: #f9efda; color: #59452b; opacity: 0; pointer-events: none; }
.lab-hit-control:focus-visible { opacity: 1; outline: 3px solid #b88747; outline-offset: 4px; }
</style>
