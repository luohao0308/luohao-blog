const assert = require('node:assert/strict')
const { readFileSync } = require('node:fs')
const path = require('node:path')
const { test } = require('node:test')
const vm = require('node:vm')
const ts = require('typescript')

const app = path.resolve(__dirname, '../app')

// Exercise the actual middleware and setup logic without a live backend or
// credentials. Browser coverage separately checks the resulting SSR/DOM.
function setup(file, { role = null, server = false, hydrating = false, serverRendered = false, loginRole = role } = {}) {
  const source = readFileSync(path.join(app, file), 'utf8')
  const script = file.endsWith('.vue') ? source.match(/<script setup lang="ts">([\s\S]*?)<\/script>/)[1] : source
  const user = { value: role === null ? null : { role } }
  const navigations = []
  let mounted
  let restores = 0
  const context = {
    exports: {},
    defineNuxtRouteMiddleware: handler => handler,
    definePageMeta: () => {},
    useHead: () => {},
    useNuxtApp: () => ({ isHydrating: hydrating, payload: { serverRendered } }),
    useRoute: () => ({ query: { redirect: '/admin/projects' } }),
    useAuth: () => ({
      user,
      ensureSession: async () => { restores++ },
      errReason: () => '',
      login: async () => { user.value = { role: loginRole } },
    }),
    navigateTo: (target, options) => { navigations.push({ target, options }); return 'redirect' },
    ref: value => ({ value }),
    computed: getter => ({ get value() { return getter() } }),
    onMounted: handler => { mounted = handler },
  }
  const code = script.replace(/^import .*$/gm, '').replaceAll('import.meta.server', String(server))
  vm.createContext(context)
  vm.runInContext(ts.transpileModule(code, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText, context)
  return { context, navigations, mount: () => mounted(), restores: () => restores, user }
}

test('only ADMIN may enter management routes', async () => {
  for (const role of [null, 0, 2, 1]) {
    const run = setup('middleware/admin-auth.ts', { role })
    await run.context.exports.default({ fullPath: '/admin/posts?page=2' })
    assert.equal(run.restores(), 1)
    assert.equal(run.navigations.length, role === 1 ? 0 : 1, `role ${role}`)
    if (role !== 1) {
      assert.equal(run.navigations[0].target.path, '/admin/login')
      assert.equal(run.navigations[0].target.query.redirect, '/admin/posts?page=2')
    }
  }
})

test('initial SSR redirect loads matching HTML; normal SPA redirects remain client side', async () => {
  for (const [hydrating, serverRendered, external] of [[true, true, true], [true, false, false], [false, true, false]]) {
    const run = setup('middleware/admin-auth.ts', { hydrating, serverRendered })
    await run.context.exports.default({ fullPath: '/admin/projects' })
    assert.equal(run.navigations[0].options?.external, external)
    assert.equal(run.navigations[0].options?.replace, true)
  }
  const server = setup('middleware/admin-auth.ts', { server: true })
  await server.context.exports.default({ fullPath: '/admin/posts' })
  assert.equal(server.restores(), 0)
  assert.equal(server.navigations.length, 0)
})

test('login keeps readers on the form with a permission notice instead of redirecting in a loop', async () => {
  const run = setup('pages/admin/login.vue', { role: 2 })
  await run.mount()
  assert.equal(run.navigations.length, 0)
  assert.equal(vm.runInContext('isNonAdmin.value', run.context), true)
  vm.runInContext("email.value = 'reader@example.invalid'; password.value = 'fixture-only'", run.context)
  await vm.runInContext('submit()', run.context)
  assert.equal(run.navigations.length, 0)
  assert.equal(vm.runInContext('isNonAdmin.value', run.context), true)
  const template = readFileSync(path.join(app, 'pages/admin/login.vue'), 'utf8')
  assert.match(template, /v-if="isNonAdmin"[^>]*role="alert"/)
  assert.match(template, /请使用站长账号登录/)
})

test('existing admins and switching from reader to admin follow the return path', async () => {
  const existing = setup('pages/admin/login.vue', { role: 1 })
  await existing.mount()
  assert.equal(existing.navigations[0].target, '/admin/projects')
  const switched = setup('pages/admin/login.vue', { role: 2, loginRole: 1 })
  await switched.mount()
  vm.runInContext("email.value = 'admin@example.invalid'; password.value = 'fixture-only'", switched.context)
  await vm.runInContext('submit()', switched.context)
  assert.equal(switched.navigations.length, 1)
  assert.equal(switched.navigations[0].target, '/admin/projects')
})
