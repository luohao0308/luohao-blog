const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')

function fixture(navigationResult) {
  const hooks = new Map(), timers = [], errors = [], scrolls = [], focus = [], calls = []
  class Element { closest(selector) { return selector === 'a' ? { getAttribute: () => '/posts/example' } : null } }
  const app = { hook(name, callback) {
    const entries = hooks.get(name) || new Set(); entries.add(callback); hooks.set(name, entries)
    return () => entries.delete(callback)
  } }
  const exports = {}
  vm.runInNewContext(ts.transpileModule(readFileSync(path.resolve(__dirname, '../app/composables/useArticleExpansion.ts'), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText, {
    exports, Element,
    require: () => ({ isArticleExpansionClick: () => true, expandArticleFrom: async (_card, update) => { calls.push('expand'); await update(); calls.push('expanded') } }),
    useNuxtApp: () => app,
    navigateTo: async to => { calls.push(to); return navigationResult },
    nextTick: async () => { calls.push('tick') },
    window: { getSelection: () => null, scrollTo: options => scrolls.push(options) },
    document: { querySelector: selector => { assert.equal(selector, '[data-article-detail] h1'); return { focus: options => focus.push(options) } } },
    showError: error => errors.push(error),
    setTimeout: callback => { timers.push(callback); return timers.length }, clearTimeout: () => {},
  })
  const event = { target: new Element(), currentTarget: {}, preventDefault() { calls.push('prevented') } }
  const operation = exports.useArticleExpansion()(event, '/posts/example')
  let settled = false
  operation.then(() => { settled = true })
  return { hooks, timers, errors, scrolls, focus, calls, operation,
    get settled() { return settled },
    fire(name, value) { for (const callback of [...(hooks.get(name) || [])]) callback(value) },
    count() { return [...hooks.values()].reduce((count, entries) => count + entries.size, 0) },
  }
}
const drain = () => new Promise(resolve => setImmediate(resolve))

test('article expansion waits for real page completion without a timeout-based success path', async () => {
  const f = fixture()
  await drain()
  assert.equal(f.settled, false); assert.equal(f.count(), 2)
  // Running every scheduled timer must never substitute for Nuxt page:finish.
  f.timers.forEach(callback => callback())
  await drain()
  assert.equal(f.settled, false); assert.equal(f.scrolls.length, 0); assert.equal(f.focus.length, 0)
  assert.equal(f.timers.length, 0)
  f.fire('page:finish'); await f.operation
  assert.equal(f.settled, true); assert.equal(f.count(), 0)
  assert.equal(f.errors.length, 0); assert.equal(f.scrolls.length, 1); assert.equal(f.focus.length, 1)
  assert.deepEqual(f.calls, ['prevented', 'expand', '/posts/example', 'tick', 'expanded'])
})

test('app:error rejects the page wait and removes both lifecycle hooks', async () => {
  const f = fixture(), failure = new Error('page render failed')
  await drain()
  assert.equal(f.count(), 2)
  f.fire('app:error', failure); await f.operation
  assert.equal(f.count(), 0); assert.deepEqual(f.errors, [failure])
  assert.equal(f.scrolls.length, 0); assert.equal(f.focus.length, 0)
  assert.equal(f.calls.includes('expanded'), false)
  f.fire('page:finish'); assert.equal(f.count(), 0)
})

test('aborted navigation removes lifecycle hooks without waiting for page:finish', async () => {
  const f = fixture(false)
  await drain()
  assert.equal(f.settled, true); await f.operation
  assert.equal(f.count(), 0); assert.equal(f.errors.length, 0)
  assert.equal(f.scrolls.length, 0); assert.equal(f.calls.includes('tick'), false)
})
