<script setup lang="ts">
import type { ArticleHeading } from '~/utils/article'
import { readingAssetHtml } from '~/utils/reading'
const props = defineProps<{ html: string, headings: ArticleHeading[] }>()
const renderedHtml = computed(() => readingAssetHtml(props.html))
const body = ref<HTMLElement | null>(null)
const preview = ref<{ src: string, alt: string } | null>(null)
const contentRoot = ref<HTMLElement | null>(null)
const tocPopover = ref<HTMLElement | null>(null)
function closeToc() { tocPopover.value?.hidePopover() }
let headerObserver: ResizeObserver | undefined
let cleanups: Array<() => void> = []
function cleanup() {
  cleanups.forEach(fn => fn())
  cleanups = []
  preview.value = null
}
function enhance() {
  if (!body.value) return
  for (const pre of body.value.querySelectorAll('pre')) {
    const code = pre.querySelector('code')
    if (!code) continue
    const toolbar = document.createElement('div')
    toolbar.className = 'code-toolbar'
    const language = document.createElement('span')
    language.textContent = code.className.match(/language-([\w+-]+)/)?.[1] || code.getAttribute('data-lang') || code.closest('[data-code-language]')?.getAttribute('data-code-language') || '代码'
    const button = document.createElement('button')
    button.type = 'button'
    button.className = 'code-copy'
    button.textContent = '复制代码'
    button.setAttribute('aria-live', 'polite')
    toolbar.append(language, button)
    pre.before(toolbar)
    let timer: ReturnType<typeof setTimeout> | undefined
    let active = true
    button.onclick = async () => {
      clearTimeout(timer)
      button.disabled = true
      try {
        await navigator.clipboard.writeText(code.textContent ?? '')
        if (active) button.textContent = '已复制'
      }
      catch { if (active) button.textContent = '复制失败，重试' }
      finally {
        if (active) {
          button.disabled = false
          timer = setTimeout(() => { button.textContent = '复制代码' }, 3000)
        }
      }
    }
    cleanups.push(() => { active = false; clearTimeout(timer); toolbar.remove() })
  }
  for (const image of body.value.querySelectorAll('img')) {
    // Authored linked images retain their navigation behavior.
    if (image.closest('a, button')) continue
    const button = document.createElement('button')
    button.type = 'button'
    button.className = 'image-preview-trigger'
    button.setAttribute('aria-label', `放大图片：${image.alt || '正文图片'}`)
    image.before(button)
    button.append(image)
    button.onclick = () => { preview.value = { src: image.currentSrc || image.src, alt: image.alt } }
    cleanups.push(() => { button.before(image); button.remove() })
  }
}
onMounted(() => {
  enhance()
  const header = document.querySelector('.site-header')
  if (header) {
    const updateOffset = () => contentRoot.value?.style.setProperty('--article-anchor-offset', `${header.getBoundingClientRect().height + 20}px`)
    updateOffset()
    headerObserver = new ResizeObserver(updateOffset)
    headerObserver.observe(header)
  }
})
watch(() => props.html, async () => { cleanup(); await nextTick(); enhance() }, { flush: 'pre' })
onBeforeUnmount(() => { cleanup(); headerObserver?.disconnect() })
</script>

<template>
  <div ref="contentRoot" class="article-content flex min-w-0 flex-col gap-8">
    <template v-if="headings.length">
      <button type="button" popovertarget="article-toc-popover" class="article-toc-toggle rounded-full border border-[var(--border-default)] bg-[var(--surface-panel)] px-4 py-3 text-sm shadow-md" aria-label="打开文章目录">目录</button>
      <nav id="article-toc-popover" ref="tocPopover" popover class="article-toc-popover rounded-xl border border-[var(--border-default)] bg-[var(--surface-panel)] p-4 shadow-lg" aria-label="浮动文章目录">
        <div class="flex items-center justify-between gap-4">
          <p class="text-sm font-medium">文章目录</p>
          <button type="button" class="min-h-11 px-2 text-sm" @click="closeToc">关闭</button>
        </div>
        <ol class="space-y-2 text-sm">
          <li v-for="heading in headings" :key="heading.id" :class="heading.level === 3 ? 'pl-3' : ''"><a :href="`#${heading.id}`" class="ui-link block break-words py-2" @click="closeToc">{{ heading.text }}</a></li>
        </ol>
      </nav>
      <nav aria-label="文章目录" class="article-toc-docked rounded-xl border border-[var(--border-default)] bg-[var(--surface-panel)] p-4 shadow-sm">
        <p class="text-sm font-medium">目录</p>
        <ol class="mt-3 space-y-2 text-sm">
          <li v-for="heading in headings" :key="heading.id" :class="heading.level === 3 ? 'pl-3' : ''"><a :href="`#${heading.id}`" class="ui-link block break-words py-1">{{ heading.text }}</a></li>
        </ol>
      </nav>
    </template>
    <!-- eslint-disable-next-line vue/no-v-html -- trusted backend-rendered HTML, preserving existing heading IDs -->
    <div ref="body" class="markdown-body min-w-0 space-y-4" v-html="renderedHtml" />
    <ArticleLightbox v-if="preview" :src="preview.src" :alt="preview.alt" @close="preview = null" />
  </div>
</template>

<style>
/* Native hash navigation retains URL/history and honors heading scroll margins. */
html:has(.article-content) { scroll-behavior: smooth; }
@media (prefers-reduced-motion: reduce) {
  html:has(.article-content) { scroll-behavior: auto !important; }
}
.article-toc-docked a, .article-toc-popover a { -webkit-tap-highlight-color: transparent; }

.article-toc-docked { display: none; }
.article-toc-toggle { position: fixed; bottom: max(1.5rem, env(safe-area-inset-bottom)); left: 1.5rem; z-index: 30; }
.article-toc-popover {
  position: fixed;
  inset: auto auto max(5rem, calc(env(safe-area-inset-bottom) + 4rem)) 1rem;
  margin: 0;
  width: min(20rem, calc(100vw - 2rem));
  max-height: calc(100dvh - var(--article-anchor-offset, 8rem) - 6rem);
  overflow-y: auto;
  color: var(--text-primary);
}
/* An 860px reading column leaves room for a separate 192px directory at 1366px. */
@media (min-width: 1360px) {
  .article-toc-toggle, .article-toc-popover:popover-open { display: none; }
  .article-content .article-toc-docked {
    display: block;
    position: fixed;
    top: var(--article-anchor-offset, 6rem);
    right: calc((100vw - 860px) / 2 - 216px);
    width: 192px;
    max-height: calc(100dvh - var(--article-anchor-offset, 6rem) - 2rem);
    overflow-y: auto;
  }
}

.article-content .markdown-body h2, .article-content .markdown-body h3 { scroll-margin-top: var(--article-anchor-offset, 14rem); }
.markdown-body .code-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.25rem 0.75rem; margin-bottom: -1px; border: 1px solid var(--border-default); border-radius: var(--radius-control) var(--radius-control) 0 0; background: var(--surface-panel); font-size: 0.75rem; }
.markdown-body .code-toolbar > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.markdown-body .code-copy { flex-shrink: 0; min-height: 44px; padding: 0.5rem; color: var(--accent); cursor: pointer; }
.markdown-body .code-copy:disabled { opacity: 0.6; cursor: wait; }
.markdown-body .code-toolbar + pre { margin-top: 0; border-top-left-radius: 0; border-top-right-radius: 0; }
.markdown-body .image-preview-trigger { display: block; max-width: 100%; cursor: zoom-in; border-radius: var(--radius-control); }
</style>
