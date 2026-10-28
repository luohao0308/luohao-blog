<script setup lang="ts">
import { createJourneyScrollSnap } from '~/utils/aboutJourneySnap'
const root = ref<HTMLElement | null>(null)
const mode = ref(false)
let jump: ((index: number) => void) | undefined
let cleanup: (() => void) | undefined
provide('about-journey-next', () => jump?.(1))
onMounted(() => {
  const element = root.value
  if (!element) return
  const stage = element.querySelector<HTMLElement>('.journey-stage')!
  const camera = element.querySelector<HTMLElement>('.journey-camera')!
  const path = element.querySelector<SVGPathElement>('.journey-path')!
  const cursor = element.querySelector<HTMLElement>('.journey-cursor')!
  const scenes = Array.from(element.querySelectorAll<HTMLElement>('[data-journey-scene]'))
  const preference = matchMedia('(prefers-reduced-motion: reduce)')
  let frame: number | undefined
  let points: { x: number, y: number }[] = []
  let lengths: number[] = []
  const clear = () => {
    scenes.forEach(scene => { scene.inert = false; scene.style.removeProperty('left'); scene.style.removeProperty('top') })
    camera.style.removeProperty('transform')
  }
  const update = () => {
    frame = undefined
    if (!mode.value) return
    const nav = document.querySelector('.site-header')?.getBoundingClientRect().height ?? 70
    const distance = Math.max(1, element.offsetHeight - stage.clientHeight)
    const progress = Math.min(1, Math.max(0, (nav - element.getBoundingClientRect().top) / distance))
    const position = progress * (scenes.length - 1)
    const segment = Math.min(scenes.length - 2, Math.floor(position))
    // Most of each interval is a readable hold; only its latter part carries into the next view.
    const fraction = Math.min(1, Math.max(0, (position - segment - .35) / .65))
    const blend = fraction * fraction * (3 - 2 * fraction)
    const a = points[segment]!, b = points[segment + 1]!
    camera.style.transform = `translate3d(${-a.x - (b.x - a.x) * blend}px, ${-a.y - (b.y - a.y) * blend}px, 0)`
    const index = blend < .5 ? segment : segment + 1
    scenes.forEach((scene, i) => { scene.inert = i !== index })
    const pathPosition = (lengths[segment] ?? 0) + ((lengths[segment + 1] ?? 0) - (lengths[segment] ?? 0)) * blend
    const point = path.getPointAtLength(pathPosition)
    cursor.style.transform = `translate3d(${point.x}px, ${point.y}px, 0)`
    cursor.dataset.shape = index === 0 ? 'caret' : index === scenes.length - 1 ? 'contact' : index === 2 ? 'orbit' : 'node'
    path.style.strokeDasharray = `${pathPosition} ${path.getTotalLength()}`
    element.dataset.journeyProgress = progress.toFixed(4)
    element.dataset.currentScene = String(index)
  }
  const snap = createJourneyScrollSnap(() => {
    if (!mode.value || preference.matches || document.hidden) return null
    const nav = document.querySelector('.site-header')?.getBoundingClientRect().height ?? 70
    return { scroll: scrollY, start: scrollY + element.getBoundingClientRect().top - nav, distance: element.offsetHeight - stage.clientHeight, count: scenes.length }
  }, top => scrollTo({ top, behavior: 'smooth' }))
  const schedule = () => { if (frame === undefined) frame = requestAnimationFrame(update); snap.schedule() }
  const configure = () => {
    snap.cancel()
    clear()
    const nav = document.querySelector('.site-header')?.getBoundingClientRect().height ?? 70
    element.style.setProperty('--journey-nav', `${nav}px`)
    element.style.setProperty('--journey-count', String(scenes.length))
    const enabled = !preference.matches && scenes.length > 1 && CSS.supports('position', 'sticky')
    element.classList.toggle('is-continuous', enabled)
    // Measure the real responsive content before opting into the camera; long profiles stay ordinary pages.
    const available = Math.max(400, innerHeight - nav)
    const fits = scenes.every(scene => scene.scrollHeight <= available - 76)
    mode.value = enabled && fits
    element.classList.toggle('is-continuous', mode.value)
    if (!mode.value) { clear(); update(); return }
    const height = stage.clientHeight
    points = scenes.map((_, index) => ({ x: 0, y: index * height * 1.18 }))
    scenes.forEach((scene, index) => { scene.style.left = `${points[index]!.x}px`; scene.style.top = `${points[index]!.y}px` })
    const anchors = points.map(point => ({ x: 12, y: point.y + 110 }))
    let d = `M ${anchors[0]!.x} ${anchors[0]!.y}`
    lengths = [0]
    anchors.slice(1).forEach(target => {
      d += ` L ${target.x} ${target.y}`
      path.setAttribute('d', d)
      lengths.push(path.getTotalLength())
    })
    update()
  }
  jump = (index) => {
    if (!mode.value) { scenes[index]?.scrollIntoView({ behavior: 'smooth' }); return }
    const nav = document.querySelector('.site-header')?.getBoundingClientRect().height ?? 70
    const distance = element.offsetHeight - stage.clientHeight
    scrollTo({ top: scrollY + element.getBoundingClientRect().top - nav + distance * index / Math.max(1, scenes.length - 1), behavior: 'smooth' })
  }
  window.addEventListener('scroll', schedule, { passive: true })
  window.addEventListener('resize', configure)
  preference.addEventListener('change', configure)
  configure()
  cleanup = () => {
    window.removeEventListener('scroll', schedule)
    window.removeEventListener('resize', configure)
    preference.removeEventListener('change', configure)
    if (frame !== undefined) cancelAnimationFrame(frame)
    snap.cancel()
    clear()
  }
})
onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <article ref="root" class="continuous-story">
    <div class="journey-stage">
      <AboutPointerBackground />
      <div class="journey-camera">
        <svg class="journey-thread" aria-hidden="true"><path class="journey-path" fill="none" stroke="currentColor" stroke-width="1.5" /></svg>
        <span class="journey-cursor" aria-hidden="true" />
        <slot />
      </div>
    </div>
  </article>
</template>

<style scoped>
.continuous-story { --journey-nav: 70px; position: relative; }
:deep([data-journey-scene]) { scroll-margin-top: calc(var(--journey-nav) + 24px); }
.journey-ground, .journey-thread, .journey-cursor { display: none; }
.is-continuous { height: calc(var(--journey-count) * 125svh); }
.is-continuous .journey-stage { position: sticky; top: var(--journey-nav); height: calc(100svh - var(--journey-nav)); min-height: 400px; overflow: clip; isolation: isolate; }
.is-continuous .journey-ground { display: block; position: absolute; z-index: -1; inset: 0; background: #fdfbf7; mask-image: linear-gradient(transparent, #000 10% 90%, transparent); }
.journey-fluid { width: 100%; height: 100%; opacity: .42; pointer-events: none; }
:global(.dark .journey-ground) { background: var(--surface-page); }
.journey-fluid[data-ink-state='unsupported'], .journey-fluid[data-ink-state='error'], .journey-fluid[data-ink-state='lost'], .journey-fluid[data-ink-state='fallback'] { visibility: hidden; }
.is-continuous .journey-camera { position: absolute; inset: 0; will-change: transform; }
.is-continuous :deep([data-journey-scene]) { position: absolute; width: 100%; padding: 44px 32px 64px; margin: 0; border: 0; box-sizing: border-box; scroll-margin-top: 130px; }
.is-continuous :deep(.about-story) { display: contents; }
.is-continuous :deep(.about-chapter::before), .is-continuous :deep(.narrative-light) { display: none; }
.is-continuous :deep(.narrative-stage) { padding: 0; }
.is-continuous :deep(.narrative-opening) { min-height: calc(100svh - var(--journey-nav) - 200px); }
.is-continuous :deep(.narrative-introduction:not(.has-personal-intro)) { display: none; }
.is-continuous :deep(h2) { font-size: clamp(28px, 4vw, 48px); margin-bottom: 28px; }
.is-continuous :deep(.skill-group) { padding: 24px 20px; background: color-mix(in srgb, var(--surface-page) 88%, transparent); }
.is-continuous :deep(.skill-number) { margin-bottom: 20px; }
.is-continuous :deep(.journey-list li) { padding: 20px 0; }
.is-continuous :deep(.journey-year) { font-size: 52px; }
.is-continuous .journey-thread { display: block; position: absolute; inset: 0; width: 400%; height: 400%; overflow: visible; color: #b79a66; pointer-events: none; }
.is-continuous .journey-cursor { display: block; position: absolute; left: -5px; top: -5px; width: 10px; height: 10px; border: 1.5px solid #a48145; border-radius: 50%; background: var(--surface-page); pointer-events: none; }
.journey-cursor[data-shape='caret'] { width: 3px; height: 36px; border-radius: 0; background: #a48145; }
.journey-cursor[data-shape='orbit'] { width: 18px; height: 18px; box-shadow: 0 0 0 8px rgb(183 154 102 / 10%); }
.journey-cursor[data-shape='contact'] { width: 22px; height: 16px; border-radius: 3px; }
.journey-cursor[data-shape='contact']::after { content: ''; position: absolute; left: 4px; top: -4px; width: 10px; height: 10px; border-right: 1px solid #a48145; border-bottom: 1px solid #a48145; transform: rotate(45deg); }
@media(max-width:640px), (max-height:760px) {
  .is-continuous :deep([data-journey-scene]) { padding: 24px 18px 64px; }
  .is-continuous :deep(.narrative-opening) { min-height: calc(100svh - var(--journey-nav) - 180px); padding: 16px 0 8px; gap: 16px; }
  .is-continuous :deep(.narrative-greeting) { font-size: 20px; }
  .is-continuous :deep(h1) { font-size: clamp(54px, 8vw, 84px); margin: 12px 0 16px; }
  .is-continuous :deep(.skills-grid) { grid-template-columns: 1fr; gap: 12px; }
  .is-continuous :deep(.skill-group) { padding: 12px; display: grid; grid-template-columns: 52px minmax(0, 1fr); gap: 8px; border-radius: 12px; }
  .is-continuous :deep(.skill-number) { display: none; }
  .is-continuous :deep(.skill-group h3) { margin: 6px 0 0; font-size: 12px; }
  .is-continuous :deep(.skill-group li), .is-continuous :deep(.skill-group li:first-child) { flex-basis: auto; padding: 3px 5px; margin: 0; font-size: 11px; letter-spacing: 0; }
  .is-continuous :deep(.skill-group ul) { gap: 4px; }
  .is-continuous :deep(.journey-list li) { grid-template-columns: 56px minmax(0, 1fr); gap: 12px; padding: 14px 0; }
  .is-continuous :deep(.journey-year) { font-size: 25px; }
  .is-continuous :deep(.journey-text) { font-size: 14px; line-height: 1.8; }
  .is-continuous :deep(.chapter-label) { margin-bottom: 12px; }
  .is-continuous :deep(h2) { font-size: 28px; margin-bottom: 20px; }
  .is-continuous :deep(.selected-grid) { gap: 20px; }
  .is-continuous :deep(.selected-group h3) { margin-bottom: 8px; }
  .is-continuous :deep(.selected-group a) { padding: 10px 0; min-height: 36px; font-size: 13px; }
}
.journey-stage { position:relative; isolation:isolate; }

.journey-camera { position:relative; z-index:1; pointer-events:none; }
.journey-camera :deep(a), .journey-camera :deep(button), .journey-camera :deep(input), .journey-camera :deep(p), .journey-camera :deep(h1), .journey-camera :deep(h2), .journey-camera :deep(h3), .journey-camera :deep(li) { pointer-events:auto; }

.is-continuous .journey-camera { pointer-events:none; }
:deep(.about-chapter) { background:transparent; }
:deep(.about-narrative .narrative-light) { display:none; }
:deep(.narrative-opening) { text-align:left; align-items:flex-start; }
:deep(.narrative-identity) { width:min(100%, 720px); }
:deep(.narrative-headline) { margin-inline:0; font-size:18px; }
@media(max-width:640px){

 :deep(.narrative-opening){min-height:480px;justify-content:flex-start;gap:14px;padding-top:26px;}
 :deep(.narrative-identity){width:100%;}
 :deep(.narrative-opening h1){font-size:64px;margin:10px 0 14px;}
 :deep(.narrative-headline){font-size:15px;max-width:32ch;}
 :deep(.narrative-scroll){position:absolute;bottom:24px;}
 .is-continuous :deep(.narrative-opening){min-height:calc(100svh - var(--journey-nav) - 130px);justify-content:flex-start;gap:10px;}
 .is-continuous :deep(.narrative-opening h1){font-size:52px;}
 .is-continuous :deep(.narrative-scroll){top:auto;bottom:15px;}
}
.is-continuous .journey-cursor[data-shape='caret'] { display:none; }
</style>
