<script setup lang="ts">
const props = withDefaults(defineProps<{ effect?: 'float' | 'focus' | 'wipe' }>(), { effect: 'focus' })
const root = ref<HTMLElement | null>(null)
let cleanup: (() => void) | undefined
onMounted(() => {
  const element = root.value
  if (!element) return
  const nodes = Array.from(element.querySelectorAll<HTMLElement>('[data-reveal]'))
  const preference = matchMedia('(prefers-reduced-motion: reduce)')
  const animations = new Map<HTMLElement, Animation>()
  const show = (node: HTMLElement) => {
    animations.get(node)?.cancel()
    animations.delete(node)
    node.style.opacity = '1'
    node.style.transform = 'none'
    node.style.filter = 'none'
    node.style.clipPath = 'none'
    node.dataset.revealed = 'true'
  }
  if (preference.matches || !('IntersectionObserver' in window) || typeof element.animate !== 'function') {
    nodes.forEach(show)
    return
  }
  nodes.forEach(node => { node.style.opacity = '0' })
  const frames: Keyframe[] = props.effect === 'focus'
    ? [{ opacity: 0, filter: 'blur(12px)', transform: 'translateY(24px) scale(.97)' }, { opacity: 1, filter: 'blur(0px)', transform: 'translateY(0) scale(1)' }]
    : props.effect === 'wipe'
      ? [{ opacity: 0, clipPath: 'inset(0 0 100% 0)', transform: 'translateY(14px)' }, { opacity: 1, clipPath: 'inset(0 0 0 0)', transform: 'translateY(0)' }]
      : [{ opacity: 0, transform: 'translateY(48px)' }, { opacity: 1, transform: 'translateY(0)' }]
  const observer = new IntersectionObserver(entries => {
    for (const entry of entries) {
      if (!entry.isIntersecting) continue
      const node = entry.target as HTMLElement
      observer.unobserve(node)
      if (node.dataset.revealed === 'true') continue
      const siblings = Array.from(node.closest('[data-section]')?.querySelectorAll('[data-reveal]') ?? [])
      const animation = node.animate(frames, { duration: props.effect === 'float' ? 850 : 1150, delay: Math.min(Math.max(siblings.indexOf(node), 0), 4) * 100, easing: 'cubic-bezier(.16,1,.3,1)', fill: 'both' })
      animations.set(node, animation)
      node.dataset.animation = props.effect
      animation.finished.then(() => { if (animations.get(node) === animation) show(node) }).catch(() => {})
    }
  }, { threshold: .12, rootMargin: '0px 0px -7% 0px' })
  nodes.forEach(node => observer.observe(node))
  const focus = (event: FocusEvent) => { const node = (event.target as HTMLElement).closest<HTMLElement>('[data-reveal]'); if (node) { observer.unobserve(node); show(node) } }
  const reduce = () => { if (preference.matches) { observer.disconnect(); nodes.forEach(show) } }
  element.addEventListener('focusin', focus)
  preference.addEventListener('change', reduce)
  cleanup = () => { observer.disconnect(); animations.forEach(animation => animation.cancel()); element.removeEventListener('focusin', focus); preference.removeEventListener('change', reduce) }
})
onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <article ref="root" :data-effect="effect"><slot /></article>
</template>

<style scoped>
:deep([data-reveal]) { transform-origin: left center; }
</style>
