import type { Ref } from 'vue'
import { startHomeEntrance, type MotionController, type MotionEnvironment } from '~/utils/homeMotion'

export function homeMotionEnvironment(memory: Ref<boolean>): MotionEnvironment {
  return {
    events: window,
    document,
    media: window.matchMedia('(prefers-reduced-motion: reduce)'),
    compact: window.matchMedia('(max-width: 640px)').matches,
    viewportWidth: window.innerWidth,
    memory,
    storage: () => window.sessionStorage,
  }
}

export function useHomeEntrance(root: Ref<HTMLElement | null>) {
  const playing = ref(false)
  const memory = useState<boolean>('home-entrance-seen', () => false)
  let controller: MotionController | undefined
  let disposed = false
  const skip = () => { controller?.stop(); playing.value = false }
  const dispose = () => { disposed = true; skip() }
  onMounted(async () => {
    await nextTick()
    if (disposed || !root.value) return
    controller = startHomeEntrance(root.value, homeMotionEnvironment(memory), value => {
      const skipButton = !value && document.activeElement?.matches('[data-home-skip]') ? document.activeElement : null
      playing.value = value
      if (skipButton) void nextTick(() => {
        if (document.activeElement === skipButton || document.activeElement === document.body)
          document.getElementById('main-content')?.focus({ preventScroll: true })
      })
    })
  })
  onBeforeUnmount(dispose)
  onBeforeRouteLeave(dispose)
  return { playing, skip }
}
