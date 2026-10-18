// Keep native modified-link actions and nested controls independent of the card.
export function isArticleExpansionClick(event: MouseEvent) {
  return !event.defaultPrevented && event.button === 0
    && !event.metaKey && !event.ctrlKey && !event.shiftKey && !event.altKey
}

export async function expandArticleFrom(card: HTMLElement, navigate: () => Promise<unknown>, detailSelector = '[data-article-detail]') {
  const doc = card.ownerDocument
  const view = doc.defaultView
  if (!view || !doc.startViewTransition || view.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    await navigate()
    return
  }
  if (doc.documentElement.hasAttribute('data-article-expanding')) return
  const box = card.getBoundingClientRect()
  const html = doc.documentElement
  html.style.setProperty('--article-expand-from', `inset(${box.top}px ${view.innerWidth - box.right}px ${view.innerHeight - box.bottom}px ${box.left}px round 24px)`)
  html.setAttribute('data-article-expanding', '')
  let navigationStarted = false
  const previousName = card.style.viewTransitionName
  card.style.viewTransitionName = 'article-content'
  let detail: HTMLElement | null = null
  let detailName = ''
  try {
    const transition = doc.startViewTransition(async () => {
      navigationStarted = true
      await navigate()
      detail = doc.querySelector<HTMLElement>(detailSelector)
      if (detail) { detailName = detail.style.viewTransitionName; detail.style.viewTransitionName = 'article-content' }
    })
    // Skipped animations (e.g. a background tab) must still finish navigation.
    void transition.ready.catch(() => {})
    const finished = transition.finished.catch(() => {})
    await transition.updateCallbackDone
    await finished
  }
  catch (error) {
    if (!navigationStarted) await navigate()
    else throw error
  }
  finally {
    card.style.viewTransitionName = previousName
    if (detail) (detail as HTMLElement).style.viewTransitionName = detailName
    html.removeAttribute('data-article-expanding')
    html.style.removeProperty('--article-expand-from')
  }
}
