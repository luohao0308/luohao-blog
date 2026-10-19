<script setup lang="ts">
import { paintCityAircraft } from '~/utils/backgroundLabAircraft'
import type { LabPointer } from '~/utils/backgroundLab'
import type { LabInteraction } from '~/utils/backgroundLabInteraction'
defineProps<{ playing: boolean, resetToken: number, showControls: boolean }>()
const emit = defineEmits<{ toggle: [] }>()
const reduced = ref(false)
function paint(ctx: CanvasRenderingContext2D, w: number, h: number, time: number, pointer: LabPointer, interaction?: LabInteraction) {
  ctx.clearRect(0, 0, w, h)
  paintCityAircraft(ctx, w, h, time, pointer, interaction)
}
</script>

<template>
  <Teleport to="body">
    <div class="about-flight-surface" role="region" aria-label="页面上空的巡航飞机">
      <InteractiveStudyCanvas effect="about-airplane" :painter="paint" :playing="playing" :reset-token="resetToken" :transparent="true" :show-labels="false" @motion-preference="reduced = $event" />
      <button v-if="showControls" class="flight-pause" type="button" :disabled="reduced" :aria-label="playing ? '暂停城市与天空动态' : '恢复城市与天空动态'" :aria-pressed="!playing" @click="emit('toggle')">{{ playing ? 'Ⅱ' : '▷' }}</button>
    </div>
  </Teleport>
</template>

<style scoped>
.about-flight-surface { position:fixed; inset:0; z-index:30; overflow:hidden; pointer-events:none; }
.about-flight-surface :deep(.lab-hit-control) { pointer-events:auto; cursor:pointer; }
.flight-pause { position:absolute; left:24px; bottom:24px; width:44px; height:44px; border:1px solid var(--border-default); border-radius:50%; background:var(--surface-page); color:var(--text-muted); pointer-events:auto; font-size:13px; }
.flight-pause:focus-visible { outline:2px solid var(--accent); outline-offset:3px; }
.flight-pause:disabled { opacity:.45; }
</style>
