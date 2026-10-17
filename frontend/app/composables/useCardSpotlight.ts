import type { Ref } from 'vue'
import { mountCardSpotlight } from '~/utils/cardSpotlight'

export function useCardSpotlight(root: Ref<HTMLElement | null>) {
  const route = useRoute()
  let spotlight: ReturnType<typeof mountCardSpotlight> | undefined
  onMounted(() => { if (root.value) spotlight = mountCardSpotlight(root.value) })
  watch(() => route.fullPath, () => spotlight?.reset())
  onBeforeUnmount(() => spotlight?.destroy())
}
