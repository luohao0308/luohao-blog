<script setup lang="ts">
import type { SpritePetRenderer, SpritePetState } from 'sprite-pet'

const props = withDefaults(defineProps<{ src: string, state?: SpritePetState, size?: number }>(), { state: 'idle', size: 88 })
const emit = defineEmits<{ ready: [value: boolean], failure: [] }>()
const canvas = ref<HTMLCanvasElement | null>(null)
let renderer: SpritePetRenderer | undefined
let preference: MediaQueryList | undefined
let generation = 0
let disposed = false
let mounted = false

function destroy() {
  renderer?.destroy()
  renderer = undefined
}
function playback() {
  if (!renderer) return
  renderer.setState(preference?.matches ? 'idle' : props.state)
  if (document.hidden || preference?.matches) {
    renderer.pause()
    renderer.render()
  }
  else renderer.play()
}
async function load() {
  const version = ++generation
  destroy()
  emit('ready', false)
  if (!props.src || !canvas.value) return
  const src = props.src
  try {
    const { SpritePetRenderer, loadSpritePetSource, getApproximateAnimationDefinition } = await import('sprite-pet')
    if (disposed || version !== generation) return
    // Image loading has no abort support. Invalidated loads must never attach.
    const source = await loadSpritePetSource({ id: 'usagi', displayName: 'Usagi', spritesheetPath: 'spritesheet.webp' }, src, { crossOrigin: 'anonymous' })
    if (disposed || version !== generation || !canvas.value) return
    const size = Number.isFinite(props.size) && props.size > 0 ? props.size : 88
    // Official adapter preserves valid frame counts; playback approximates the
    // source's non-uniform durations using the low-level fixed-FPS renderer.
    const animations = {
      idle: getApproximateAnimationDefinition('idle'),
      wave: getApproximateAnimationDefinition('waving'),
      working: getApproximateAnimationDefinition('running'),
      failure: getApproximateAnimationDefinition('failed'),
      waiting: getApproximateAnimationDefinition('waiting'),
      reviewing: getApproximateAnimationDefinition('review'),
      jump: getApproximateAnimationDefinition('jumping'),
      'move-right': getApproximateAnimationDefinition('running-right'),
      'move-left': getApproximateAnimationDefinition('running-left'),
    }
    renderer = new SpritePetRenderer({ canvas: canvas.value, source, width: size, height: size * 208 / 192,
      pixelRatio: Math.min(2, window.devicePixelRatio || 1), fit: 'contain', autoplay: false, initialState: 'idle', animations })
    renderer.render()
    playback()
    emit('ready', true)
  }
  catch (error) {
    if (disposed || version !== generation) return
    destroy()
    console.warn('[PetSprite] Preview unavailable', error)
    emit('ready', false)
    emit('failure')
  }
}
watch(() => props.src, () => { if (mounted) void load() })
watch(() => props.state, playback)
watch(() => props.size, () => {
  if (!renderer) return
  const size = Number.isFinite(props.size) && props.size > 0 ? props.size : 88
  renderer.resize(size, size * 208 / 192)
})
onMounted(() => {
  mounted = true
  preference = window.matchMedia('(prefers-reduced-motion: reduce)')
  preference.addEventListener('change', playback)
  document.addEventListener('visibilitychange', playback)
  void load()
})
onBeforeUnmount(() => {
  disposed = true
  mounted = false
  generation++
  preference?.removeEventListener('change', playback)
  document.removeEventListener('visibilitychange', playback)
  destroy()
})
</script>

<template>
  <canvas ref="canvas" class="pet-sprite" aria-hidden="true" />
</template>

<style scoped>
.pet-sprite { display: block; flex: none; max-width: 100%; pointer-events: none; }
</style>
