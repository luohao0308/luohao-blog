export const HOME_ENTRANCE_KEY = 'blog:home-entrance:v1'

export interface MotionEnvironment {
  events: EventTarget
  document: EventTarget & { hidden: boolean }
  media: EventTarget & { matches: boolean }
  compact: boolean
  viewportWidth?: number
  memory: { value: boolean }
  storage: () => Pick<Storage, 'getItem' | 'setItem'>
}

export interface MotionController {
  readonly playing: boolean
  stop: () => void
}

type MotionStep = { element: HTMLElement, frames: Keyframe[], duration: number, delay: number }
const resting: Keyframe = { transform: 'perspective(1100px) translate3d(0,0,0) rotateX(0deg) rotateZ(0deg)', opacity: 1 }

// Only WAAPI effects are written. Cancelling them restores the original CSS,
// including any existing inline styles, without guessing which declarations to remove.
function play(steps: MotionStep[], env: MotionEnvironment, changed: (playing: boolean) => void): MotionController {
  const animations: Animation[] = []
  const removers: (() => void)[] = []
  let playing = false
  let pointerActive = false
  let releaseTimer: ReturnType<typeof setTimeout> | undefined
  const stop = () => {
    pointerActive = false
    if (releaseTimer !== undefined) { clearTimeout(releaseTimer); releaseTimer = undefined }
    for (const remove of removers.splice(0)) remove()
    for (const animation of animations.splice(0)) animation.cancel()
    if (playing) { playing = false; changed(false) }
  }
  const listen = (target: EventTarget, type: string, listener: EventListener) => {
    target.addEventListener(type, listener)
    removers.push(() => target.removeEventListener(type, listener))
  }
  try {
    for (const step of steps) {
      const animation = step.element.animate(step.frames, { duration: step.duration, delay: step.delay, easing: 'linear', fill: 'both' })
      animations.push(animation)
      // Attach rejection handlers immediately, including before a later animate() throws.
      void animation.finished.then(() => {
        if (animations.length && animations.every(item => item.playState === 'finished')) stop()
      }, stop)
    }
    playing = animations.length > 0
    if (playing) {
      changed(true)
      void Promise.all(animations.map(animation => animation.finished)).then(stop, stop)
      listen(env.events, 'pointerdown', event => {
        // Keep the hit target in place through native pointerup/click activation.
        // No synthetic navigation or event cancellation: modifiers and focus
        // retain their browser semantics.
        if (!event.target || !steps.some(step => step.element.contains(event.target as Node))) { stop(); return }
        pointerActive = true
        if (releaseTimer !== undefined) { clearTimeout(releaseTimer); releaseTimer = undefined }
        try { for (const animation of animations) animation.pause() } catch { stop() }
      })
      listen(env.events, 'focusin', () => { if (!pointerActive) stop() })
      listen(env.events, 'click', stop)
      listen(env.events, 'auxclick', stop)
      listen(env.events, 'pointerup', () => {
        if (pointerActive) {
          if (releaseTimer !== undefined) clearTimeout(releaseTimer)
          releaseTimer = setTimeout(stop, 0)
        }
      })
      listen(env.events, 'pointercancel', stop)
      listen(env.events, 'resize', stop)
      listen(env.events, 'keydown', event => { if ((event as KeyboardEvent).key === 'Escape') stop() })
      listen(env.document, 'visibilitychange', () => { if (env.document.hidden) stop() })
      listen(env.media, 'change', () => { if (env.media.matches) stop() })
    }
  } catch { stop() }
  return { get playing() { return playing }, stop }
}

function canPlay(elements: HTMLElement[], env: MotionEnvironment) {
  return elements.length > 0 && !env.document.hidden && !env.media.matches && elements.every(element => typeof element.animate === 'function')
}

export function startHomeEntrance(root: HTMLElement, env: MotionEnvironment, changed: (playing: boolean) => void = () => {}): MotionController {
  const shellLayers = [...(root.closest?.('[data-home-shell]')?.querySelectorAll<HTMLElement>('[data-home-shell-layer]') ?? [])]
  const layers = [
    ...shellLayers.filter(element => element.dataset.homeShellLayer === 'nav'),
    ...root.querySelectorAll<HTMLElement>('[data-home-layer]'),
    ...shellLayers.filter(element => element.dataset.homeShellLayer === 'footer'),
  ]
  const cards = [...root.querySelectorAll<HTMLElement>('[data-home-card]')].slice(0, 3)
  const elements = [...layers, ...cards]
  const idle = { playing: false, stop: () => {} }
  if (!canPlay(elements, env) || env.memory.value) return idle
  try { if (env.storage().getItem(HOME_ENTRANCE_KEY)) return idle } catch { /* Client memory remains authoritative when storage is blocked. */ }
  const rectangles = cards.map(element => element.getBoundingClientRect())
  const first = rectangles[0]
  const multiRow = !!first && rectangles.some(rectangle => Math.abs(rectangle.top - first.top) > 1)
  const strength = env.compact ? 0.45 : 1
  const shadow = '0 20px 38px rgb(45 65 95 / 0.16)'
  const clearShadow = '0 0 0 rgb(45 65 95 / 0)'
  const steps: MotionStep[] = layers.map((element, index) => {
    const shell = element.dataset.homeShellLayer
    const subtle = element.dataset.homeLayer === 'subtle'
    const layerStrength = strength * (subtle ? 0.65 : 1)
    const tilt = (shell ? 8 : 10) * layerStrength
    const yaw = (index % 2 ? 2 : -4) * layerStrength
    const depth = (shell === 'nav' ? 24 : shell === 'footer' ? 36 : 52 + Math.min(index, 5) * 10) * strength
    const lift = (shell === 'nav' ? 14 : shell === 'footer' ? 32 : 6 + Math.min(index, 5) * 14) * strength
    const rectangle = element.getBoundingClientRect()
    const width = rectangle.width || rectangle.right - rectangle.left
    const height = rectangle.height || 0
    const radians = Math.PI / 180
    // Bound all corners including perspective. This also protects full-width
    // shell surfaces, whose viewport margin differs from the content root.
    const projectedWidth = width + height * Math.sin(tilt * radians) * Math.sin(Math.abs(yaw) * radians)
    const maxDepth = depth + width / 2 * Math.sin(Math.abs(yaw) * radians) + height / 2 * Math.sin(tilt * radians)
    const availableWidth = env.viewportWidth === undefined ? width : Math.max(0, 2 * Math.min(rectangle.left + width / 2, env.viewportWidth - rectangle.left - width / 2) - 8)
    const parentPerspective = element.parentElement === root
      ? Number.parseFloat(root.ownerDocument?.defaultView?.getComputedStyle(root).perspective ?? '')
      : 0
    const effectivePerspective = parentPerspective > 0 ? 1 / (1 / 1100 + 1 / parentPerspective) : 1100
    const safeScale = projectedWidth > 0 ? availableWidth / projectedWidth * Math.max(0.1, 1 - maxDepth / effectivePerspective) : 1
    const scale = Math.max(0.1, Math.min(shell ? 0.94 : 0.97, safeScale))
    const floating = {
      transform: `perspective(1100px) translate3d(0,${lift}px,${depth}px) rotateX(${tilt}deg) rotateY(${yaw}deg) scale(${scale})`,
      transformOrigin: '50% 50%', borderRadius: shell ? '12px' : '20px',
      boxShadow: subtle ? '0 12px 24px rgb(45 65 95 / 0.08), 0 0 0 1px rgb(100 116 139 / 0.10)' : '0 22px 44px rgb(45 65 95 / 0.10), 0 0 0 1px rgb(100 116 139 / 0.14)', opacity: 1,
    }
    const delay = Math.min(index, 7) * 35
    return {
      element, duration: 2700 - delay, delay,
      frames: [
        { ...resting, transformOrigin: '50% 50%', borderRadius: '0px', boxShadow: clearShadow, offset: 0, easing: 'ease-out' },
        { ...floating, offset: 0.25 },
        { ...floating, offset: 0.55, easing: 'ease-in-out' },
        { ...resting, transformOrigin: '50% 50%', borderRadius: '0px', boxShadow: clearShadow, offset: 0.90 },
        { ...resting, transformOrigin: '50% 50%', borderRadius: '0px', boxShadow: clearShadow, offset: 1 },
      ],
    }
  })
  // Geometry is measured before creating effects, so every translation refers
  // to the real grid rather than a previously animated bounding rectangle.
  const anchorX = first ? (Math.min(...rectangles.map(rectangle => rectangle.left)) + Math.max(...rectangles.map(rectangle => rectangle.right))) / 2 : 0
  const fanScale = env.compact ? 0.62 : multiRow ? 0.72 : 0.92
  const gap = env.compact ? Math.min(36, (first?.width ?? 0) * 0.09) : multiRow ? 120 : 155
  const hero = root.querySelector<HTMLElement>('[data-home-hero]')?.getBoundingClientRect()
  const anchorY = first ? (hero ? hero.bottom + first.height * 0.22 : first.top + first.height / 2) : 0
  cards.forEach((element, index) => {
    const rectangle = rectangles[index]!
    const side = index - (cards.length - 1) / 2
    const x = anchorX + side * gap - (rectangle.left + rectangle.width / 2)
    const y = anchorY - (rectangle.top + rectangle.height / 2) + Math.abs(side) * 12 * strength
    const floating = {
      transform: `perspective(1100px) translate3d(${x}px,${y}px,${(45 + index * 45) * strength}px) rotateX(${10 * strength}deg) rotateZ(${side * 10 * strength}deg) scale(${fanScale})`,
      opacity: 1, boxShadow: shadow,
    }
    steps.push({
      element, duration: 2700, delay: 0,
      frames: [
        { ...resting, boxShadow: clearShadow, offset: 0, easing: 'ease-out' },
        { ...floating, offset: 0.25 },
        { ...floating, offset: 0.55, easing: 'ease-in-out' },
        { ...resting, boxShadow: clearShadow, offset: 0.90 },
        { ...resting, boxShadow: clearShadow, offset: 1 },
      ],
    })
  })
  const controller = play(steps, env, changed)
  if (controller.playing) {
    env.memory.value = true
    try { env.storage().setItem(HOME_ENTRANCE_KEY, '1') } catch { /* Nuxt memory prevents replay in this tab. */ }
  }
  return controller
}

export function startArticleReveal(root: HTMLElement, env: MotionEnvironment): MotionController {
  const elements = [...root.querySelectorAll<HTMLElement>('[data-article-reveal]')].slice(0, 6)
  if (!canPlay(elements, env)) return { playing: false, stop: () => {} }
  return play(elements.map((element, index) => ({
    element, duration: 420, delay: index * 55,
    frames: [
      {
        transform: `perspective(1100px) translate3d(${index % 2 ? 4 : -4}px,${env.compact ? 8 : 14}px,0) rotateX(4deg) rotateZ(${(index % 2 ? 1 : -1) * (env.compact ? 0.8 : 1.5)}deg) scale(0.98)`,
        opacity: 0.5, easing: 'ease-out',
      },
      { transform: 'none', opacity: 1 },
    ],
  })), env, () => {})
}
