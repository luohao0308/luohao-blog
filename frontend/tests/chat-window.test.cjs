const assert = require('node:assert/strict')
const { test } = require('node:test')
const { readFileSync } = require('node:fs')
const vm = require('node:vm')
const ts = require('typescript')
const { computed, ref, reactive, watch, nextTick, effectScope } = require('vue')

function fixture(articleSlug, fetchReply) {
  const props = reactive({ articleSlug, embedded: true }), requests = [], statuses = [], cleanups = [], effects = effectScope()
  const source = readFileSync('app/components/ChatWindow.vue', 'utf8').match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1].replaceAll('import.meta.client', 'false')
  const context = { computed, ref, reactive, watch, nextTick, AbortController, setTimeout, clearTimeout, exports: {},
    defineProps: () => props, onBeforeUnmount: fn => cleanups.push(fn),
    defineEmits: () => (event, value) => { assert.equal(event, 'status'); statuses.push(value) },
    $fetch: async (url, options) => { requests.push({ url, ...options }); return fetchReply(options) },
  }
  effects.run(() => vm.runInNewContext(ts.transpileModule(source + '\nglobalThis.api = { input, questions, busy, ask, send };', { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.CommonJS } }).outputText, context))
  return { ...context.api, props, requests, statuses, close() { cleanups.forEach(fn => fn()); effects.stop() } }
}
const body = request => JSON.parse(JSON.stringify(request.body))

test('site-wide questions omit article scope and never auto-request or auto-retry', async () => {
  const f = fixture(undefined, async () => ({ answer: '全站回答', outcome: 1 }))
  try {
    assert.equal(f.requests.length, 0)
    f.input.value = '博客讲了哪些内容？'; await f.ask()
    assert.deepEqual(body(f.requests[0]), { query: '博客讲了哪些内容？' })
    assert.equal(f.requests[0].retry, 0)
    assert.equal(f.questions.value[0].reply.answer, '全站回答')
    assert.equal(f.requests.length, 1)
    assert.deepEqual(f.statuses, ['working', 'done'])
  } finally { f.close() }
})

test('article questions include the exact scope and reject missing or mismatched echo', async () => {
  for (const echo of [undefined, 'other']) {
    const f = fixture('reading', async () => ({ answer: '不应显示', outcome: 1, article_slug: echo }))
    try {
      f.input.value = '本文讲什么？'; await f.ask()
      assert.deepEqual(body(f.requests[0]), { query: '本文讲什么？', article_slug: 'reading' })
      assert.equal(f.questions.value[0].state, 'failed')
      assert.equal(f.questions.value[0].reply, undefined)
    } finally { f.close() }
  }
})

test('site-wide mode rejects an accidentally article-scoped answer', async () => {
  const f = fixture(undefined, async () => ({ answer: '不应显示的本文回答', outcome: 1, article_slug: 'reading' }))
  try {
    f.input.value = '全站问题'; await f.ask()
    assert.equal(f.questions.value[0].state, 'failed')
    assert.equal(f.questions.value[0].reply, undefined)
  } finally { f.close() }
})

test('switching scope keeps the transcript, draft, and in-flight reply', async () => {
  let resolve
  const f = fixture('reading', options => options.body.article_slug
    ? new Promise(r => { resolve = r })
    : Promise.resolve({ answer: '全站回答', outcome: 1 }))
  try {
    f.input.value = '本文问题'
    const pending = f.ask()
    f.input.value = '准备中的下一条'
    f.props.articleSlug = undefined
    await nextTick()
    assert.equal(f.requests[0].signal.aborted, false)
    assert.equal(f.questions.value.length, 1)
    assert.equal(f.input.value, '准备中的下一条')
    resolve({ answer: '本文回答', outcome: 1, article_slug: 'reading' })
    await pending
    assert.equal(f.questions.value[0].reply.answer, '本文回答')
    await f.ask()
    assert.deepEqual(body(f.requests[1]), { query: '准备中的下一条' })
    assert.equal(f.questions.value.length, 2)
    assert.equal(f.questions.value[1].reply.answer, '全站回答')
  } finally { f.close() }
})

test('retry uses the failed question’s original scope after a switch', async () => {
  let failed = true
  const f = fixture('reading', async () => {
    if (failed) throw { response: { status: 429 } }
    return { answer: '回答', outcome: 1, article_slug: 'reading' }
  })
  try {
    f.input.value = '保留的问题'
    await f.ask()
    assert.equal(f.questions.value[0].state, 'failed')
    assert.equal(f.questions.value[0].query, '保留的问题')
    f.props.articleSlug = undefined
    await nextTick()
    assert.equal(f.requests.length, 1)
    failed = false
    await f.send(f.questions.value[0])
    assert.deepEqual(body(f.requests[1]), { query: '保留的问题', article_slug: 'reading' })
    assert.equal(f.questions.value[0].state, 'returned')
  } finally { f.close() }
})

test('unmount aborts work and never applies the late response', async () => {
  let resolve
  const f = fixture('reading', () => new Promise(r => { resolve = r }))
  f.input.value = '离开页面'; const pending = f.ask(); f.close()
  assert.equal(f.requests[0].signal.aborted, true)
  resolve({ answer: '迟到', article_slug: 'reading', outcome: 1 }); await pending
  assert.equal(f.questions.value[0].reply, undefined)
})
