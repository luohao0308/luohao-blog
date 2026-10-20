<script setup lang="ts">
import { createAboutBackgroundPainter, type AboutBackgroundId } from '~/utils/aboutPointerBackground'
const effect = ref<AboutBackgroundId>('ripple')
const playing = ref(true)
const reduced = ref(false)
const canvas = ref<HTMLCanvasElement | null>(null)
const root = ref<HTMLElement | null>(null)
let refresh: (() => void) | undefined
let cleanup: (() => void) | undefined
watch([effect, playing], () => refresh?.())
onMounted(() => {
  const surface = canvas.value, element = root.value
  const ctx = surface?.getContext('2d')
  if (!surface || !element || !ctx) return
  const stage = element.parentElement!
  const preference = matchMedia('(prefers-reduced-motion: reduce)')
  const painter = createAboutBackgroundPainter()
  let width = 1, height = 1, ratio = 1, visible = true, frame: number | undefined
  let last = 0, lastDraw = 0, elapsed = 0, frames = 0
  const pointer = { x: .72, y: .48, strength: 0 }, target = { ...pointer }
  const active = () => playing.value && !preference.matches && !document.hidden && visible
  function paint() {
    ctx!.setTransform(ratio, 0, 0, ratio, 0, 0)
    painter(ctx!, width, height, elapsed, pointer, effect.value, document.documentElement.classList.contains('dark'))
    surface!.dataset.backgroundFrame = String(++frames)
    surface!.dataset.pointerStrength = pointer.strength.toFixed(3)
  }
  function tick(now: number) {
    frame = undefined
    if (!active()) { sync(); return }
    const delta = last ? Math.min(.06, (now - last) / 1000) : 0
    last = now; elapsed += delta
    if (now - lastDraw >= 1000 / 30) {
      const blend = 1 - Math.exp(-delta * 9)
      pointer.x += (target.x - pointer.x) * blend; pointer.y += (target.y - pointer.y) * blend
      pointer.strength += (target.strength - pointer.strength) * blend
      paint(); lastDraw = now
    }
    frame = requestAnimationFrame(tick)
  }
  function sync() {
    if (frame !== undefined) cancelAnimationFrame(frame)
    frame = undefined; last = 0; lastDraw = 0; reduced.value = preference.matches
    surface!.dataset.backgroundState = preference.matches ? 'reduced' : document.hidden ? 'hidden' : !visible ? 'offscreen' : !playing.value ? 'paused' : 'running'
    if (preference.matches) { pointer.strength = 0; target.strength = 0 }
    paint()
    if (active()) frame = requestAnimationFrame(tick)
  }
  function resize() {
    const box = surface!.getBoundingClientRect()
    width = Math.max(1, box.width); height = Math.max(1, box.height)
    ratio = Math.min(devicePixelRatio || 1, 1.5, Math.sqrt(1200000 / (width * height)))
    surface!.width = Math.max(1, Math.floor(width * ratio)); surface!.height = Math.max(1, Math.floor(height * ratio))
    sync()
  }
  function move(event: PointerEvent) {
    if (!active()) return
    const box = surface!.getBoundingClientRect()
    target.x = Math.min(1, Math.max(0, (event.clientX - box.left) / width))
    target.y = Math.min(1, Math.max(0, (event.clientY - box.top) / height)); target.strength = 1
  }
  const leave = () => { target.strength = 0 }
  const release = (event: PointerEvent) => { if (event.pointerType !== 'mouse') leave() }
  stage.addEventListener('pointermove', move, { passive: true })
  stage.addEventListener('pointerleave', leave)
  stage.addEventListener('pointercancel', leave)
  stage.addEventListener('pointerup', release)
  document.addEventListener('visibilitychange', sync)
  preference.addEventListener('change', sync)
  const resizeObserver = new ResizeObserver(resize); resizeObserver.observe(surface)
  const intersection = new IntersectionObserver(entries => { visible = entries[0]?.isIntersecting ?? false; sync() }); intersection.observe(surface)
  const theme = new MutationObserver(() => paint()); theme.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
  refresh = sync; resize()
  cleanup = () => {
    refresh = undefined
    if (frame !== undefined) cancelAnimationFrame(frame)
    stage.removeEventListener('pointermove', move); stage.removeEventListener('pointerleave', leave); stage.removeEventListener('pointercancel', leave); stage.removeEventListener('pointerup', release)
    document.removeEventListener('visibilitychange', sync); preference.removeEventListener('change', sync)
    resizeObserver.disconnect(); intersection.disconnect(); theme.disconnect()
  }
})
onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <div ref="root" class="about-pointer-background" :data-background="effect">
    <canvas ref="canvas" aria-hidden="true" />

  </div>
</template>

<style scoped>
.about-pointer-background { position:absolute; inset:0; pointer-events:none; }
canvas { display:block; position:sticky; z-index:0; top:var(--journey-nav,70px); width:100%; height:calc(100svh - var(--journey-nav,70px)); mask-image:linear-gradient(transparent,#000 8% 90%,transparent); }
</style>
