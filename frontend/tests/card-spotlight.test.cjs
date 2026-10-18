const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')

function load() {
  const exports = {}
  vm.runInNewContext(ts.transpileModule(readFileSync(path.resolve(__dirname, '../app/utils/cardSpotlight.ts'), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText, { exports })
  return exports.mountCardSpotlight
}
class Events {
  listeners = new Map()
  addEventListener(type, fn, options) {
    const capture = options === true || !!options?.capture
    const entries = this.listeners.get(type) || []
    entries.push({ fn, capture }); this.listeners.set(type, entries)
  }
  removeEventListener(type, fn, options) {
    const capture = options === true || !!options?.capture
    this.listeners.set(type, (this.listeners.get(type) || []).filter(entry => entry.fn !== fn || entry.capture !== capture))
  }
  fire(type, event = {}) { for (const entry of [...(this.listeners.get(type) || [])]) entry.fn(event) }
  count() { return [...this.listeners.values()].reduce((sum, entries) => sum + entries.length, 0) }
}
function fixture() {
  const root = new Events(), doc = new Events(), view = new Events(), fine = new Events(), reduce = new Events()
  const frames = new Map(), log = []
  let nextFrame = 0
  fine.matches = true; reduce.matches = false; doc.hidden = false
  view.matchMedia = query => query.includes('reduced-motion') ? reduce : fine
  view.requestAnimationFrame = fn => { frames.set(++nextFrame, fn); return nextFrame }
  view.cancelAnimationFrame = id => frames.delete(id)
  doc.defaultView = view; root.ownerDocument = doc; root.dataset = {}
  root.removeAttribute = name => { assert.equal(name, 'data-card-spotlight'); delete root.dataset.cardSpotlight }
  function card(name, left, top) {
    const values = new Map()
    return { name, values, isConnected: true,
      getBoundingClientRect() { log.push(`read:${name}`); return { left, top } },
      style: {
        setProperty(key, value) { log.push(`write:${name}`); values.set(key, value) },
        removeProperty(key) { values.delete(key) },
      },
    }
  }
  let cards = [card('a', 10, 20), card('b', 100, 200)]
  root.querySelectorAll = selector => { assert.equal(selector, '.bento-card, .ui-panel'); return cards }
  const controller = load()(root)
  return { root, doc, view, fine, reduce, frames, log, controller, card,
    get cards() { return cards }, set cards(value) { cards = value },
    move(x = 150, y = 250, pointerType = 'mouse') { root.fire('pointermove', { clientX: x, clientY: y, pointerType }) },
    flush() { const pending = [...frames.values()]; frames.clear(); pending.forEach(fn => fn()) },
  }
}
function assertReset(f) {
  assert.equal(f.frames.size, 0)
  assert.equal(f.root.dataset.cardSpotlight, undefined)
  f.cards.forEach(card => assert.equal(card.values.size, 0))
}

test('pointer updates coalesce to the latest position and read all geometry before writing', () => {
  const f = fixture()
  f.move(30, 50); f.move(170, 280)
  assert.equal(f.frames.size, 1); assert.equal(f.log.length, 0)
  f.flush()
  assert.equal(f.root.dataset.cardSpotlight, 'true')
  assert.equal(f.cards[0].values.get('--card-x'), '160px')
  assert.equal(f.cards[0].values.get('--card-y'), '260px')
  assert.equal(f.cards[1].values.get('--card-x'), '70px')
  assert.equal(f.cards[1].values.get('--card-y'), '80px')
  assert.deepEqual(f.log, ['read:a', 'read:b', 'write:a', 'write:a', 'write:b', 'write:b'])
  f.controller.destroy()
})

test('leave, cancel, scrolling, resizing, media, visibility and explicit reset clear pending and painted effects', () => {
  for (const [target, signal] of [['root', 'pointerleave'], ['root', 'pointercancel'], ['view', 'scroll'], ['view', 'resize'], ['fine', 'change'], ['reduce', 'change'], ['doc', 'visibilitychange'], ['controller', 'reset']]) {
    const f = fixture()
    f.move(); f.flush(); f.move(300, 400)
    if (signal === 'reset') f.controller.reset()
    else f[target].fire(signal)
    assertReset(f)
    f.flush(); assertReset(f)
    f.controller.destroy()
  }
})

test('touch, coarse pointers, reduced motion and hidden documents never schedule pointer lighting', () => {
  for (const state of ['touch', 'pen', 'coarse', 'reduce', 'hidden']) {
    const f = fixture()
    f.move(); f.flush()
    if (state === 'coarse') f.fine.matches = false
    if (state === 'reduce') f.reduce.matches = true
    if (state === 'hidden') f.doc.hidden = true
    f.move(200, 300, state === 'touch' || state === 'pen' ? state : 'mouse')
    assertReset(f)
    f.controller.destroy()
  }
  const f = fixture()
  f.move(); f.reduce.matches = true; f.flush()
  assertReset(f); assert.equal(f.log.length, 0)
  f.controller.destroy()
})

test('new cards receive lighting and removed cards lose stored coordinates', () => {
  const f = fixture()
  f.move(); f.flush()
  const removed = f.cards[0], added = f.card('new', 120, 210)
  removed.isConnected = false
  f.cards = [f.cards[1], added]
  f.move(200, 300); f.flush()
  assert.equal(removed.values.size, 0)
  assert.equal(added.values.get('--card-x'), '80px')
  assert.equal(added.values.get('--card-y'), '90px')
  f.controller.reset(); assertReset(f)
  f.cards = []; f.move(); f.flush()
  assert.equal(f.root.dataset.cardSpotlight, undefined)
  f.controller.destroy()
})

test('destroy cancels work and removes all listeners including capturing scroll', () => {
  const f = fixture()
  f.move(); f.flush(); f.move()
  f.controller.destroy(); assertReset(f)
  assert.equal([f.root, f.doc, f.view, f.fine, f.reduce].reduce((sum, target) => sum + target.count(), 0), 0)
  const logLength = f.log.length
  f.move(); f.view.fire('scroll'); f.flush()
  assertReset(f); assert.equal(f.log.length, logLength)
  f.controller.destroy()
})
