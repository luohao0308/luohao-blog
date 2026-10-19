<script setup lang="ts">
import { mountAboutInk } from '~/utils/aboutInk'
const props = defineProps<{ name: string, headline: string, intro: string, continuous?: boolean, image?: string, skills?: { group: string, items: string[] }[] }>()
const paragraphs = computed(() => props.intro.split(/\n+/).map(line => line.trim()).filter(Boolean))
const root = ref<HTMLElement | null>(null)
let cleanup: (() => void) | undefined
const journeyNext = inject<(() => void) | undefined>('about-journey-next', undefined)
const replay = shallowRef<(() => void) | undefined>()
function continueReading(event: MouseEvent) {
  if (props.continuous && journeyNext) { event.preventDefault(); journeyNext(); return }
  const element = root.value
  if (!element?.classList.contains('is-motion')) return
  event.preventDefault()
  const header = document.querySelector('.site-header')?.getBoundingClientRect().height ?? 70
  const distance = element.offsetHeight - (element.querySelector('.narrative-stage')?.clientHeight ?? innerHeight)
  window.scrollTo({ top: window.scrollY + element.getBoundingClientRect().top - header + distance * .9, behavior: 'smooth' })
  element.querySelector<HTMLElement>('#about-introduction')?.focus({ preventScroll: true })
}

onMounted(() => {
  const element = root.value
  if (!element) return
  const preference = window.matchMedia('(prefers-reduced-motion: reduce)')
  let frame: number | undefined
  let entrance: Animation[] = []
  let light: Animation | undefined
  let fluid: ReturnType<typeof mountAboutInk> = null
  const stopAnimations = () => {
    entrance.forEach(animation => animation.cancel())
    entrance = []
    light?.cancel()
    light = undefined
  }
  const update = () => {
    frame = undefined
    const header = document.querySelector('.site-header')?.getBoundingClientRect().height ?? 70
    element.style.setProperty('--narrative-nav', `${header}px`)
    const stage = element.querySelector('.narrative-stage')
    const distance = Math.max(1, element.offsetHeight - (stage?.clientHeight ?? window.innerHeight))
    const progress = preference.matches ? 0 : Math.min(1, Math.max(0, (header - element.getBoundingClientRect().top) / distance))
    element.style.setProperty('--reading-progress', String(progress))
    element.style.setProperty('--intro-progress', String(Math.min(1, Math.max(0, (progress - .2) / .55))))
    fluid?.setProgress(progress * .4)
    element.dataset.readingProgress = progress.toFixed(3)
    if (light) {
      if (document.hidden || element.getBoundingClientRect().bottom <= header) light.pause()
      else if (light.playState === 'paused') light.play()
    }
  }
  const schedule = () => { if (frame === undefined) frame = requestAnimationFrame(update) }
  const configure = () => {
    stopAnimations()
    const enabled = !preference.matches && typeof element.animate === 'function'
    const introHeight = element.querySelector<HTMLElement>('.narrative-introduction')?.scrollHeight ?? 0
    element.classList.toggle('is-motion', !props.continuous && enabled && introHeight < (window.innerHeight - 140) * .4)
    update()
    if (!enabled) return
    // Animate real SSR text; cancel returns to its readable base style.
    if (Number(element.dataset.readingProgress) < .1) {
      element.querySelectorAll<HTMLElement>('[data-enter]').forEach((item, index) => {
        entrance.push(item.animate([
          { opacity: 0, transform: 'translateY(54px)', filter: 'blur(8px)' },
          { opacity: 1, transform: 'translateY(0)', filter: 'blur(0)' },
        ], { duration: 1600, delay: index * 180, easing: 'cubic-bezier(.16,1,.3,1)', fill: 'backwards' }))
      })
    }
    const glow = element.querySelector<HTMLElement>('.narrative-glow')
    if (glow && !props.continuous) light = glow.animate([
      { transform: 'translate(-12%, 8%) scale(.9)', opacity: .5 },
      { transform: 'translate(12%, -7%) scale(1.18)', opacity: 1 },
    ], { duration: 6500, iterations: Infinity, direction: 'alternate', easing: 'ease-in-out' })
  }
  const visibility = () => {
    if (document.hidden) light?.pause()
    else light?.play()
  }
  window.addEventListener('scroll', schedule, { passive: true })
  window.addEventListener('resize', configure)
  preference.addEventListener('change', configure)
  document.addEventListener('visibilitychange', visibility)
  replay.value = () => {
    const header = document.querySelector('.site-header')?.getBoundingClientRect().height ?? 70
    window.scrollTo({ top: window.scrollY + element.getBoundingClientRect().top - header, behavior: 'instant' })
    configure()
  }
  configure()
  const canvas = element.querySelector<HTMLCanvasElement>('.narrative-fluid')
  const surface = element.querySelector<HTMLElement>('.narrative-stage')
  if (canvas && surface && !props.continuous) {
    try { fluid = mountAboutInk(canvas, { palette: 'warm', surface }) }
    catch (error) { console.error('About light field initialization failed', error); canvas.dataset.inkState = 'fallback' }
  }
  cleanup = () => {
    window.removeEventListener('scroll', schedule)
    window.removeEventListener('resize', configure)
    preference.removeEventListener('change', configure)
    document.removeEventListener('visibilitychange', visibility)
    if (frame !== undefined) cancelAnimationFrame(frame)
    stopAnimations()
    replay.value = undefined
    fluid?.destroy()
  }
})
onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <section ref="root" class="about-narrative" aria-labelledby="about-name">
    <div class="narrative-stage">
      <div class="narrative-light" aria-hidden="true"><div class="narrative-glow" /><canvas v-if="!continuous" class="narrative-fluid" aria-hidden="true" /><div class="narrative-orbit" /></div>
      <button type="button" class="narrative-replay" @click="replay?.()">重播开场 <span aria-hidden="true">↺</span></button>
      <div class="narrative-opening">
        <p class="narrative-label" data-enter>关于我 <span aria-hidden="true">/</span> ABOUT</p>
        <div class="narrative-identity">
          <p class="narrative-greeting" data-enter>你好，我是</p>
          <h1 id="about-name" data-enter>{{ name }}</h1>
          <p v-if="headline" class="narrative-headline" data-enter>{{ headline }}</p>
        </div>
        <a v-if="paragraphs.length || skills?.length" href="#about-introduction" class="narrative-scroll" data-enter @click="continueReading">向下滚动，继续了解 <span aria-hidden="true">↓</span></a>
      </div>
      <div id="about-introduction" tabindex="-1" class="narrative-introduction" :class="{ 'with-portrait': image, 'has-personal-intro': paragraphs.length }">
        <div v-if="paragraphs.length" class="narrative-prose">
          <p v-for="(paragraph, index) in paragraphs" :key="index">{{ paragraph }}</p>
        </div>
        <div v-else-if="skills?.length" class="narrative-directions">
          <div v-for="(group, index) in skills.slice(0, 3)" :key="group.group" :style="{ '--direction-index': index }" class="direction-preview">
            <span>{{ group.group }}</span>
            <strong>{{ group.items[0] }}</strong>
            <p>{{ group.items.slice(1, 3).join(' / ') }}</p>
          </div>
        </div>
        <img v-if="image" :src="assetUrl(image)" :alt="`${name} 头像`" width="160" height="160" class="narrative-portrait">
      </div>
    </div>
  </section>
</template>

<style scoped>
.about-narrative { --reading-progress: 0; --intro-progress: 1; --narrative-nav: 70px; position: relative; isolation: isolate; color: var(--text-primary); margin-bottom: 64px; }
.narrative-stage { position: relative; isolation: isolate; padding-bottom: 72px; }
.narrative-light { position: absolute; z-index: -1; inset: 0; overflow: clip; pointer-events: none; background: #fdfbf7; mask-image: linear-gradient(transparent, #000 12%, #000 80%, transparent); }
.narrative-fluid { position: absolute; inset: 0; width: 100%; height: 100%; opacity: .78; pointer-events: none; }
.narrative-fluid[data-ink-state='unsupported'], .narrative-fluid[data-ink-state='fallback'], .narrative-fluid[data-ink-state='error'], .narrative-fluid[data-ink-state='lost'] { visibility: hidden; }
.narrative-glow { position: absolute; inset: -15%; background: radial-gradient(ellipse 45% 40% at 65% 40%, rgb(222 181 104 / 30%), transparent 90%), radial-gradient(ellipse 40% 40% at 25% 65%, rgb(221 195 155 / 20%), transparent 90%); }
:global(html:not(.dark) .about-narrative) { color: #30302e; }
:global(.dark .narrative-light) { background: var(--surface-page); }
:global(.dark .narrative-glow) { opacity: .65; }
.narrative-opening { min-height: min(660px, 72svh); padding: 40px 0 28px; display: flex; flex-direction: column; align-items: center; justify-content: space-between; text-align: center; gap: 40px; }
.narrative-label { color: var(--text-muted); font-size: 10px; letter-spacing: .22em; }
.narrative-label span { margin: 0 12px; opacity: .4; }
.narrative-identity { width: 100%; }
.narrative-greeting { font-size: clamp(19px, 2.7vw, 28px); font-weight: 500; letter-spacing: .06em; }
h1 { margin: 18px 0 28px; font-size: clamp(64px, 10vw, 128px); line-height: 1.1; font-weight: 600; letter-spacing: -.055em; overflow-wrap: anywhere; }
.narrative-headline { max-width: 38ch; margin-inline: auto; font-size: clamp(16px, 2.1vw, 23px); line-height: 1.9; color: var(--text-muted); text-wrap: balance; }
.narrative-scroll { display: inline-flex; align-items: center; gap: 16px; min-height: 44px; font-size: 12px; color: var(--text-muted); }
.is-motion .narrative-scroll:focus-visible { opacity: 1; }
.narrative-scroll:hover { color: var(--accent); }
.narrative-scroll span { font-size: 19px; }
.narrative-introduction { position: relative; display: grid; align-items: start; gap: 32px; max-width: 760px; margin: 0 auto; padding-top: 48px; scroll-margin-top: 180px; }
.narrative-introduction.with-portrait { grid-template-columns: minmax(0, 1fr) 120px; }
.narrative-prose { display: grid; gap: 24px; }
.narrative-prose p { font-size: clamp(18px, 2.5vw, 28px); line-height: 1.9; overflow-wrap: anywhere; text-wrap: pretty; }
.narrative-orbit { position: absolute; left: 50%; top: 44%; width: min(80vw, 560px); aspect-ratio: 1; border: 1px solid rgb(195 170 124 / 22%); border-radius: 50%; transform: translate(-50%, -50%) scale(calc(.85 + var(--reading-progress) * .6)); }
.narrative-orbit::before, .narrative-orbit::after { content: ''; position: absolute; inset: 28px -32px; border: 1px solid rgb(195 170 124 / 13%); border-radius: 50%; transform: rotate(calc(25deg + var(--reading-progress) * 70deg)); }
.narrative-orbit::after { inset: -32px 28px; transform: rotate(calc(-25deg - var(--reading-progress) * 70deg)); }
.narrative-directions { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 20px; width: 100%; }
.direction-preview { min-width: 0; padding: 20px; border-top: 1px solid rgb(195 170 124 / 45%); transform: translateY(calc((1 - var(--intro-progress)) * var(--direction-index) * 36px)); }
.direction-preview > span { display: block; color: var(--text-muted); font-size: 12px; margin-bottom: 16px; }
.direction-preview strong { display: block; font-size: clamp(20px, 3vw, 36px); line-height: 1.2; font-weight: 500; overflow-wrap: anywhere; }
.direction-preview p { color: var(--text-muted); font-size: 11px; line-height: 1.8; margin-top: 12px; overflow-wrap: anywhere; }
.narrative-portrait { width: 120px; height: 150px; object-fit: cover; border-radius: 16px; }
.narrative-replay { display: none; }
.is-motion .narrative-replay { display: block; position: absolute; top: 24px; right: 0; z-index: 2; padding: 10px 12px; min-height: 44px; color: var(--text-muted); font-size: 11px; }
/* A bounded sticky scene keeps the transformation in view, then releases into the real page. */
.is-motion { height: 180svh; min-height: 1050px; }
.is-motion .narrative-stage { position: sticky; top: var(--narrative-nav); height: calc(100svh - var(--narrative-nav)); min-height: 480px; padding-bottom: 0; }
.is-motion .narrative-opening { position: absolute; inset: 0; min-height: 0; }
.is-motion .narrative-identity { position: absolute; top: 45%; transform: translateY(-50%) translateY(calc(var(--reading-progress) * -140px)) scale(calc(1 - var(--reading-progress) * .46)); }
.is-motion .narrative-greeting { opacity: calc(1 - var(--reading-progress) * 1.3); }
.is-motion .narrative-headline { opacity: calc(1 - var(--reading-progress) * 1.8); }
.is-motion .narrative-scroll { margin-top: auto; opacity: calc(1 - var(--reading-progress) * 3); }
.is-motion .narrative-introduction { position: absolute; top: 57%; left: 50%; width: min(100%, 760px); padding: 0 4px; margin: 0; transform: translate(-50%, calc((1 - var(--intro-progress)) * 90px)); opacity: var(--intro-progress); }
.is-motion .narrative-stage::after { content: ""; position: absolute; bottom: 0; left: 0; height: calc(var(--reading-progress) * 120px); width: 1px; background: #c3aa7c; opacity: var(--reading-progress); }
.is-motion .narrative-light { transform: scale(calc(1 + var(--reading-progress) * .08)); }
@media (max-width: 640px) {
  .narrative-opening { min-height: 66svh; gap: 32px; }
  .narrative-headline { font-size: 16px; }
  .narrative-directions { gap: 10px; }
  .direction-preview { padding: 16px 0; }
  .narrative-introduction.with-portrait { grid-template-columns: 1fr; gap: 24px; }
  .narrative-portrait { width: 64px; height: 80px; grid-row: 1; }
  .is-motion { height: 175svh; min-height: 900px; }
  .is-motion .narrative-identity { top: 43%; transform: translateY(-50%) translateY(calc(var(--reading-progress) * -95px)) scale(calc(1 - var(--reading-progress) * .4)); }
}
@media (prefers-reduced-motion: reduce) {
  .is-motion .narrative-replay { display: none; }
  .is-motion { height: auto; min-height: 0; }
  .is-motion .narrative-stage { position: relative; top: 0; height: auto; min-height: 0; padding-bottom: 72px; }
  .is-motion .narrative-opening { position: relative; min-height: 66svh; }
  .is-motion .narrative-identity { position: static; transform: none; }
  .is-motion .narrative-greeting, .is-motion .narrative-headline, .is-motion .narrative-scroll { opacity: 1; }
  .is-motion .narrative-introduction { position: relative; top: auto; left: auto; max-height: none; overflow: visible; margin-inline: auto; padding-top: 48px; transform: none; opacity: 1; }
}
</style>
