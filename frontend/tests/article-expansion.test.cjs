const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')

function load() {
  const exports = {}
  vm.runInNewContext(ts.transpileModule(readFileSync(path.resolve(__dirname, '../app/utils/articleExpansion.ts'), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 },
  }).outputText, { exports })
  return exports
}
function deferred() {
  let resolve, reject
  const promise = new Promise((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}
function fixture() {
  const attributes = new Map(), values = new Map()
  const html = {
    hasAttribute: key => attributes.has(key),
    setAttribute: (key, value) => attributes.set(key, value),
    removeAttribute: key => attributes.delete(key),
    style: { setProperty: (key, value) => values.set(key, value), removeProperty: key => values.delete(key) },
  }
  const view = { innerWidth: 1000, innerHeight: 800, matchMedia: () => ({ matches: false }) }
  const detail = { style: { viewTransitionName: 'detail-original' } }
  const doc = { documentElement: html, defaultView: view, querySelector: selector => { assert.equal(selector, '[data-article-detail]'); return detail } }
  const card = { style: { viewTransitionName: 'card-original' }, ownerDocument: doc, getBoundingClientRect: () => ({ top: 50, right: 700, bottom: 350, left: 200 }) }
  return { doc, view, card, detail, attributes, values }
}
function assertClean(f) {
  assert.equal(f.card.style.viewTransitionName, 'card-original')
  assert.equal(f.detail.style.viewTransitionName, 'detail-original')
  assert.equal(f.attributes.has('data-article-expanding'), false)
  assert.equal(f.values.has('--article-expand-from'), false)
}

test('only an unhandled primary click without modifiers expands an article', () => {
  const { isArticleExpansionClick } = load()
  const click = { defaultPrevented: false, button: 0, metaKey: false, ctrlKey: false, shiftKey: false, altKey: false }
  assert.equal(isArticleExpansionClick(click), true)
  for (const modification of [{ defaultPrevented: true }, { button: 1 }, { button: 2 }, { metaKey: true }, { ctrlKey: true }, { shiftKey: true }, { altKey: true }]) {
    assert.equal(isArticleExpansionClick({ ...click, ...modification }), false)
  }
})

test('unsupported documents and reduced motion navigate once without staging visual state', async () => {
  const { expandArticleFrom } = load()
  for (const reason of ['no-view', 'no-api', 'reduced-motion']) {
    const f = fixture(); let calls = 0
    if (reason === 'no-view') f.doc.defaultView = null
    if (reason === 'reduced-motion') {
      f.view.matchMedia = () => ({ matches: true })
      f.doc.startViewTransition = () => assert.fail('reduced motion must bypass transitions')
    }
    f.card.getBoundingClientRect = () => assert.fail('fallback must not read geometry')
    await expandArticleFrom(f.card, async () => { calls++; assertClean(f) })
    assert.equal(calls, 1); assertClean(f)
  }
})

test('native transition captures the source rectangle and keeps state until the animation finishes', async () => {
  const { expandArticleFrom } = load(), f = fixture(), finished = deferred(), navigated = deferred()
  let calls = 0, callback, updated
  f.doc.startViewTransition = update => {
    callback = update
    updated = Promise.resolve().then(update)
    return { ready: Promise.resolve(), updateCallbackDone: updated, finished: finished.promise }
  }
  const operation = expandArticleFrom(f.card, async () => { calls++; navigated.resolve() })
  assert.equal(typeof callback, 'function')
  assert.equal(f.card.style.viewTransitionName, 'article-content')
  assert.equal(f.detail.style.viewTransitionName, 'detail-original')
  assert.equal(f.attributes.has('data-article-expanding'), true)
  assert.equal(f.values.get('--article-expand-from'), 'inset(50px 300px 450px 200px round 24px)')
  await navigated.promise
  // Wait for the native update callback, while the animation remains unfinished.
  await updated
  assert.equal(f.detail.style.viewTransitionName, 'article-content')
  assert.equal(calls, 1)
  assert.equal(f.attributes.has('data-article-expanding'), true)
  // A second click during the same expansion must not start another navigation.
  await expandArticleFrom(f.card, async () => { calls++ })
  assert.equal(calls, 1)
  finished.resolve(); await operation; assertClean(f)
})

test('skipped ready and finished promises do not suppress or repeat navigation', async () => {
  const { expandArticleFrom } = load(), f = fixture()
  let calls = 0
  f.doc.startViewTransition = update => ({
    ready: Promise.reject(new Error('ready skipped')),
    updateCallbackDone: Promise.resolve().then(update),
    finished: Promise.reject(new Error('animation skipped')),
  })
  await expandArticleFrom(f.card, async () => { calls++ })
  assert.equal(calls, 1); assertClean(f)
})

test('failed navigation propagates once and removes transition state', async () => {
  const { expandArticleFrom } = load(), f = fixture(), failure = new Error('navigation failed')
  let calls = 0
  f.doc.startViewTransition = update => ({
    ready: Promise.resolve(), updateCallbackDone: Promise.resolve().then(update), finished: Promise.reject(failure),
  })
  await assert.rejects(expandArticleFrom(f.card, async () => { calls++; throw failure }), error => error === failure)
  assert.equal(calls, 1); assertClean(f)
})

test('a synchronous native API failure falls back once and cleans even if navigation rejects', async () => {
  const { expandArticleFrom } = load()
  for (const rejectNavigation of [false, true]) {
    const f = fixture(), failure = new Error('fallback navigation failed'); let calls = 0
    f.doc.startViewTransition = () => { throw new Error('native API failed') }
    const operation = expandArticleFrom(f.card, async () => { calls++; if (rejectNavigation) throw failure })
    if (rejectNavigation) await assert.rejects(operation, error => error === failure)
    else await operation
    assert.equal(calls, 1); assertClean(f)
  }
})
