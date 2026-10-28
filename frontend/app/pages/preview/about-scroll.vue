<script setup lang="ts">
if (!import.meta.dev) throw createError({ statusCode: 404, statusMessage: 'Page not found' })
definePageMeta({ layout: false })
useHead({ title: '关于页 · 下滑效果对比', meta: [{ name: 'robots', content: 'noindex, nofollow' }] })
const route = useRoute()
const router = useRouter()
const { init, toggle } = useTheme()
onMounted(init)
const options = [
  { id: 'float', name: '逐段上浮', detail: '段落依次向上浮现，节奏轻、内容清楚。' },
  { id: 'focus', name: '景深渐显', detail: '文字由虚到实，轻微靠近，适合沉浸阅读。' },
  { id: 'wipe', name: '遮罩揭示', detail: '文字分段揭开，节奏更鲜明。' },
] as const
const effect = computed(() => options.find(option => option.id === route.query.effect)?.id ?? 'float')
const selected = computed(() => options.find(option => option.id === effect.value)!)
const generation = ref(0)
async function select(id: string) { await router.replace({ query: { effect: id } }); generation.value++; window.scrollTo({ top: 0, behavior: 'instant' }) }
function replay() { generation.value++; window.scrollTo({ top: 0, behavior: 'instant' }) }
</script>

<template>
  <main class="preview-shell min-h-screen bg-[var(--surface-page)] text-[var(--text-primary)]">
    <div class="preview-background" aria-hidden="true"><AboutPointerBackground /></div>
    <header class="preview-controls">
      <div class="control-inner">
        <span class="control-title">关于页 · 下滑效果</span>
        <nav aria-label="预览效果">
          <button v-for="option in options" :key="option.id" type="button" :aria-pressed="effect === option.id" @click="select(option.id)">{{ option.name }}</button>
        </nav>
        <div class="control-actions"><button type="button" @click="replay">重新播放</button><button type="button" @click="toggle">切换明暗</button></div>
      </div>
    </header>
    <aside class="effect-note" aria-live="polite">{{ selected.detail }}向下滚动查看，切换选项可以比较同一份内容。</aside>
    <AboutScrollDemo :key="`${effect}-${generation}`" :effect="effect" />
  </main>
</template>

<style scoped>
.preview-shell { position: relative; isolation: isolate; }
.preview-background { position: fixed; z-index: 0; inset: 0; pointer-events: none; opacity: .55; }
.preview-controls { position: sticky; z-index: 5; top: 0; border-bottom: 1px solid var(--border-default); background: color-mix(in srgb, var(--surface-page) 94%, transparent); backdrop-filter: blur(14px); }
.control-inner { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; max-width: 1060px; margin: auto; padding: 14px 22px; }
.control-title { font-size: 13px; font-weight: 500; }
nav, .control-actions { display: flex; gap: 6px; }
button { padding: 9px 12px; border-radius: 6px; font-size: 12px; color: var(--text-muted); cursor: pointer; }
button[aria-pressed='true'] { background: var(--text-primary); color: var(--surface-page); }
button:focus-visible { outline: 2px solid var(--accent); outline-offset: 3px; }
.effect-note { position: relative; z-index: 1; max-width: 760px; padding: 22px 24px 0; margin: auto; font-size: 12px; line-height: 1.9; color: var(--text-muted); }
@media (max-width: 640px) { .control-inner { padding: 10px 14px; gap: 8px; } .control-title { width: 100%; } button { padding: 8px; } .effect-note { padding-inline: 20px; } }
</style>
