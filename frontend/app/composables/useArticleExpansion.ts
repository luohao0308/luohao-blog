import { expandArticleFrom, isArticleExpansionClick } from '~/utils/articleExpansion'

export function useArticleExpansion() {
  const app = useNuxtApp()
  return async function openArticle(event: MouseEvent, to: string) {
    if (!isArticleExpansionClick(event) || !(event.target instanceof Element)) return
    const target = event.target
    if (target.closest('button, input, textarea, select, summary, [contenteditable], [role="button"]')) return
    const link = target.closest('a')
    if (link && link.getAttribute('href') !== to) return
    if (!link && window.getSelection()?.toString()) return
    const card = event.currentTarget as HTMLElement
    event.preventDefault()
    try {
      await expandArticleFrom(card, async () => {
        // Router completion can precede Nuxt's async page/Suspense completion.
        let finish!: () => void
        let fail!: (error: unknown) => void
        const ready = new Promise<void>((resolve, reject) => { finish = resolve; fail = reject })
        // Observe errors immediately, even if navigation has not resolved yet.
        void ready.catch(() => {})
        const unhook = app.hook('page:finish', finish)
        const unhookError = app.hook('app:error', fail)
        try {
          const result = await navigateTo(to)
          if (result === false || result) return
          await ready
          await nextTick()
          window.scrollTo({ top: 0, behavior: 'instant' })
        }
        finally { unhook(); unhookError() }
      })
      document.querySelector<HTMLElement>('[data-article-detail] h1')?.focus({ preventScroll: true })
    }
    catch (error) { showError(error as Error) }
  }
}
