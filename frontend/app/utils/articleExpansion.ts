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
    let skipped = false
    const ready = transition.ready.catch(() => { skipped = true })
    const finished = transition.finished.catch(() => {})
    await transition.updateCallbackDone
    await finished
    await ready
    // Chromium skips native transitions when an async route takes too long.
    // Once the real destination is ready, retain the card-origin reveal rather
    // than silently losing the effect. Hidden tabs and reduced motion stay quiet.
    if (skipped && detail && !doc.hidden && !view.matchMedia('(prefers-reduced-motion: reduce)').matches && html.animate) {
      const reveal = html.animate([
        { clipPath: `polygon(${box.left}px ${box.top}px, ${box.right}px ${box.top}px, ${box.right}px ${box.bottom}px, ${box.left}px ${box.bottom}px)` },
        { clipPath: 'polygon(0px 0px, 100% 0px, 100% 100%, 0px 100%)' },
      ], { duration: 750, easing: 'cubic-bezier(.7, 0, .2, 1)' })
      await reveal.finished.catch(() => {})
    }
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
