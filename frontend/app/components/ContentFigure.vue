<script setup lang="ts">
import type { EditorialImage } from '~/data/editorial'

const props = defineProps<{ media: EditorialImage }>()
const failed = ref(false)
const imageElement = ref<HTMLImageElement | null>(null)
onMounted(() => {
  if (imageElement.value?.complete && !imageElement.value.naturalWidth) failed.value = true
})
watch(() => props.media.src, () => { failed.value = false })
</script>

<template>
  <figure v-if="!failed" class="space-y-3">
    <a :href="media.src" target="_blank" rel="noopener" :aria-label="`${media.alt}，打开大图`" class="block overflow-hidden rounded-lg bg-slate-100 focus-visible:outline-2 focus-visible:outline-blue-600 dark:bg-slate-900">
      <img ref="imageElement" :src="media.src" :alt="media.alt" :width="media.poster ? 1536 : media.diagram ? 1200 : 1600" :height="media.poster ? 864 : media.diagram ? 680 : 1067" loading="lazy" decoding="async" class="w-full" :class="media.diagram || media.poster ? 'h-auto' : 'aspect-[16/9] object-cover'" @error="failed = true">
    </a>
    <figcaption v-if="media.caption || media.credit" class="flex flex-wrap justify-between gap-x-4 gap-y-1 text-xs leading-6 text-slate-500 dark:text-slate-400">
      <span>{{ media.caption }}</span>
      <a v-if="media.sourceUrl" :href="media.sourceUrl" target="_blank" rel="noopener" class="underline decoration-slate-300 underline-offset-4 hover:text-blue-600">{{ media.credit }} ↗</a>
      <span v-else-if="media.credit">{{ media.credit }}</span>
    </figcaption>
  </figure>
</template>
