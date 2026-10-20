<script setup lang="ts">
import { paintBusinessCity } from '~/utils/backgroundLabCityVariants'
import type { LabPointer } from '~/utils/backgroundLab'
import type { LabInteraction } from '~/utils/backgroundLabInteraction'

const props = defineProps<{ playing: boolean, resetToken: number }>()
const themeToken = ref(0)
let themeObserver: MutationObserver | undefined

onMounted(() => {
  themeObserver = new MutationObserver(() => { themeToken.value++ })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})
onBeforeUnmount(() => themeObserver?.disconnect())

function paint(ctx: CanvasRenderingContext2D, width: number, height: number, time: number, pointer: LabPointer, interaction?: LabInteraction) {
  ctx.clearRect(0, 0, width, height)
  paintBusinessCity(ctx, width, height, time, pointer, interaction, { aircraft: false, layout: 'hero' })
}
</script>

<template>
  <div class="home-visual about-city-surface" role="region" aria-label="可互动的模型城">
    <ClientOnly>
      <InteractiveStudyCanvas effect="business-city" :painter="paint" :playing="props.playing" :transparent="true" :reset-token="resetToken" :render-token="themeToken" :show-labels="false" />
    </ClientOnly>

  </div>
</template>

<style scoped>
.home-visual { position: relative; min-width: 0; width: 100%; aspect-ratio: 1.3; min-height: 320px; margin: auto; overflow: clip; }
@media (max-width: 640px) { .home-visual { max-width: 440px; } }
</style>
