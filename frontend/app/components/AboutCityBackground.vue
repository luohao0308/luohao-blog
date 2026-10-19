<script setup lang="ts">
import { paintBusinessCity } from '~/utils/backgroundLabCityVariants'
import type { LabPointer } from '~/utils/backgroundLab'
import type { LabInteraction } from '~/utils/backgroundLabInteraction'
defineProps<{ enabled: boolean }>()
const playback = inject('about-city-playback', { playing: ref(true), resetToken: ref(0) })
const playing = playback.playing
const reduced = ref(false)
const resetToken = playback.resetToken
const themeToken = ref(0)
let themeObserver: MutationObserver | undefined
onMounted(() => {
  themeObserver = new MutationObserver(() => { themeToken.value++ })
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] })
})
onBeforeUnmount(() => themeObserver?.disconnect())
function paint(ctx: CanvasRenderingContext2D, w: number, h: number, time: number, pointer: LabPointer, interaction?: LabInteraction) {
  ctx.fillStyle = document.documentElement.classList.contains('dark') ? '#171c1b' : '#fdfbf7'
  ctx.fillRect(0, 0, w, h)
  paintBusinessCity(ctx, w, h, time, pointer, interaction, { aircraft: false })
}
</script>

<template>
  <div class="about-city-surface" :class="{ subdued: !enabled }" role="region" aria-label="可互动的商务模型城背景">
    <InteractiveStudyCanvas effect="business-city" :painter="paint" :show-labels="false" :playing="playing && enabled" :reset-token="resetToken" :render-token="themeToken" @motion-preference="reduced = $event" />
    <div v-if="enabled" class="city-tools">
      <div><button type="button" :disabled="reduced" :aria-pressed="playing" @click="playing = !playing">{{ playing ? '暂停城市' : '播放城市' }}</button><button type="button" @click="resetToken++">重置城市</button></div>
    </div>
  </div>
</template>

<style scoped>
.about-city-surface { position:absolute; inset:0; overflow:hidden; transition:opacity .4s; }
.about-city-surface.subdued { opacity:.13; pointer-events:none; }
.city-tools { position:absolute; right:16px; bottom:64px; display:flex; flex-direction:column; align-items:end; gap:8px; max-width:66%; }
.city-hint { padding:4px 8px; border-radius:5px; background:color-mix(in srgb,var(--surface-page) 88%,transparent); font-size:10px; color:var(--text-muted); }
.city-tools button { min-height:44px; padding:8px 12px; border:1px solid var(--border-default); border-radius:8px; margin-left:6px; font-size:11px; color:var(--text-primary); background:var(--surface-page); }
.city-tools button:focus-visible { outline:2px solid var(--accent); outline-offset:3px; }
.city-tools button:disabled { opacity:.5; }
@media(max-width:640px) { .city-tools { right:auto; left:8px; bottom:58px; max-width:76%; align-items:start; }.city-hint{font-size:9px}.city-tools button { font-size:10px; padding:6px 10px; margin-left:0; margin-right:6px; } }
@media(prefers-reduced-motion:reduce){.about-city-surface{transition:none}}
</style>
