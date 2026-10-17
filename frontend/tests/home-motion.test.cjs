const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')
function load() {
  const exports = {}
  vm.runInNewContext(ts.transpileModule(readFileSync(path.resolve(__dirname, '../app/utils/homeMotion.ts'), 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, { exports, Promise, setTimeout, clearTimeout })
  return exports
}
class Events {
  listeners = new Map()
  addEventListener(type, fn) { const set = this.listeners.get(type) || new Set(); set.add(fn); this.listeners.set(type, set) }
  removeEventListener(type, fn) { this.listeners.get(type)?.delete(fn) }
  fire(type, event = {}) { for (const fn of this.listeners.get(type) || []) fn(event) }
  count() { return [...this.listeners.values()].reduce((n, set) => n + set.size, 0) }
}
function fixture(options = {}) {
  const events = new Events(), document = new Events(), media = new Events()
  document.hidden = !!options.hidden; media.matches = !!options.reduce
  const animations = []
  const cards = Array.from({ length: 8 }, (_, index) => ({ dataset: {}, contains: target => target?.cardIndex === index, getBoundingClientRect: () => ({ left: 100 + index * 380, right: 460 + index * 380, top: 750, width: 360, height: 200 }), animate: options.noAnimation ? undefined : (frames, timing) => {
    if (options.throws) throw Error('WAAPI failed')
    let resolve, reject
    const animation = { frames, timing, pause() { this.paused = true }, finished: new Promise((a, b) => { resolve = a; reject = b }), cancel() { this.cancelled = true; reject(Error('cancelled')) }, resolve() { resolve() }, reject() { reject(Error('interrupted')) } }
    animations.push(animation); return animation
  } }))
  const root = { getBoundingClientRect: () => ({ left: 168, right: 1272 }), querySelectorAll: () => cards, querySelector: () => null }
  const memory = { value: false }
  let stored = false
  const env = { events, document, media, memory, compact: false, storage: () => {
    if (options.storageThrows) throw Error('storage blocked')
    return { getItem: () => stored ? '1' : null, setItem: () => { stored = true } }
  } }
  return { root, env, animations, events, document, media, memory }
}
test('storage denial falls back to memory and prevents a second entrance', () => {
  const { startHomeEntrance } = load(), f = fixture({ storageThrows: true })
  const first = startHomeEntrance(f.root, f.env)
  assert.equal(first.playing, true); assert.equal(f.memory.value, true)
  first.stop()
  assert.equal(startHomeEntrance(f.root, f.env).playing, false)
})
test('unsupported, reduced, and background entry neither animates nor consumes session', () => {
  const { startHomeEntrance } = load()
  for (const options of [{ noAnimation: true }, { reduce: true }, { hidden: true }]) {
    const f = fixture(options)
    assert.equal(startHomeEntrance(f.root, f.env).playing, false)
    assert.equal(f.memory.value, false); assert.equal(f.animations.length, 0)
  }
})
test('skip and all interruption signals cancel effects and remove listeners', async () => {
  const { startHomeEntrance } = load()
  for (const signal of ['stop', 'pointerdown', 'focusin', 'resize', 'keydown', 'visibilitychange', 'change']) {
    const f = fixture(), controller = startHomeEntrance(f.root, f.env)
    if (signal === 'stop') controller.stop()
    else if (signal === 'visibilitychange') { f.document.hidden = true; f.document.fire(signal) }
    else if (signal === 'change') { f.media.matches = true; f.media.fire(signal) }
    else f.events.fire(signal, { key: 'Escape' })
    await Promise.resolve()
    assert.equal(controller.playing, false)
    assert.ok(f.animations.every(a => a.cancelled))
    assert.equal(f.events.count() + f.document.count() + f.media.count(), 0)
  }
})
test('synchronous WAAPI failures and rejected finished promises clean up safely', async () => {
  const { startHomeEntrance } = load()
  const failed = fixture({ throws: true })
  assert.equal(startHomeEntrance(failed.root, failed.env).playing, false)
  assert.equal(failed.events.count(), 0)
  const f = fixture(), controller = startHomeEntrance(f.root, f.env)
  f.animations[0].reject()
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(controller.playing, false); assert.ok(f.animations.every(a => a.cancelled))
})
test('normal completion clears effects and article reveal is bounded to six items', async () => {
  const { startArticleReveal } = load(), f = fixture()
  const controller = startArticleReveal(f.root, f.env)
  assert.equal(f.animations.length, 6)
  f.animations.forEach(a => a.resolve())
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(controller.playing, false); assert.ok(f.animations.every(a => a.cancelled))
  assert.equal(f.memory.value, false)
})
test('persisted session suppresses a fresh controller and unrelated keys do not interrupt', () => {
  const { startHomeEntrance } = load(), f = fixture()
  const controller = startHomeEntrance(f.root, f.env)
  f.events.fire('keydown', { key: 'Tab' })
  assert.equal(controller.playing, true)
  controller.stop()
  f.memory.value = false
  assert.equal(startHomeEntrance(f.root, f.env).playing, false)
})
test('a later synchronous animate failure cancels animations already started', async () => {
  const { startHomeEntrance } = load(), f = fixture()
  const original = f.root.querySelectorAll
  f.root.querySelectorAll = () => {
    const elements = original()
    elements[1].animate = () => { throw Error('second element failed') }
    return elements
  }
  assert.equal(startHomeEntrance(f.root, f.env).playing, false)
  await new Promise(resolve => setImmediate(resolve))
  assert.ok(f.animations.every(a => a.cancelled))
  assert.equal(f.memory.value, false)
})

test('home peak holds floating layers and converges real grid cards around one anchor', () => {
  const { startHomeEntrance } = load(), f = fixture()
  startHomeEntrance(f.root, f.env)
  const cards = f.animations.slice(-3)
  const centers = cards.map((animation, index) => {
    const frames = animation.frames
    assert.deepEqual(Array.from(frames, frame => frame.offset), [0, 0.25, 0.55, 0.9, 1])
    assert.equal(frames[1].transform, frames[2].transform)
    assert.equal(frames[0].transform, frames[4].transform)
    assert.equal(animation.timing.easing, 'linear')
    const x = Number(frames[1].transform.match(/translate3d\(([-.\d]+)px/)[1])
    return 280 + index * 380 + x
  })
  assert.deepEqual(centers, [505, 660, 815])
  assert.match(cards[0].frames[1].transform, /rotateZ\(-10deg\)/)
  assert.match(cards[2].frames[1].transform, /rotateZ\(10deg\)/)
  assert.match(f.animations[0].frames[2].transform, /rotateX\(29deg\)/)
  f.events.fire('pointerdown')
  assert.ok(f.animations.every(animation => animation.cancelled))
})
test('home handles layer-only roots without article geometry', () => {
  const { startHomeEntrance } = load(), f = fixture()
  const original = f.root.querySelectorAll
  f.root.querySelectorAll = selector => selector === '[data-home-card]' ? [] : original()
  const controller = startHomeEntrance(f.root, f.env)
  assert.equal(controller.playing, true)
  controller.stop()
})

test('hero still anchors the fan while secondary surfaces join the strong assembly', async () => {
  const { startHomeEntrance } = load(), f = fixture()
  f.root.querySelector = () => ({ getBoundingClientRect: () => ({ bottom: 550 }) })
  f.root.querySelectorAll()[1].dataset.homeLayer = 'subtle'
  const controller = startHomeEntrance(f.root, f.env)
  const card = f.animations.at(-2)
  const y = Number(card.frames[1].transform.match(/translate3d\([-.\d]+px,([-.\d]+)px/)[1])
  assert.equal(850 + y, 550 + 200 * 0.22)
  const hero = f.animations[0], subtle = f.animations[1]
  assert.equal(hero.frames[1].boxShadow, undefined)
  assert.equal(subtle.frames[1].boxShadow, undefined)
  const angle = Number(subtle.frames[1].transform.match(/rotateX\(([-.\d]+)deg/)[1])
  assert.equal(angle, 36)
  assert.match(subtle.frames[4].transform, /translate3d\(0,0,0\)/)
  assert.equal(card.frames[4].transform, card.frames[0].transform)
  controller.stop()
  await Promise.resolve()
  assert.ok(f.animations.every(animation => animation.cancelled))
})

test('two-column wrapped cards center on the complete grid and reduce fan scale', () => {
  const { startHomeEntrance } = load(), f = fixture()
  const elements = f.root.querySelectorAll()
  elements.slice(0, 3).forEach((element, index) => {
    element.getBoundingClientRect = () => ({ left: 24 + (index % 2) * 370, right: 374 + (index % 2) * 370, width: 350, top: 650 + Math.floor(index / 2) * 220, height: 200 })
  })
  const controller = startHomeEntrance(f.root, f.env)
  const centers = f.animations.slice(-3).map((animation, index) => {
    assert.match(animation.frames[1].transform, /scale\(0\.72\)/)
    return 199 + (index % 2) * 370 + Number(animation.frames[1].transform.match(/translate3d\(([-.\d]+)px/)[1])
  })
  assert.deepEqual(centers, [264, 384, 504])
  controller.stop()
})
test('compact fan projected bounds fit inside the source column', () => {
  const { startHomeEntrance } = load()
  for (const viewport of [320, 390]) {
    const f = fixture(); f.env.compact = true
    const width = viewport - 48, left = 24
    f.root.querySelectorAll().slice(0, 3).forEach((element, index) => {
      element.getBoundingClientRect = () => ({ left, right: left + width, width, top: 700 + index * 220, height: 200 })
    })
    const controller = startHomeEntrance(f.root, f.env)
    f.animations.slice(-3).forEach((animation, index) => {
      const transform = animation.frames[1].transform
      assert.match(transform, /scale\(0\.62\)/)
      const x = Number(transform.match(/translate3d\(([-.\d]+)px/)[1])
      // Bound rotated corners and perspective magnification conservatively.
      const angle = Math.abs((index - 1) * 4.5) * Math.PI / 180
      const extent = (width * Math.cos(angle) + 200 * Math.sin(angle)) * 0.62 / 2 * 1.1
      assert.ok(viewport / 2 + x - extent >= 0)
      assert.ok(viewport / 2 + x + extent <= viewport)
    })
    controller.stop()
  }
})

test('narrow viewport edges constrain layers while retaining the desktop fan', () => {
  const { startHomeEntrance } = load()
  for (const viewport of [1024, 1280, 1440]) {
    const f = fixture(), margin = viewport === 1440 ? 168 : 24
    f.env.viewportWidth = viewport
    f.root.getBoundingClientRect = () => ({ left: margin, right: viewport - margin })
    const controller = startHomeEntrance(f.root, f.env)
    assert.ok(Number(f.animations[0].frames[1].transform.match(/scale\(([-.\d]+)\)/)[1]) <= 0.97)
    assert.match(f.animations.at(-1).frames[1].transform, /scale\(0\.92\)/)
    f.events.fire('resize')
    assert.equal(controller.playing, false)
    assert.ok(f.animations.every(animation => animation.cancelled))
    assert.equal(f.events.count(), 0)
  }
})

test('animated pointer gesture freezes through focus and releases only after native click', async () => {
  const { startHomeEntrance } = load(), f = fixture()
  const controller = startHomeEntrance(f.root, f.env)
  const target = { cardIndex: 1 }
  const event = { target, ctrlKey: true, preventDefault() { assert.fail('native event must remain untouched') } }
  f.events.fire('pointerdown', event)
  f.events.fire('focusin', event)
  assert.equal(controller.playing, true)
  assert.ok(f.animations.every(animation => animation.paused && !animation.cancelled))
  f.events.fire('pointerup', event)
  assert.equal(controller.playing, true)
  f.events.fire('click', event)
  assert.equal(controller.playing, false)
  assert.ok(f.animations.every(animation => animation.cancelled))
  assert.equal(f.events.count(), 0)
  await new Promise(resolve => setTimeout(resolve, 5))
  assert.equal(f.events.count(), 0)
})
test('gesture without click has a release fallback and cancel removes its timer', async () => {
  const { startHomeEntrance } = load()
  for (const cancellation of [null, 'pointercancel', 'resize', 'keydown']) {
    const f = fixture(), controller = startHomeEntrance(f.root, f.env)
    f.events.fire('pointerdown', { target: { cardIndex: 0 } })
    f.events.fire('pointerup')
    if (cancellation) {
      f.events.fire(cancellation, { key: 'Escape' })
      assert.equal(controller.playing, false)
    }
    await new Promise(resolve => setTimeout(resolve, 5))
    assert.equal(controller.playing, false)
    assert.ok(f.animations.every(animation => animation.cancelled))
    assert.equal(f.events.count() + f.document.count() + f.media.count(), 0)
  }
})

test('list fan touches only selected items with bounded alternating motion and restores on cancellation', () => {
  const { startArticleReveal } = load()
  for (const compact of [false, true]) {
    const f = fixture(); f.env.compact = compact
    const elements = f.root.querySelectorAll()
    const originalTransforms = elements.map((element, index) => {
      element.style = { transform: `translateX(${index}px)`, opacity: '1' }
      return { ...element.style }
    })
    f.root.querySelectorAll = selector => {
      assert.equal(selector, '[data-article-reveal]')
      return elements
    }
    f.env.storage = () => { assert.fail('list animation must not read or consume home session') }
    const controller = startArticleReveal(f.root, f.env)
    assert.equal(f.animations.length, 6)
    let previousAngle
    f.animations.forEach((animation, index) => {
      const start = animation.frames[0].transform
      const angle = Number(start.match(/rotateZ\(([-.\d]+)deg/)[1])
      const x = Number(start.match(/translate3d\(([-.\d]+)px/)[1])
      const y = Number(start.match(/translate3d\([-.\d]+px,([-.\d]+)px/)[1])
      assert.ok(Math.abs(angle) > 0 && Math.abs(angle) <= (compact ? 1 : 2))
      assert.ok(Math.abs(x) <= 4 && y <= 14 && y > 0)
      if (previousAngle !== undefined) assert.ok(previousAngle * angle < 0)
      previousAngle = angle
      assert.equal(animation.frames.at(-1).transform, 'none')
      assert.equal(animation.frames.at(-1).opacity, 1)
      assert.ok(animation.timing.duration + animation.timing.delay < 750)
      assert.deepEqual(elements[index].style, originalTransforms[index])
    })
    f.events.fire('keydown', { key: 'Escape' })
    assert.equal(controller.playing, false)
    assert.ok(f.animations.every(animation => animation.cancelled))
    elements.forEach((element, index) => assert.deepEqual(element.style, originalTransforms[index]))
    assert.equal(f.memory.value, false)
    assert.equal(f.events.count() + f.document.count() + f.media.count(), 0)
  }
})


test('shell layers bracket content with separate depth, lift and stagger and cancel together', () => {
  const { startHomeEntrance } = load(), f = fixture()
  const content = f.root.querySelectorAll()
  const nav = { ...content[0], dataset: { homeShellLayer: 'nav' } }
  const footer = { ...content[0], dataset: { homeShellLayer: 'footer' } }
  f.root.closest = () => ({ querySelectorAll: () => [footer, nav] })
  const controller = startHomeEntrance(f.root, f.env)
  const layers = f.animations.slice(0, -3)
  assert.equal(layers.length, content.length + 2)
  assert.ok(Number(layers[0].frames[1].transform.match(/scale\(([-.\d]+)\)/)[1]) <= 0.94)
  const vectors = layers.map(a => a.frames[1].transform.match(/translate3d\(0,([-.\d]+)px,([-.\d]+)px\)/))
  assert.ok(new Set(vectors.map(vector => vector[1])).size > 3)
  assert.ok(new Set(vectors.map(vector => vector[2])).size > 3)
  assert.match(layers[0].frames[1].transform, /84px\) rotateX\(36deg\)/)
  assert.match(layers.at(-1).frames[1].transform, /rotateX\(36deg\)/)
  assert.ok(new Set(layers.map(a => a.timing.delay)).size > 3)
  for (const layer of layers) {
    assert.ok(Math.abs(layer.timing.duration + layer.timing.delay - 3100 / 1.75) < 0.001)
    assert.match(layer.frames.at(-1).transform, /translate3d\(0,0,0\)/)
    assert.equal(layer.frames[1].transformOrigin, undefined, 'origin is a static surface style')
  }
  f.events.fire('resize')
  assert.equal(controller.playing, false)
  assert.ok(f.animations.every(a => a.cancelled))
})

test('assembly restores the approved large unfold and travelling perspective on desktop and mobile', () => {
  const { startHomeEntrance } = load()
  for (const compact of [false, true]) {
    const f = fixture(); f.env.compact = compact
    const controller = startHomeEntrance(f.root, f.env)
    const layer = f.animations[0]
    const angle = frame => Number(frame.transform.match(/rotateX\(([-.\d]+)deg/)[1])
    assert.ok(angle(layer.frames[0]) >= (compact ? 38 : 54), 'visible unfold from depth')
    assert.match(layer.frames[0].transform, /-420px/)
    assert.ok(angle(layer.frames[1]) >= (compact ? 25 : 36), 'strong floating presentation')
    assert.ok(angle(layer.frames[2]) < angle(layer.frames[1]), 'perspective travels during the hold')
    assert.notEqual(layer.frames[1].transform, layer.frames[2].transform)
    assert.equal(layer.frames[0].opacity, 1, 'SSR content stays visible')
    assert.equal(layer.frames.at(-1).opacity, 1)
    controller.stop()
    assert.ok(f.animations.every(a => a.cancelled))
  }
})

test('assembly uses the original continuous easing without restarting it at phase boundaries', () => {
  const { startHomeEntrance } = load(), f = fixture()
  const controller = startHomeEntrance(f.root, f.env)
  const layers = f.animations.slice(0, -3)
  for (const layer of layers) {
    assert.equal(layer.timing.easing, 'cubic-bezier(0.25, 0.55, 0.25, 1)')
    assert.ok(layer.frames.every(frame => frame.easing === undefined))
    const operations = frame => Array.from(frame.transform.matchAll(/([a-zA-Z0-9]+)\(/g), match => match[1])
    assert.deepEqual(operations(layer.frames.at(-1)), operations(layer.frames[0]), 'consistent transform list through the landing')
  }
  assert.ok(f.animations.slice(-3).every(card => card.timing.easing === 'linear'), 'fan keeps its existing easing')
  controller.stop()
})

test('home keyframes avoid paint properties and temporary surfaces restore on every exit', async () => {
  const { startHomeEntrance } = load()
  for (const reason of ['stop', 'resize', 'finish', 'reject']) {
    const f = fixture(), elements = f.root.querySelectorAll()
    elements[0].dataset.homeMotionSurface = 'previous'
    const controller = startHomeEntrance(f.root, f.env)
    for (const animation of f.animations) {
      assert.ok(animation.frames.every(frame => Object.keys(frame).every(key => ['transform', 'opacity', 'offset', 'easing'].includes(key))))
    }
    assert.equal(elements[0].dataset.homeMotionSurface, 'card')
    if (reason === 'stop') controller.stop()
    if (reason === 'resize') f.events.fire('resize')
    if (reason === 'finish') f.animations.forEach(animation => animation.resolve())
    if (reason === 'reject') f.animations[0].reject()
    await new Promise(resolve => setImmediate(resolve))
    assert.equal(elements[0].dataset.homeMotionSurface, 'previous')
    assert.ok(elements.slice(1).every(element => element.dataset.homeMotionSurface === undefined))
  }
})


test('article fan preserves its approved trajectory and shares the faster home playback rate', () => {
  const { startHomeEntrance } = load(), f = fixture()
  const controller = startHomeEntrance(f.root, f.env)
  const cards = f.animations.slice(-3)
  cards.forEach((animation, index) => {
    assert.deepEqual(JSON.parse(JSON.stringify(animation.timing)), { duration: 2700 / 1.75, delay: 0, easing: 'linear', fill: 'both' })
    const side = index - 1
    const expected = `perspective(1100px) translate3d(${225 - index * 225}px,${Math.abs(side) * 12}px,${45 + index * 45}px) rotateX(10deg) rotateZ(${side * 10}deg) scale(0.92)`
    assert.equal(animation.frames[1].transform, expected)
    assert.equal(animation.frames[2].transform, expected)
    assert.deepEqual(Array.from(animation.frames, frame => frame.offset), [0, 0.25, 0.55, 0.9, 1])
  })
  controller.stop()
})


test('direct root layers account for parent perspective and navigation floats below the top edge', () => {
  const { startHomeEntrance } = load(), f = fixture()
  const content = f.root.querySelectorAll()
  f.env.viewportWidth = 768
  f.root.ownerDocument = { defaultView: { getComputedStyle: () => ({ perspective: '1600px' }) } }
  content[0].parentElement = f.root
  content[0].getBoundingClientRect = () => ({ left: 24, right: 744, width: 720, height: 400, top: 100 })
  const nav = { ...content[1], dataset: { homeShellLayer: 'nav' } }
  f.root.closest = () => ({ querySelectorAll: () => [nav] })
  const controller = startHomeEntrance(f.root, f.env)
  const navLift = Number(f.animations[0].frames[1].transform.match(/translate3d\(0,([-.\d]+)px/)[1])
  assert.ok(navLift >= 12)
  const transform = f.animations[1].frames[1].transform
  const scale = Number(transform.match(/scale\(([-.\d]+)\)/)[1])
  const radians = Math.PI / 180
  const pitch = Math.sin(36 * radians), turn = Math.sin(11 * radians), bank = Math.sin(4 * radians)
  const maxDepth = 42 + 360 * (turn + bank * pitch) + 200 * (pitch + bank * turn)
  const projectedWidth = 720 + 400 * (bank + pitch * turn)
  const effectivePerspective = 1 / (1 / 1100 + 1 / 1600)
  assert.ok(scale * projectedWidth / (1 - maxDepth / effectivePerspective) <= 760.001)
  controller.stop()
})
