<script setup lang="ts">
import type { ArticleHeading } from '~/utils/article'
import { readingAssetHtml } from '~/utils/reading'
const props = defineProps<{ html: string, headings: ArticleHeading[] }>()
const renderedHtml = computed(() => readingAssetHtml(props.html))
const body = ref<HTMLElement | null>(null)
const preview = ref<{ src: string, alt: string } | null>(null)
const contentRoot = ref<HTMLElement | null>(null)
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
  <div ref="contentRoot" class="article-content grid min-w-0 gap-8 lg:grid-cols-[minmax(0,1fr)_14rem] lg:items-start">
    <div v-if="headings.length" class="order-1 lg:order-2 lg:sticky lg:top-24">
      <details class="ui-panel p-4 lg:hidden">
        <summary class="min-h-11 cursor-pointer text-sm font-medium">文章目录</summary>
        <nav aria-label="文章目录">
          <ol class="mt-3 space-y-2 text-sm">
            <li v-for="heading in headings" :key="heading.id" :class="heading.level === 3 ? 'pl-3' : ''"><a :href="`#${heading.id}`" class="ui-link block break-words py-1">{{ heading.text }}</a></li>
          </ol>
        </nav>
      </details>
      <nav aria-label="文章目录" class="ui-panel hidden p-4 lg:block">
        <p class="text-sm font-medium">目录</p>
        <ol class="mt-3 space-y-2 text-sm">
          <li v-for="heading in headings" :key="heading.id" :class="heading.level === 3 ? 'pl-3' : ''"><a :href="`#${heading.id}`" class="ui-link block break-words py-1">{{ heading.text }}</a></li>
        </ol>
      </nav>
    </div>
    <!-- eslint-disable-next-line vue/no-v-html -- trusted backend-rendered HTML, preserving existing heading IDs -->
    <div ref="body" class="markdown-body order-2 min-w-0 space-y-4 lg:order-1" v-html="renderedHtml" />
    <ArticleLightbox v-if="preview" :src="preview.src" :alt="preview.alt" @close="preview = null" />
  </div>
</template>

<style>
.article-content .markdown-body h2, .article-content .markdown-body h3 { scroll-margin-top: var(--article-anchor-offset, 14rem); }
.markdown-body .code-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.25rem 0.75rem; margin-bottom: -1px; border: 1px solid var(--border-default); border-radius: var(--radius-control) var(--radius-control) 0 0; background: var(--surface-panel); font-size: 0.75rem; }
.markdown-body .code-toolbar > span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.markdown-body .code-copy { flex-shrink: 0; min-height: 44px; padding: 0.5rem; color: var(--accent); cursor: pointer; }
.markdown-body .code-copy:disabled { opacity: 0.6; cursor: wait; }
.markdown-body .code-toolbar + pre { margin-top: 0; border-top-left-radius: 0; border-top-right-radius: 0; }
.markdown-body .image-preview-trigger { display: block; max-width: 100%; cursor: zoom-in; border-radius: var(--radius-control); }
</style>
