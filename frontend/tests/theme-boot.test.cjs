const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const vm = require('node:vm')
const { test } = require('node:test')
const ts = require('typescript')
const read = file => readFileSync(path.resolve(__dirname, '../app', file), 'utf8')
const compile = source => ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText
function theme(stored = null, systemDark = true, denied = false) {
  const classes = new Set()
  const root = { classList: { toggle: (name, enabled) => enabled ? classes.add(name) : classes.delete(name), contains: name => classes.has(name) }, style: {} }
  const context = { exports: {}, document: { documentElement: root }, window: { matchMedia: () => ({ matches: systemDark }) }, localStorage: { getItem() { if (denied) throw Error('denied'); return stored }, setItem() { if (denied) throw Error('denied') } }, useState: (_, init) => ({ value: init() }) }
  vm.createContext(context)
  vm.runInContext(read('app.vue').match(/const themeScript = `([^`]+)`/)[1], context)
  const headDark = classes.has('dark')
  vm.runInContext(compile(read('composables/useTheme.ts').replaceAll('import.meta.client', 'true')), context)
  const controller = context.exports.useTheme(); controller.init()
  return { controller, classes, root, headDark }
}
test('head and hydration use light without an explicit valid dark/system preference', () => {
  for (const stored of [null, '', 'invalid', 'light']) {
    const run = theme(stored)
    assert.equal(run.headDark, false)
    assert.equal(run.classes.has('dark'), false)
    assert.equal(run.controller.preference.value, 'light')
    assert.equal(run.root.style.colorScheme, 'light')
  }
})
test('head and hydration preserve explicit dark and system choices', () => {
  for (const [stored, darkSystem, expected] of [['dark', false, true], ['system', true, true], ['system', false, false]]) {
    const run = theme(stored, darkSystem)
    assert.equal(run.headDark, expected)
    assert.equal(run.classes.has('dark'), expected)
    assert.equal(run.controller.preference.value, stored)
  }
})
test('storage failures do not interrupt initialization or applying a user choice', () => {
  const run = theme(null, true, true)
  assert.equal(run.headDark, false)
  assert.equal(run.controller.preference.value, 'light')
  assert.doesNotThrow(() => run.controller.set('dark'))
  assert.equal(run.classes.has('dark'), true)
  assert.equal(run.controller.preference.value, 'dark')
  assert.doesNotThrow(() => run.controller.toggle())
  assert.equal(run.classes.has('dark'), false)
})
function boot(options = {}) {
  const root = { dataset: {}, removeAttribute: () => { delete root.dataset.homeEntering } }
  let timer, pageshow
  const context = { exports: {}, location: { pathname: options.path ?? '/' }, document: { documentElement: root }, window: { scrollY: options.scroll ?? 0, matchMedia: () => ({ matches: !!options.reduced }), addEventListener: (_, callback) => { pageshow = callback } }, HTMLElement: { prototype: { animate: options.unsupported ? undefined : () => {} } }, sessionStorage: { getItem() { if (options.denied) throw Error('denied'); return options.seen ? '1' : null } }, setTimeout: (callback, ms) => { assert.equal(ms, 1500); timer = callback } }
  vm.createContext(context)
  vm.runInContext(compile(read('utils/homeEntranceBoot.ts')), context)
  vm.runInContext(context.exports.HOME_ENTRANCE_BOOT_SCRIPT, context)
  return { root, timeout: () => timer(), pageshow: persisted => pageshow({ persisted }) }
}
test('boot gate is homepage-only and fails open on timeout/bfcache; static fallbacks stay visible', () => {
  const run = boot()
  assert.equal(run.root.dataset.homeEntering, 'true')
  run.pageshow(false)
  assert.equal(run.root.dataset.homeEntering, 'true')
  run.pageshow(true)
  assert.equal(run.root.dataset.homeEntering, undefined)
  assert.equal(run.root.dataset.homeEntranceAborted, 'true')
  const stalled = boot(); stalled.timeout()
  assert.equal(stalled.root.dataset.homeEntering, undefined)
  assert.equal(stalled.root.dataset.homeEntranceAborted, 'true')
  for (const options of [{ path: '/posts' }, { scroll: 50 }, { reduced: true }, { unsupported: true }, { seen: true }, { denied: true }]) assert.equal(boot(options).root.dataset.homeEntering, undefined)
})

// Mounted animation removes the gate itself before either delayed browser event.
test('a released boot gate never aborts an animation that already started', () => {
  const run = boot()
  run.root.removeAttribute('data-home-entering')
  run.timeout()
  run.pageshow(true)
  assert.equal(run.root.dataset.homeEntranceAborted, undefined)
})

test('restored scroll or unavailable storage prevents a late homepage entrance', () => {
  for (const options of [{ scroll: 50 }, { denied: true }]) {
    const run = boot(options)
    assert.equal(run.root.dataset.homeEntering, undefined)
    assert.equal(run.root.dataset.homeEntranceAborted, 'true')
  }
  for (const options of [{ path: '/posts', scroll: 50 }, { path: '/posts', denied: true }, { seen: true }, { reduced: true }, { unsupported: true }]) {
    assert.equal(boot(options).root.dataset.homeEntranceAborted, undefined)
  }
})
