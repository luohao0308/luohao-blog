<script setup lang="ts">
const root = ref<HTMLElement | null>(null)
let cleanup: (() => void) | undefined
onMounted(() => {
  const element = root.value
  if (!element || !('IntersectionObserver' in window)) return
  const preference = window.matchMedia('(prefers-reduced-motion: reduce)')
  const animations: Animation[] = []
  const observer = new IntersectionObserver((entries) => {
    if (!entries.some(entry => entry.isIntersecting)) return
    observer.disconnect()
    element.classList.add('is-read')
    if (preference.matches || element.closest('.is-continuous')) return
    element.querySelectorAll<HTMLElement>('[data-chapter-item]').forEach((item, index) => {
      if (typeof item.animate !== 'function') return
      animations.push(item.animate([
        { opacity: .1, transform: item.classList.contains('skill-group') ? 'perspective(1400px) translateY(80px) rotateX(14deg)' : item.closest('.journey-list') ? 'translateX(64px)' : 'translateY(48px)' },
        { opacity: 1, transform: 'translateY(0)' },
      ], { duration: 1200, delay: Math.min(index, 5) * 130, easing: 'cubic-bezier(.16,1,.3,1)', fill: 'backwards' }))
    })
  }, { threshold: .08 })
  const cancel = () => { if (preference.matches) animations.forEach(animation => animation.cancel()) }
  observer.observe(element)
  preference.addEventListener('change', cancel)
  cleanup = () => { observer.disconnect(); animations.forEach(animation => animation.cancel()); preference.removeEventListener('change', cancel) }
})
onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <section ref="root" class="about-chapter"><slot /></section>
</template>
