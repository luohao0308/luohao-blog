const { test } = require('node:test'), assert = require('node:assert/strict'), fs = require('node:fs'), vm = require('node:vm'), ts = require('typescript'), vue = require('vue')
function fixture(fetcher) {
  const props = vue.reactive({ root: { id: 'root-1' } })
  const user = vue.ref({ id: 'admin-a' }), drafts = vue.ref({}), hooks = [], emitted = []
  let source = fs.readFileSync(require('node:path').join(__dirname, '../app/components/admin/AuthorReplyEditor.vue'), 'utf8').split('<script setup lang="ts">')[1].split('</script>')[0].replace(/^import .*$/gm, '')
  source += '\nglobalThis.editor = { text, current, ready, busy, notice, load, save, remove };'
  const context = { ...vue, onMounted: fn => hooks.push(fn), onBeforeUnmount: () => {}, defineProps: () => props, defineEmits: () => (...args) => emitted.push(args), useAuth: () => ({ user, authFetch: fetcher }), useState: () => drafts }
  vm.runInNewContext(ts.transpileModule(source, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, context)
  return { ...context.editor, user, drafts, emitted, props }
}
test('reply conflict retains draft and disables save until explicit latest version load', async () => {
  let mode = 'load', calls = 0
  const f = fixture(async (_url, options) => { calls++; if (options.method === 'PUT') throw { response: { status: 409 } }; return { id: 'reply', user_id: 'admin-a', reply_version: mode === 'load' ? '2' : '3', content: 'server' } })
  await f.load(); f.text.value = 'my draft'; await f.save()
  assert.equal(f.text.value, 'my draft'); assert.equal(f.ready.value, false); assert.match(f.notice.value, /草稿已保留/)
  const before = calls; await f.save(); assert.equal(calls, before)
  mode = 'new'; await f.load(); assert.equal(f.current.value.reply_version, '3'); assert.equal(f.text.value, 'my draft'); assert.equal(f.ready.value, true)
})
test('account change blocks delayed successful save from changing reply or success state', async () => {
  let resolve
  const f = fixture(async (_url, options) => options.method === 'PUT' ? new Promise(r => { resolve = r }) : { id: 'reply', user_id: 'admin-a', reply_version: '1', content: 'old' })
  await f.load(); f.text.value = 'private draft'; const pending = f.save(); f.user.value = { id: 'admin-b' }; await vue.nextTick(); resolve({ id: 'reply', content: 'private draft', status: 2 }); await pending
  assert.equal(f.text.value, ''); assert.equal(f.current.value, null); assert.equal(f.emitted.some(e => e[0] === 'saved'), false); assert.equal(f.drafts.value['admin-a:root-1'], 'private draft')
})
test('failed automatic detection preserves old reply and submitted draft', async () => {
  const f = fixture(async (_url, options) => { if (options.method === 'PUT') throw { response: { status: 400 } }; return { id: 'reply', user_id: 'admin-a', reply_version: '4', content: 'old reply' } })
  await f.load(); f.text.value = 'new draft'; await f.save(); assert.equal(f.current.value.content, 'old reply'); assert.equal(f.text.value, 'new draft')
})

test('switching accounts or root cannot overwrite another draft during editor reset', async () => {
  const f = fixture(async () => ({ id: 'reply', user_id: 'admin-a', reply_version: '1', content: 'server' }))
  f.drafts.value['admin-b:root-1'] = 'B draft'
  await f.load(); f.text.value = 'A draft'
  f.user.value = { id: 'admin-b' }; await vue.nextTick()
  assert.equal(f.drafts.value['admin-a:root-1'], 'A draft'); assert.equal(f.drafts.value['admin-b:root-1'], 'B draft')
  f.user.value = { id: 'admin-a' }; await vue.nextTick(); assert.equal(f.drafts.value['admin-a:root-1'], 'A draft')
  f.drafts.value['admin-a:root-2'] = 'other root draft'
  f.props.root = { id: 'root-2' }; await vue.nextTick()
  assert.equal(f.drafts.value['admin-a:root-2'], 'other root draft'); assert.equal(f.ready.value, false)
})
