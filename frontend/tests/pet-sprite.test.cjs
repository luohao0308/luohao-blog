const assert = require('node:assert/strict')
const { test } = require('node:test')
const { readFileSync } = require('node:fs')
const vm = require('node:vm')
const ts = require('typescript')
const { ref, reactive, watch, effectScope, nextTick } = require('vue')
const drain = () => new Promise(resolve => setImmediate(resolve))

function fixture(loadSource = async () => ({}), reduced = false) {
  const props = reactive({ src: '/local/usagi.webp', state: 'idle', size: 88 })
  const events = [], instances = [], mounts = [], cleanups = [], warnings = [], listeners = new Map(), effects = effectScope()
  const preference = { matches: reduced, addEventListener: (name, fn) => listeners.set('media:' + name, fn), removeEventListener: name => listeners.delete('media:' + name) }
  const document = { hidden: false, addEventListener: (name, fn) => listeners.set(name, fn), removeEventListener: name => listeners.delete(name) }
  let imports = 0
  class Renderer {
    constructor(options) { this.options = options; this.calls = []; instances.push(this) }
    setState(value) { this.calls.push(['state', value]) }
    render() { this.calls.push(['render']) }
    play() { this.calls.push(['play']) }
    pause() { this.calls.push(['pause']) }
    destroy() { this.calls.push(['destroy']) }
    resize(...args) { this.calls.push(['resize', ...args]) }
  }
  const source = readFileSync('app/components/PetSprite.vue', 'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1].replace(/import type[^\n]*\n/, '').replace("import('sprite-pet')", '__loadModule()')
  const context = { ref, watch, Number, Math, document, window: { devicePixelRatio: 3, matchMedia: () => preference },
    defineProps: () => props, withDefaults: value => value, defineEmits: () => (...args) => events.push(args),
    onMounted: fn => mounts.push(fn), onBeforeUnmount: fn => cleanups.push(fn),
    console: { warn: (...args) => warnings.push(args) },
    __loadModule: async () => { imports++; return { SpritePetRenderer: Renderer, loadSpritePetSource: loadSource, getApproximateAnimationDefinition: name => ({ source: name, frameCount: name === 'idle' ? 6 : name === 'waving' ? 4 : 8 }) } },
  }
  effects.run(() => vm.runInNewContext(ts.transpileModule(source + '\nglobalThis.api = { canvas };', { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context))
  context.api.canvas.value = {}
  return { props, instances, events, warnings, preference, document, listeners, get imports() { return imports },
    mount() { mounts.forEach(fn => fn()) }, close() { cleanups.forEach(fn => fn()); effects.stop() } }
}

test('pet loads only on mount, uses low-level renderer and bounded geometry, then cleans up', async () => {
  const f = fixture()
  assert.equal(f.imports, 0)
  f.mount(); await drain()
  const r = f.instances[0]
  assert.equal(r.options.autoplay, false)
  assert.equal(r.options.initialState, 'idle')
  assert.equal(r.options.fit, 'contain')
  assert.equal(r.options.pixelRatio, 2)
  assert.equal(r.options.animations.idle.frameCount, 6)
  assert.equal(r.options.animations.wave.frameCount, 4)
  assert.equal(r.options.animations.working.source, 'running')
  assert.equal(r.options.animations.failure.source, 'failed')
  assert.equal(r.options.width, 88)
  assert.equal(r.options.height, 88 * 208 / 192)
  assert.deepEqual(f.events.at(-1), ['ready', true])
  f.props.state = 'working'; await nextTick()
  assert(r.calls.some(c => c[0] === 'state' && c[1] === 'working'))
  f.props.size = 64; await nextTick()
  assert(r.calls.some(c => c[0] === 'resize' && c[1] === 64))
  f.close()
  assert.equal(r.calls.at(-1)[0], 'destroy')
  assert.equal(f.listeners.size, 0)
})

test('reduced motion renders idle without playing, visibility pauses and resume respects preference', async () => {
  const f = fixture(undefined, true)
  f.mount(); await drain()
  const r = f.instances[0]
  f.props.state = 'jump'; await nextTick()
  assert.equal(r.calls.some(c => c[0] === 'play'), false)
  assert(r.calls.filter(c => c[0] === 'state').every(c => c[1] === 'idle'))
  f.preference.matches = false; f.listeners.get('media:change')()
  assert.equal(r.calls.at(-1)[0], 'play')
  f.document.hidden = true; f.listeners.get('visibilitychange')()
  assert.deepEqual(r.calls.slice(-2), [['pause'], ['render']])
  f.document.hidden = false; f.listeners.get('visibilitychange')()
  assert.equal(r.calls.at(-1)[0], 'play')
  f.close()
})

test('late source loads cannot construct after a replacement or unmount', async () => {
  const pending = []
  const f = fixture((_manifest, src) => new Promise(resolve => pending.push({ src, resolve })))
  f.mount(); await drain()
  f.props.src = '/local/replacement.webp'; await nextTick(); await drain()
  pending[0].resolve({}); await drain()
  assert.equal(f.instances.length, 0)
  pending[1].resolve({}); await drain()
  assert.equal(f.instances.length, 1)
  f.props.src = '/local/late.webp'; await nextTick(); await drain()
  assert.equal(f.instances[0].calls.at(-1)[0], 'destroy')
  f.close(); pending[2].resolve({}); await drain()
  assert.equal(f.instances.length, 1)
})

test('load failure reports readiness false and leaves the parent fallback available', async () => {
  const f = fixture(async () => { throw new Error('cannot decode') })
  f.mount(); await drain()
  assert.equal(f.instances.length, 0)
  assert.equal(f.warnings.length, 1)
  assert.deepEqual(f.events.slice(-2), [['ready', false], ['failure']])
  f.close()
})
