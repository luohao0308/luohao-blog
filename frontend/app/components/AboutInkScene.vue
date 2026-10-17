<script setup lang="ts">
import { mountAboutInk } from '~/utils/aboutInk'

defineProps<{ name: string, headline: string }>()
const scene = ref<HTMLElement | null>(null)
const canvas = ref<HTMLCanvasElement | null>(null)
let cleanup: (() => void) | undefined
onMounted(() => {
  const root = scene.value, surface = canvas.value
  if (!root || !surface) return
  let ink: ReturnType<typeof mountAboutInk> = null
  try { ink = mountAboutInk(surface) }
  catch (error) {
    surface.dataset.inkState = 'error'
    console.error('About ink initialization failed', error)
  }
  if (!ink && !surface.dataset.inkState) surface.dataset.inkState = 'fallback'
  let frame: number | undefined
  const update = () => {
    frame = undefined
    const header = document.querySelector('.site-header')?.getBoundingClientRect().height ?? 70
    root.style.setProperty('--ink-nav-height', `${header}px`)
    const box = root.getBoundingClientRect()
    const distance = Math.max(1, box.height - (window.innerHeight - header))
    const progress = Math.max(0, Math.min(1, (header - box.top) / distance))
    root.dataset.inkProgress = progress.toFixed(3)
    root.style.setProperty('--ink-progress', String(progress))
    ink?.setProgress(progress)
  }
  const schedule = () => { if (frame === undefined) frame = requestAnimationFrame(update) }
  window.addEventListener('scroll', schedule, { passive: true })
  window.addEventListener('resize', schedule)
  update()
  cleanup = () => {
    window.removeEventListener('scroll', schedule)
    window.removeEventListener('resize', schedule)
    if (frame !== undefined) cancelAnimationFrame(frame)
    ink?.destroy()
  }
})
onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <section ref="scene" data-about-ink data-ink-progress="0" class="ink-track" aria-labelledby="about-ink-name">
    <div class="ink-stage">
      <div class="ink-fallback" aria-hidden="true" />
      <canvas ref="canvas" data-ink-canvas class="ink-canvas" aria-hidden="true" />
      <div class="ink-topline">
        <span>ABOUT / THE PERSON BEHIND THE CODE</span>
        <span class="ink-edition">保持好奇，持续构建。</span>
      </div>
      <div class="ink-identity">
        <p class="ink-eyebrow">你好，我是</p>
        <h1 id="about-ink-name">{{ name }}</h1>
        <span class="ink-signature">Code. Create. Explore.</span>
      </div>
      <div class="ink-bottomline">
        <p class="ink-headline">{{ headline || '把想法写成代码，把探索留在这里。' }}</p>
        <a href="#about-story" class="ink-scroll"><span>向下探索</span><span aria-hidden="true">↓</span></a>
      </div>
      <div class="ink-spine" aria-hidden="true"><i /></div>
    </div>
  </section>
</template>

<style scoped>
.ink-track { --ink-progress: 0; --ink-nav-height: 70px; height: 165svh; min-height: 780px; position: relative; background: var(--surface-page); }
.ink-stage { height: calc(100svh - var(--ink-nav-height)); min-height: 520px; position: sticky; top: var(--ink-nav-height); overflow: clip; isolation: isolate; color: var(--text-primary); }
.ink-canvas, .ink-fallback { position: absolute; inset: 0; width: 100%; height: 100%; }
.ink-canvas { touch-action: pan-y; }
.ink-canvas[data-ink-state='lost'], .ink-canvas[data-ink-state='fallback'], .ink-canvas[data-ink-state='unsupported'], .ink-canvas[data-ink-state='error'] { visibility: hidden; }
.ink-fallback { background: radial-gradient(ellipse at 15% 60%, #c4d2df80, transparent 60%), radial-gradient(ellipse at 85% 30%, #d9e2e990, transparent 50%), var(--surface-page); }
.ink-topline, .ink-bottomline { position: absolute; z-index: 1; left: clamp(24px, 5vw, 80px); right: clamp(24px, 5vw, 80px); display: flex; justify-content: space-between; gap: 28px; pointer-events: none; }
.ink-topline { top: clamp(26px, 5vh, 56px); font: 10px/1.7 ui-monospace, monospace; letter-spacing: .12em; color: var(--text-muted); }
.ink-edition { font-family: inherit; letter-spacing: .1em; }
.ink-identity { position: absolute; z-index: 1; inset: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 28px; pointer-events: none; transform: translateY(calc(var(--ink-progress) * -35px)); }
.ink-eyebrow { font-size: 13px; letter-spacing: .24em; margin-bottom: 10px; color: var(--text-muted); }
.ink-identity h1 { font-family: Georgia, 'Times New Roman', 'Songti SC', serif; font-size: clamp(90px, 23vw, 340px); line-height: .95; font-weight: 400; letter-spacing: -.075em; padding-right: .075em; color: #23394d; text-shadow: 0 1px 0 rgb(255 255 255 / 60%); }
.ink-signature { margin-top: 32px; font: 11px/1.5 ui-monospace, monospace; letter-spacing: .18em; color: var(--text-muted); }
.ink-bottomline { bottom: clamp(35px, 6vh, 72px); align-items: flex-end; }
.ink-headline { max-width: 27ch; font-size: clamp(16px, 1.7vw, 24px); line-height: 1.7; color: #344e66; }
.ink-scroll { display: flex; align-items: center; gap: 28px; min-height: 44px; padding: 6px 0; font-size: 11px; letter-spacing: .12em; color: var(--text-muted); pointer-events: auto; }
.ink-scroll span:last-child { font-size: 22px; }
.ink-spine { position: absolute; bottom: 0; left: 50%; height: 90px; width: 1px; background: color-mix(in srgb, var(--accent) 20%, transparent); opacity: var(--ink-progress); pointer-events: none; }
.ink-spine i { display: block; width: 5px; height: 5px; margin-left: -2px; border-radius: 50%; background: var(--accent); }
:global(.dark) .ink-identity h1 { color: #d4e3ef; text-shadow: none; }
:global(.dark) .ink-headline { color: #bed0e1; }
:global(.dark) .ink-fallback { background: radial-gradient(ellipse at 25% 65%, #274058, transparent 65%), var(--surface-page); }
@media (max-width: 640px) {
  .ink-track { height: 150svh; min-height: 680px; }
  .ink-stage { min-height: 490px; }
  .ink-topline { font-size: 8px; letter-spacing: .04em; }
  .ink-edition { display: none; }
  .ink-identity h1 { font-size: clamp(80px, 24vw, 150px); }
  .ink-eyebrow { font-size: 11px; }
  .ink-signature { font-size: 9px; letter-spacing: .12em; }
  .ink-headline { max-width: 20ch; font-size: 16px; }
  .ink-bottomline { gap: 18px; }
  .ink-scroll { gap: 12px; white-space: nowrap; }
}
@media (prefers-reduced-motion: reduce) {
  .ink-track { height: auto; min-height: 0; }
  .ink-stage { position: relative; top: 0; }
  .ink-identity { transform: none; }
}
</style>
