import type { Ref } from 'vue'
import type { SpritePetState } from 'sprite-pet'

export type PetRequestStatus = 'idle' | 'pending' | 'done' | 'failure'

export function usePetActivity(ready: Ref<boolean>, opened: Ref<boolean>, dragging: Ref<boolean>, position: Ref<{ x: number, y: number } | null>) {
  const state = ref<SpritePetState>('idle')
  let requestStatus: PetRequestStatus = 'idle'
  let timer: ReturnType<typeof setTimeout> | undefined
  let media: MediaQueryList | undefined
  let mounted = false
  let disposed = false
  let generation = 0
  let idleStep = 0
  let pendingStep = 0
  let lastX = position.value?.x
  const idleActions: SpritePetState[] = ['wave', 'jump', 'waiting']
  const pendingActions: SpritePetState[] = ['working', 'reviewing', 'waiting']

  function clearTimer() {
    generation++
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
  }
  function schedule(delay: number, action: () => void) {
    const version = generation
    timer = setTimeout(() => {
      timer = undefined
      if (!disposed && version === generation) action()
    }, delay)
  }
  function canAnimate() {
    return mounted && !disposed && ready.value && !document.hidden && !media?.matches
  }
  function sync() {
    clearTimer()
    if (!canAnimate()) { state.value = 'idle'; return }
    if (dragging.value) { state.value = 'jump'; return }
    if (requestStatus === 'pending') {
      state.value = pendingActions[pendingStep % pendingActions.length]!
      schedule(2500, () => { pendingStep++; sync() })
      return
    }
    if (requestStatus === 'done' || requestStatus === 'failure') {
      state.value = requestStatus === 'done' ? 'wave' : 'failure'
      schedule(1800, () => { requestStatus = 'idle'; sync() })
      return
    }
    state.value = opened.value ? 'waiting' : 'idle'
    if (!opened.value) {
      schedule(12_000 + Math.floor(Math.random() * 6001), () => {
        state.value = idleActions[idleStep++ % idleActions.length]!
        schedule(1600, sync)
      })
    }
  }
  function greet() {
    if (!canAnimate() || dragging.value || requestStatus !== 'idle') return
    clearTimer()
    state.value = 'wave'
    schedule(1600, sync)
  }
  function onRequestStatus(status: PetRequestStatus) {
    requestStatus = status
    if (status === 'pending') pendingStep = 0
    sync()
  }
  function restartIdle() { sync() }
  watch([ready, opened, dragging], () => {
    lastX = position.value?.x
    sync()
  })
  watch(() => position.value?.x, (x) => {
    if (dragging.value && canAnimate() && x !== undefined && lastX !== undefined && x !== lastX)
      state.value = x > lastX ? 'move-right' : 'move-left'
    lastX = x
  })
  onMounted(() => {
    mounted = true
    media = window.matchMedia('(prefers-reduced-motion: reduce)')
    media.addEventListener('change', sync)
    document.addEventListener('visibilitychange', sync)
    sync()
  })
  onBeforeUnmount(() => {
    disposed = true
    mounted = false
    clearTimer()
    media?.removeEventListener('change', sync)
    document.removeEventListener('visibilitychange', sync)
  })
  return { state, greet, onRequestStatus, restartIdle }
}
