import { createReferenceAssembly } from './referenceAssembly'

export const HOME_ENTRANCE_KEY = 'blog:home-entrance:v1'
const HOME_PLAYBACK_RATE = 1.75

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

type MotionStep = { element: HTMLElement, frames: Keyframe[], duration: number, delay: number, easing?: string, surface?: 'layer' | 'card' }
const resting: Keyframe = { transform: 'perspective(1100px) translate3d(0,0,0) rotateX(0deg) rotateZ(0deg)', opacity: 1 }

// Effects and their temporary presentation share one cleanup path. Existing
// inline styles on real content are never overwritten.
function play(steps: MotionStep[], env: MotionEnvironment, changed: (playing: boolean) => void, release?: () => void): MotionController {
  const animations: Animation[] = []
  const removers: (() => void)[] = []
  let playing = false
  let pointerActive = false
  let releaseTimer: ReturnType<typeof setTimeout> | undefined
  const stop = () => {
    pointerActive = false
    if (releaseTimer !== undefined) { clearTimeout(releaseTimer); releaseTimer = undefined }
    for (const remove of removers.splice(0).reverse()) remove()
    for (const animation of animations.splice(0)) animation.cancel()
    const cleanup = release
    release = undefined
    cleanup?.()
    if (playing) { playing = false; changed(false) }
  }
  const listen = (target: EventTarget, type: string, listener: EventListener) => {
    target.addEventListener(type, listener)
    removers.push(() => target.removeEventListener(type, listener))
  }
  try {
    for (const step of steps) {
      if (step.surface) {
        const previous = step.element.dataset.homeMotionSurface
        step.element.dataset.homeMotionSurface = step.surface
        removers.push(() => {
          if (previous === undefined) delete step.element.dataset.homeMotionSurface
          else step.element.dataset.homeMotionSurface = previous
        })
      }
      const animation = step.element.animate(step.frames, { duration: step.duration, delay: step.delay, easing: step.easing ?? 'linear', fill: 'both' })
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
      listen(env.events, 'focusin', event => {
        const target = event.target as HTMLElement | null
        if (!pointerActive && !target?.closest?.('[data-home-skip]')) stop()
      })
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
  const idle = { playing: false, stop: () => {} }
  if (!canPlay([root], env) || env.memory.value) return idle
  try { if (env.storage().getItem(HOME_ENTRANCE_KEY)) return idle } catch { /* Client memory remains the fallback. */ }
  let scene: ReturnType<typeof createReferenceAssembly> | undefined
  try {
    scene = createReferenceAssembly(root, env.viewportWidth ?? root.ownerDocument.defaultView!.innerWidth)
    const controller = play(scene.steps, env, changed, scene.cleanup)
    if (controller.playing) {
      env.memory.value = true
      try { env.storage().setItem(HOME_ENTRANCE_KEY, '1') } catch { /* SPA memory prevents replay. */ }
    }
    return controller
  } catch { scene?.cleanup(); return idle }
}

// Keep the previous variant available for its frozen regression coverage.
// The default entrance above assembles the actual page elements.
export function startHybridEntrance(root: HTMLElement, env: MotionEnvironment, changed: (playing: boolean) => void = () => {}): MotionController {
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
  const steps: MotionStep[] = layers.map((element, index) => {
    const shell = element.dataset.homeShellLayer
    // Layer presentation follows the larger 04 study. Card strength and its
    // timeline below remain independent, including on compact screens.
    const layerStrength = env.compact ? 0.72 : 1
    const spread = env.compact ? 32 : 62
    const depth = (2 - index) * 42
    const lift = shell === 'nav' ? 38 : 52 + (index - 1) * spread
    const rectangle = element.getBoundingClientRect()
    const width = rectangle.width || rectangle.right - rectangle.left
    const height = rectangle.height || 0
    const radians = Math.PI / 180
    const availableWidth = env.viewportWidth === undefined ? width : Math.max(0, 2 * Math.min(rectangle.left + width / 2, env.viewportWidth - rectangle.left - width / 2) - 8)
    const parentPerspective = element.parentElement === root
      ? Number.parseFloat(root.ownerDocument?.defaultView?.getComputedStyle(root).perspective ?? '')
      : 0
    const effectivePerspective = parentPerspective > 0 ? 1 / (1 / 1100 + 1 / parentPerspective) : 1100
    const pose = (tilt: number, yaw: number, roll: number, z: number, y: number, preferredScale: number): Keyframe => {
      const pitch = Math.sin(tilt * radians)
      const turn = Math.abs(Math.sin(yaw * radians))
      const bank = Math.abs(Math.sin(roll * radians))
      // Conservative corner bounds cover roll as well as both perspectives.
      // Each pose is bounded separately; depth arrival needs a different scale.
      const projectedWidth = width + height * (bank + pitch * turn)
      const maxDepth = z + width / 2 * (turn + bank * pitch) + height / 2 * (pitch + bank * turn)
      const safeScale = projectedWidth > 0 ? availableWidth / projectedWidth * Math.max(0.08, 1 - maxDepth / effectivePerspective) : 1
      const scale = Math.max(0.08, Math.min(preferredScale, safeScale))
      return {
        transform: `perspective(1100px) translate3d(0,${y}px,${z}px) rotateX(${tilt}deg) rotateY(${yaw}deg) rotateZ(${roll}deg) scale(${scale})`,
        opacity: 1,
      }
    }
    const arriving = pose(54 * layerStrength, -15 * layerStrength, -5 * layerStrength, -420, (lift + 60) * layerStrength, 0.72)
    const opened = pose(36 * layerStrength, -11 * layerStrength, -4 * layerStrength, depth * layerStrength, lift * layerStrength, 0.86)
    const travelling = pose(29 * layerStrength, 8 * layerStrength, 2 * layerStrength, depth * layerStrength, lift * layerStrength, 0.88)
    const delay = Math.min(index, 7) * 65 / HOME_PLAYBACK_RATE
    const settled: Keyframe = {
      transform: 'perspective(1100px) translate3d(0,0,0) rotateX(0deg) rotateY(0deg) rotateZ(0deg) scale(1)',
      opacity: 1,
    }
    return {
      element, duration: 3100 / HOME_PLAYBACK_RATE - delay, delay,
      surface: 'layer',
      easing: 'cubic-bezier(0.25, 0.55, 0.25, 1)',
      frames: [
        { ...arriving, offset: 0 },
        { ...opened, offset: 0.24 },
        { ...travelling, offset: 0.60 },
        { ...settled, offset: 0.92 },
        { ...settled, offset: 1 },
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
      opacity: 1,
    }
    steps.push({
      element, duration: 2700 / HOME_PLAYBACK_RATE, delay: 0,
      surface: 'card',
      frames: [
        { ...resting, offset: 0, easing: 'ease-out' },
        { ...floating, offset: 0.25 },
        { ...floating, offset: 0.55, easing: 'ease-in-out' },
        { ...resting, offset: 0.90 },
        { ...resting, offset: 1 },
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
