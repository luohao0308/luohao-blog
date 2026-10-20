const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')

function compile(source) {
  return ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
}
const painters = {}
vm.runInNewContext(compile(readFileSync(path.resolve(__dirname, '../app/utils/aboutPointerBackground.ts'), 'utf8')), { exports: painters })
class Events {
  listeners = new Map()
  addEventListener(type, fn) { const list = this.listeners.get(type) || []; list.push(fn); this.listeners.set(type, list) }
  removeEventListener(type, fn) { this.listeners.set(type, (this.listeners.get(type) || []).filter(item => item !== fn)) }
  fire(type, event = {}) { for (const fn of [...(this.listeners.get(type) || [])]) fn(event) }
  count() { return [...this.listeners.values()].reduce((sum, list) => sum + list.length, 0) }
}
function drawingContext() {
  const calls = []
  const check = (method, args) => {
    for (const value of args) if (typeof value === 'number') assert.ok(Number.isFinite(value), `${method} received ${value}`)
    calls.push(method)
  }
  const context = new Proxy({}, { get: (_, method) => (...args) => {
    check(method, args)
    if (method === 'createRadialGradient') return { addColorStop(...values) { check('addColorStop', values) } }
  } })
  return { context, calls }
}
function fixture({ reduced = false } = {}) {
  const stage = new Events(), document = new Events(), media = new Events()
  const frames = new Map(), observers = [], refs = [], watches = []
  const { context } = drawingContext()
  const surface = { dataset: {}, getContext: () => context, getBoundingClientRect: () => ({ left: 0, top: 0, width: 1440, height: 900 }) }
  document.hidden = false; document.documentElement = { classList: { contains: () => false } }; media.matches = reduced
  let mount, unmount, next = 0
  const ref = initial => {
    let value = initial
    const item = { get value() { return value }, set value(updated) { value = updated; for (const watch of watches) if (watch.sources.includes(item)) watch.fn() } }
    refs.push(item); return item
  }
  class Observer {
    constructor(callback) { this.callback = callback; this.disconnected = false; observers.push(this) }
    observe() {} disconnect() { this.disconnected = true }
  }
  const source = readFileSync(path.resolve(__dirname, '../app/components/AboutPointerBackground.vue'), 'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1]
  vm.runInNewContext(compile(source), {
    exports: {}, require: id => { assert.equal(id, '~/utils/aboutPointerBackground'); return painters },
    ref, computed: fn => ({ get value() { return fn() } }), watch: (sources, fn) => watches.push({ sources, fn }),
    onMounted: fn => { mount = fn }, onBeforeUnmount: fn => { unmount = fn },
    document, matchMedia: () => media, devicePixelRatio: 3,
    requestAnimationFrame: fn => { frames.set(++next, fn); return next }, cancelAnimationFrame: id => frames.delete(id),
    ResizeObserver: Observer, IntersectionObserver: Observer, MutationObserver: Observer,
  })
  refs[3].value = surface; refs[4].value = { parentElement: stage }; mount()
  return { surface, stage, document, media, frames, observers, refs, unmount,
    flush(now) { const pending = [...frames.values()]; frames.clear(); pending.forEach(fn => fn(now)) },
  }
}

test('reduced motion renders a still background, ignores pointer input, and toggles the loop with preference changes', () => {
  const f = fixture({ reduced: true })
  assert.equal(f.surface.dataset.backgroundState, 'reduced'); assert.equal(f.frames.size, 0)
  const before = f.surface.dataset.backgroundFrame
  f.stage.fire('pointermove', { clientX: 200, clientY: 200 })
  assert.equal(f.surface.dataset.backgroundFrame, before)
  assert.ok(f.surface.width * f.surface.height <= 1200000)
  f.media.matches = false; f.media.fire('change'); assert.equal(f.frames.size, 1)
  f.flush(100); f.flush(150)
  f.media.matches = true; f.media.fire('change')
  assert.equal(f.frames.size, 0); assert.equal(f.surface.dataset.backgroundState, 'reduced')
  assert.equal(f.surface.dataset.pointerStrength, '0.000')
  f.refs[0].value = 'mesh'; assert.equal(f.frames.size, 0)
  f.unmount()
})

test('pause freezes frame output and resume schedules exactly one frame', () => {
  const f = fixture()
  f.flush(100); f.flush(150)
  f.refs[1].value = false
  assert.equal(f.surface.dataset.backgroundState, 'paused'); assert.equal(f.frames.size, 0)
  const before = f.surface.dataset.backgroundFrame
  f.flush(5000); f.stage.fire('pointermove', { clientX: 900, clientY: 200 })
  assert.equal(f.surface.dataset.backgroundFrame, before)
  f.refs[1].value = true; assert.equal(f.frames.size, 1)
  f.unmount()
})

test('touch release lets pointer influence decay while mouse release preserves hover', () => {
  for (const pointerType of ['touch', 'mouse']) {
    const f = fixture()
    f.stage.fire('pointermove', { pointerType, clientX: 900, clientY: 200 })
    for (let now = 100; now <= 1100; now += 50) f.flush(now)
    const held = Number(f.surface.dataset.pointerStrength)
    assert.ok(held > .9, `${pointerType} responds before release`)
    f.stage.fire('pointerup', { pointerType })
    for (let now = 1150; now <= 2150; now += 50) f.flush(now)
    const released = Number(f.surface.dataset.pointerStrength)
    if (pointerType === 'touch') assert.ok(released < .01, `touch influence decays: ${released}`)
    else assert.ok(released > .99, `mouse hover persists: ${released}`)
    f.unmount()
  }
})

test('hidden and offscreen stop frames; returning visible respects pause state', () => {
  const f = fixture()
  f.document.hidden = true; f.document.fire('visibilitychange')
  assert.equal(f.surface.dataset.backgroundState, 'hidden'); assert.equal(f.frames.size, 0)
  f.document.hidden = false; f.document.fire('visibilitychange'); assert.equal(f.frames.size, 1)
  f.observers[1].callback([{ isIntersecting: false }])
  assert.equal(f.surface.dataset.backgroundState, 'offscreen'); assert.equal(f.frames.size, 0)
  f.refs[1].value = false
  f.observers[1].callback([{ isIntersecting: true }]); assert.equal(f.frames.size, 0)
  f.refs[1].value = true; assert.equal(f.frames.size, 1)
  f.unmount()
})

test('unmount cancels animation and releases all observers and DOM listeners', () => {
  const f = fixture()
  assert.equal(f.frames.size, 1); assert.equal(f.observers.length, 3)
  assert.ok(f.stage.count() > 0); f.unmount()
  assert.equal(f.frames.size, 0); assert.ok(f.observers.every(item => item.disconnected))
  for (const target of [f.stage, f.document, f.media]) assert.equal(target.count(), 0)
  f.refs[0].value = 'sand'; assert.equal(f.frames.size, 0)
})

test('all five painters produce finite coordinates with bounded work at desktop and phone sizes', () => {
  assert.equal(painters.aboutBackgrounds.length, 5)
  for (const [width, height] of [[1440, 900], [390, 844]]) {
    const paint = painters.createAboutBackgroundPainter()
    for (const { id } of painters.aboutBackgrounds) {
      for (let frame = 0; frame < 100; frame++) {
        const { context, calls } = drawingContext()
        paint(context, width, height, frame * .1, { x: frame % 2, y: (frame % 7) / 6, strength: 1 }, id, frame % 2 === 0)
        assert.ok(calls.length > 0 && calls.length < 10000, `${id} bounded drawing work: ${calls.length}`)
      }
    }
  }
})
