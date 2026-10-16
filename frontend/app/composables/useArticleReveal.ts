import type { Ref } from 'vue'
import { startArticleReveal, type MotionController } from '~/utils/homeMotion'
import { homeMotionEnvironment } from '~/composables/useHomeEntrance'

export function useArticleReveal(root: Ref<HTMLElement | null>): void {
  let controller: MotionController | undefined
  let disposed = false
  const dispose = () => { disposed = true; controller?.stop() }
  onMounted(async () => {
    await nextTick()
    if (disposed || !root.value) return
    controller = startArticleReveal(root.value, homeMotionEnvironment(ref(false)))
  })
  onBeforeUnmount(dispose)
  onBeforeRouteLeave(dispose)
}
