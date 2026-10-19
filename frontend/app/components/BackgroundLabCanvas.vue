<script setup lang="ts">
import { paintBackgroundStudy, type LabPointer } from '~/utils/backgroundLab'
import type { BackgroundStudyId } from '~/data/backgroundLab'
import type { LabInteraction } from '~/utils/backgroundLabInteraction'
import InteractiveStudyCanvas from '~/components/InteractiveStudyCanvas.vue'
const props = defineProps<{ effect: BackgroundStudyId, playing: boolean, resetToken?: number }>()
const emit = defineEmits<{ 'motion-preference': [reduced: boolean], 'interaction-label': [label: string] }>()
const painter = (ctx: CanvasRenderingContext2D, width: number, height: number, time: number, pointer: LabPointer, interaction?: LabInteraction) => {
  paintBackgroundStudy(ctx, props.effect, width, height, time, pointer, interaction)
}
</script>

<template>
  <InteractiveStudyCanvas :effect="effect" :playing="playing" :reset-token="resetToken" :painter="painter" :show-labels="effect !== 'business-city'" @motion-preference="emit('motion-preference', $event)" @interaction-label="emit('interaction-label', $event)" />
</template>

<style scoped>
/* Drawing styles live in InteractiveStudyCanvas. */
</style>
