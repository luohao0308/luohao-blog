// Executable logic checks with a mocked wx transport; not a DevTools/real-device UI test.
const assert = require('node:assert/strict')
const { execFileSync } = require('node:child_process')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const root = path.resolve(__dirname, '..')
const out = fs.mkdtempSync(path.join(os.tmpdir(), 'blog-mini-verify-'))
const storage = new Map()
let respond = () => ({ statusCode: 200, data: {}, header: {} })
let calls = []
let page
let navigation
function loadPage(name) {
  require(path.join(out, 'pages', name, name + '.js'))
  const instance = { ...page, data: structuredClone(page.data) }
  instance.setData = patch => Object.assign(instance.data, patch)
  return instance
}
global.Page = options => { page = options }
global.wx = {
  getStorageSync: key => storage.get(key),
  setStorageSync: (key, value) => storage.set(key, structuredClone(value)),
  removeStorageSync: key => storage.delete(key),
  request: options => { calls.push(options); queueMicrotask(() => options.success(respond(options))) },
  showToast() {}, setNavigationBarTitle() {}, pageScrollTo() {},
  navigateTo: options => { navigation = options.url },
}
async function main() {
  execFileSync(path.join(root, 'node_modules/.bin/tsc'), ['--project', root, '--noEmit', 'false', '--outDir', out], { stdio: 'inherit' })
  const api = require(path.join(out, 'utils/api.js'))
  const auth = require(path.join(out, 'utils/auth.js'))
  const favorites = require(path.join(out, 'utils/favorites.js'))
  const request = require(path.join(out, 'utils/request.js'))
  storage.set('auth:access', 'expired-fixture')
  storage.set('auth:refresh', 'refresh-fixture')
  auth.registerAuthBridge()
  let mutations = 0
  respond = options => {
    if (options.url.endsWith('/auth/refresh')) return { statusCode: 200, header: {}, data: { access_token: 'fresh-fixture', expires_in: '900', user: { id: 'fixture' } } }
    mutations++
    return { statusCode: mutations === 1 ? 401 : 200, header: {}, data: { id: 'comment-fixture' } }
  }
  await api.createComment('fixture', 'one comment')
  assert.equal(mutations, 2, '401 refresh must replay a POST exactly once')
  assert.equal(calls.filter(x => x.url.endsWith('/auth/refresh')).length, 1)
  assert.equal(calls.at(-1).header.Authorization, 'Bearer fresh-fixture')
  // A second 401 must stop, not enter an unbounded refresh loop.
  calls = []
  respond = options => options.url.endsWith('/auth/refresh')
    ? { statusCode: 200, header: {}, data: { access_token: 'fresh-fixture', expires_in: '900', user: {} } }
    : { statusCode: 401, header: {}, data: {} }
  await assert.rejects(api.createComment('fixture', 'denied'), request.UnauthorizedError)
  assert.equal(calls.length, 3)
  assert.equal(favorites.toggleFavorite('fixture', 'Title'), true)
  assert.equal(favorites.isFavorite('fixture'), true)
  assert.equal(favorites.toggleFavorite('fixture', 'Title'), false)
  assert.equal(favorites.listFavorites().length, 0)
  const post = loadPage('post')
  post.data.slug = 'fixture'
  post.data.likeCount = 9
  respond = () => ({ statusCode: 503, header: {}, data: { message: 'unavailable' } })
  await post.toggleLike()
  assert.equal(post.data.liked, false)
  assert.equal(post.data.likeCount, 9)
  assert.equal(favorites.isLiked('fixture'), false, 'failed like must clear local dedup marker')
  respond = () => ({ statusCode: 200, header: {}, data: {} })
  await post.toggleLike()
  assert.equal(post.data.likeCount, 10)
  assert.equal(favorites.isLiked('fixture'), true)
  respond = () => ({ statusCode: 200, header: {}, data: { comments: [
    { id: 'pending', content: 'pending', status: 1 }, { id: 'approved', content: 'approved', status: 2 },
  ] } })
  await post.fetchComments()
  assert.equal(post.data.comments[0].pending, true)
  assert.equal(post.data.commentCount, 1)
  const search = loadPage('search')
  search.data.query = '  Ent & Go  '
  respond = () => ({ statusCode: 200, header: {}, data: { articles: [
    { slug: 'public', status: 2, title: 'Public' }, { slug: 'private', status: 1, title: 'Draft' },
  ] } })
  await search.doSearch()
  assert.deepEqual(search.data.results.map(x => x.slug), ['public'])
  assert.equal(new URL(calls.at(-1).url).searchParams.get('query'), 'Ent & Go')
  const chat = loadPage('chat')
  chat.data.input = 'Question'
  respond = () => ({ statusCode: 200, header: {}, data: { answer: 'Answer', citations: [{ slug: 'public', title: 'Public' }] } })
  await chat.send()
  assert.equal(chat.data.messages.at(-1).text, 'Answer')
  assert.equal(chat.data.messages.at(-1).citations[0].slug, 'public')
  assert.equal(calls.at(-1).timeout, 60000)
  chat.openCitation({ currentTarget: { dataset: { slug: 'public' } } })
  assert.equal(navigation, '/pages/post/post?slug=public')
  console.log('Mini logic PASS: single POST replay, repeated 401, favorites, like rollback, pending comments, search, chat citations')
}
main().catch(error => { console.error(error); process.exitCode = 1 }).finally(() => fs.rmSync(out, { recursive: true, force: true }))
