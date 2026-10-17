import { ASSEMBLY_REFERENCE_HTML } from './assemblyReference'

export interface AssemblyStep {
  element: HTMLElement
  frames: Keyframe[]
  duration: number
  delay: number
  easing: string
}

// These are the original white-layers timeline values, without a speed factor,
// per-panel perspective, geometry-dependent scaling or mobile angle reduction.
export function referenceLayerFrames(index: number, compact: boolean): Keyframe[] {
  const spread = (index - 2) * (compact ? 26 : 55)
  const depth = (2 - index) * 40
  return [
    { opacity: 0, transform: `translate3d(${index % 2 ? 90 : -90}px,${spread + 60}px,-420px) rotateX(54deg) rotateY(-15deg) rotateZ(-5deg) scale(.72)` },
    { offset: 0.24, opacity: 1, transform: `translate3d(0,${spread}px,${depth}px) rotateX(36deg) rotateY(-11deg) rotateZ(-4deg) scale(.86)` },
    { offset: 0.60, opacity: 1, transform: `translate3d(0,${spread}px,${depth}px) rotateX(29deg) rotateY(8deg) rotateZ(2deg) scale(.88)` },
    { offset: 0.92, opacity: 1, transform: 'none' },
    { opacity: 1, transform: 'none' },
  ]
}

export function prepareReferenceLayers(stage: HTMLElement, viewportWidth: number) {
  const screen = stage.querySelector<HTMLElement>('.screen')!
  const host = stage.querySelector<HTMLElement>('.layer-host')!
  const wrap = stage.querySelector<HTMLElement>('.screen-wrap')!
  const originalAnimating = wrap.dataset.animating
  const doc = stage.ownerDocument
  const wasAdStage = stage.classList.contains('ad-stage')
  const originalOpacity = screen.style.opacity
  const originalInert = screen.inert
  const created: HTMLElement[] = []
  const cleanup = () => {
    for (const piece of created.splice(0)) piece.remove()
    screen.style.opacity = originalOpacity
    screen.inert = originalInert
    if (originalAnimating === undefined) delete wrap.dataset.animating
    else wrap.dataset.animating = originalAnimating
    if (!wasAdStage) stage.classList.remove('ad-stage')
  }
  try {
    stage.classList.add('ad-stage')
    wrap.dataset.animating = 'true'
    const width = screen.offsetWidth, height = screen.offsetHeight
    const root = screen.getBoundingClientRect()
    const specs = [
      ['.chrome', 'FRAME'], ['.site-header', 'NAVIGATION'], ['.hero', 'INTRO + VISUAL'],
      ['.notes', 'CONTENT'], ['.screen-footer', 'DETAILS'],
    ] as const
    const steps: AssemblyStep[] = []
    screen.style.opacity = '0'
    screen.inert = true
    specs.forEach(([selector, label], index) => {
      const box = screen.querySelector<HTMLElement>(selector)!.getBoundingClientRect()
      const x = box.left - root.left, y = box.top - root.top
      const piece = doc.createElement('div'), clip = doc.createElement('div')
      const inner = doc.createElement('div'), tag = doc.createElement('span')
      piece.className = 'ad-piece'; clip.className = 'ad-clip'
      inner.className = 'tile-inner'; tag.className = 'ad-tag'; tag.textContent = label
      Object.assign(piece.style, { left: `${x}px`, top: `${y}px`, width: `${box.width}px`, height: `${box.height}px` })
      Object.assign(inner.style, { width: `${width}px`, height: `${height}px`, transform: `translate(${-x}px,${-y}px)` })
      const snapshot = screen.cloneNode(true) as HTMLElement
      snapshot.removeAttribute('id')
      Object.assign(snapshot.style, { opacity: '1', transform: 'none', filter: 'none' })
      snapshot.inert = true; snapshot.setAttribute('aria-hidden', 'true')
      for (const element of snapshot.querySelectorAll('[id]')) element.removeAttribute('id')
      inner.append(snapshot); clip.append(inner); piece.append(clip, tag); host.append(piece); created.push(piece)
      steps.push({ element: piece, frames: referenceLayerFrames(index, viewportWidth < 600), duration: 2900, delay: index * 65, easing: 'cubic-bezier(.25,.55,.25,1)' })
      steps.push({ element: tag, frames: [
        { opacity: 0 }, { offset: 0.25, opacity: 1 }, { offset: 0.63, opacity: 1 },
        { offset: 0.83, opacity: 0 }, { opacity: 0 },
      ], duration: 2900, delay: index * 65, easing: 'linear' })
    })
    return { steps, cleanup }
  } catch (error) { cleanup(); throw error }
}

export function referenceFanFrames(index: number, compact: boolean): Keyframe[] {
  const x = (index - 1) * (compact ? 38 : 160), angle = (index - 1) * 11
  return [
    { opacity: 0, transform: 'translate3d(-50%,-25%,-90px) rotateX(35deg) scale(.7)' },
    { offset: 0.28, opacity: 1, transform: `translate3d(calc(-50% + ${x}px),-62%,${140 + index * 45}px) rotateX(12deg) rotateZ(${angle}deg) scale(${compact ? 0.9 : 1})` },
    { offset: 0.57, opacity: 1, transform: `translate3d(calc(-50% + ${x}px),-65%,${140 + index * 45}px) rotateX(6deg) rotateZ(${angle}deg)` },
    { offset: 0.88, opacity: 1, transform: `translate3d(calc(-50% + ${x * 0.45}px),-50%,0) rotateX(0deg) rotateZ(0deg) scale(.8)` },
    { opacity: 0, transform: `translate3d(calc(-50% + ${x * 0.45}px),-50%,-15px) scale(.75)` },
  ]
}

function prepareReferenceFan(stage: HTMLElement, root: HTMLElement, viewportWidth: number) {
  const doc = stage.ownerDocument, screen = stage.querySelector<HTMLElement>('.screen')!
  const notes = screen.querySelector<HTMLElement>('.notes')!.getBoundingClientRect()
  const screenBox = screen.getBoundingClientRect()
  const centerY = notes.top - screenBox.top + Math.min(notes.height / 2, 80)
  const compact = viewportWidth < 600
  const host = stage.querySelector<HTMLElement>('.layer-host')!
  const created: HTMLElement[] = [], steps: AssemblyStep[] = []
  const cleanup = () => { for (const card of created.splice(0)) card.remove() }
  try {
    const articles = [...root.querySelectorAll<HTMLElement>('[data-home-card]')].slice(0, 3)
    articles.forEach((article, index) => {
      const card = doc.createElement('div'), label = doc.createElement('div')
      const title = doc.createElement('h3'), tags = doc.createElement('div')
      card.className = 'assembly-fan-card'; label.className = 'fan-label'; tags.className = 'fan-tags'
      card.inert = true; card.setAttribute('aria-hidden', 'true')
      Object.assign(card.style, { top: `${centerY}px`, width: compact ? '190px' : '260px', height: compact ? '138px' : '156px', padding: compact ? '14px' : '18px' })
      label.textContent = `WRITING / 0${index + 1}`
      title.textContent = article.querySelector('h2')?.textContent?.trim() ?? ''
      title.style.fontSize = compact ? '13px' : '15px'
      tags.textContent = [...article.querySelectorAll('a[href*="/tags/"]')].map(tag => tag.textContent?.trim()).filter(Boolean).join(' · ')
      card.append(label, title, tags); host.append(card); created.push(card)
      steps.push({ element: card, frames: referenceFanFrames(index, compact), duration: 1900, delay: 530 + index * 65, easing: 'linear' })
    })
    return { steps, cleanup }
  } catch (error) { cleanup(); throw error }
}

export function createReferenceAssembly(root: HTMLElement, viewportWidth: number) {
  const doc = root.ownerDocument
  const shell = root.closest<HTMLElement>('[data-home-shell]') ?? root
  const overlay = doc.createElement('div')
  overlay.className = 'home-assembly-overlay'
  overlay.setAttribute('aria-hidden', 'true'); overlay.inert = true
  // Keep the approved presentation artwork unchanged. Real article titles are
  // rendered in an independent fan through textContent, never user HTML.
  overlay.innerHTML = ASSEMBLY_REFERENCE_HTML
  const targets = [shell.querySelector<HTMLElement>('header'), shell.querySelector<HTMLElement>('main'), shell.querySelector<HTMLElement>('footer')].filter((element): element is HTMLElement => !!element)
  const inertStates = targets.map(element => element.inert)
  let presentation: ReturnType<typeof prepareReferenceLayers> | undefined
  let fan: ReturnType<typeof prepareReferenceFan> | undefined
  let cleaned = false
  const cleanup = () => {
    if (cleaned) return
    cleaned = true
    fan?.cleanup(); presentation?.cleanup(); overlay.remove()
    targets.forEach((element, index) => { element.inert = inertStates[index]! })
  }
  try {
    doc.body.append(overlay)
    presentation = prepareReferenceLayers(overlay.querySelector<HTMLElement>('.stage')!, viewportWidth)
    fan = prepareReferenceFan(overlay.querySelector<HTMLElement>('.stage')!, root, viewportWidth)
    targets.forEach(element => { element.inert = true })
    return { steps: [...presentation.steps, ...fan.steps], cleanup }
  } catch (error) { cleanup(); throw error }
}
