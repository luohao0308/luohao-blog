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
function deferred() {
  let resolve
  const promise = new Promise(done => { resolve = done })
  return { promise, resolve }
}
global.Page = options => { page = options }
global.wx = {
  getStorageSync: key => storage.get(key),
  setStorageSync: (key, value) => storage.set(key, structuredClone(value)),
  removeStorageSync: key => storage.delete(key),
  login: options => queueMicrotask(() => options.success({ code: 'synthetic-wechat-code' })),
  request: options => {
    calls.push(options)
    queueMicrotask(() => Promise.resolve(respond(options)).then(options.success, options.fail))
  },
  showToast() {}, setNavigationBarTitle() {}, pageScrollTo() {},
  navigateTo: options => { navigation = options.url },
}
async function main() {
  execFileSync(path.join(root, 'node_modules/.bin/tsc'), ['--project', root, '--noEmit', 'false', '--outDir', out], { stdio: 'inherit' })
  const api = require(path.join(out, 'utils/api.js'))
  const auth = require(path.join(out, 'utils/auth.js'))
  const config = require(path.join(out, 'utils/config.js'))
  const { decorateArticleHtml } = require(path.join(out, 'utils/html.js'))
  // ArticleForm inserts root-relative API images. rich-text needs absolute URLs.
  const relativeImage = '/api/v1/assets/media/image-fixture.jpg'
  const absoluteImage = 'https://makerhao.cn' + relativeImage
  for (const src of [relativeImage, '/v1/assets/media/image-fixture.jpg', absoluteImage]) {
    assert.match(decorateArticleHtml(`<p>Before</p><img src="${src}" alt="正文图"><p>After</p>`), new RegExp(`src="${absoluteImage.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}"`))
  }
  assert.ok(decorateArticleHtml("<IMG SRC='/api/v1/assets/media/image-fixture.jpg'>").includes(`SRC='${absoluteImage}'`))
  assert.ok(decorateArticleHtml('<img data-src="unchanged" src="/api/v1/assets/media/image-fixture.jpg">').includes('data-src="unchanged"'))
  assert.ok(decorateArticleHtml('<img src="https://images.example.invalid/public.jpg">').includes('src="https://images.example.invalid/public.jpg"'))
  const favorites = require(path.join(out, 'utils/favorites.js'))
  const request = require(path.join(out, 'utils/request.js'))
  const projectConfig = JSON.parse(fs.readFileSync(path.join(root, 'project.config.json'), 'utf8'))
  assert.equal(config.API_BASE, 'https://makerhao.cn/api/v1')
  assert.equal(config.assetUrl('/v1/assets/avatars/fixture.png'), 'https://makerhao.cn/api/v1/assets/avatars/fixture.png')
  assert.equal(projectConfig.setting.urlCheck, true, 'production domain validation must be enabled by default')
  // Actual protojson wire DTOs must map consistently on all profile/session paths.
  const wireUser = { id: 'profile-fixture', email: 'profile@example.invalid', display_name: '测试昵称', role: 2, avatar_url: '/v1/assets/avatars/fixture.webp' }
  const avatar = 'https://makerhao.cn/api/v1/assets/avatars/fixture.webp'
  const expectedAccount = { id: wireUser.id, email: wireUser.email, displayName: wireUser.display_name, role: 2, avatarUrl: avatar }
  const loginReply = { access_token: 'profile-access-fixture', token_type: 'Bearer', expires_in: '900', user: wireUser }
  const me = loadPage('me')
  respond = options => ({ statusCode: 200, header: { 'Set-Cookie': 'refresh_token=profile-refresh-fixture; HttpOnly' }, data: options.url.endsWith('/auth/wechat') ? { status: 1, login: loginReply } : loginReply })
  await me.startWechatLogin()
  assert.deepEqual(me.data.account, expectedAccount, 'first WeChat login must map actual wire nickname and final image src')
  assert.equal(me.data.mode, 'signedin')
  assert.deepEqual(auth.currentAccount(), expectedAccount)
  assert.deepEqual(storage.get('auth:account'), expectedAccount)
  assert.equal(storage.get('auth:refresh'), 'profile-refresh-fixture')
  me.data.mode = 'binding'
  me.data.bindTicket = 'synthetic-binding-ticket'
  me.data.email = 'profile@example.invalid'
  me.data.password = 'synthetic-test-only-password'
  await me.submitBind()
  assert.deepEqual(me.data.account, expectedAccount, 'binding must map actual wire nickname/avatar')
  assert.equal(me.data.password, '')
  respond = () => ({ statusCode: 200, header: {}, data: wireUser })
  assert.deepEqual(await auth.fetchMe(), expectedAccount)
  await me.restore()
  assert.deepEqual(me.data.account, expectedAccount, 'restore must not double-prefix normalized avatar')
  respond = () => ({ statusCode: 200, header: { 'Set-Cookie': 'refresh_token=rotated-profile-fixture; HttpOnly' }, data: loginReply })
  assert.equal(await auth.refreshSession(), true)
  assert.deepEqual(auth.currentAccount(), expectedAccount, 'refresh must normalize before caching')
  assert.equal(storage.get('auth:refresh'), 'rotated-profile-fixture')
  storage.set('auth:account', wireUser)
  assert.deepEqual(auth.currentAccount(), expectedAccount, 'legacy snake_case cached user must restore')
  storage.set('auth:account', expectedAccount)
  assert.deepEqual(auth.currentAccount(), expectedAccount, 'normalized cache must remain idempotent')
  assert.equal(config.assetUrl(avatar), avatar)
  assert.equal(config.assetUrl('/api/v1/assets/avatars/fixture.webp'), avatar)
  for (const route of ['avatars', 'media']) {
    for (const filename of ['fixture.png', 'a123456789abcdef0123456789abcdef.jpg', 'a123456789abcdef0123456789abcdef.webp']) {
      for (const prefix of ['/v1/assets/', '/api/v1/assets/', 'https://makerhao.cn/api/v1/assets/']) {
        assert.equal(config.assetUrl(prefix + route + '/' + filename), 'https://makerhao.cn/api/v1/assets/' + route + '/' + filename)
      }
    }
  }
  for (const prefix of ['/v1/assets/', '/api/v1/assets/', 'https://makerhao.cn/api/v1/assets/']) {
    for (const suffix of ['../auth/me', 'avatars/./fixture.png', 'avatars/../auth/me', 'avatars/%2e%2e/auth/me', 'avatars/%2E%2E%2Fauth%2Fme', 'avatars/fixture%2epng', 'avatars/fixture%zz.png', 'avatars/fixture.png?x=1', 'avatars/fixture.png#fragment', 'avatars/nested/fixture.png', 'avatar/fixture.png', 'avatars/fixture.svg', 'avatars/fixture.png/']) {
      assert.equal(config.assetUrl(prefix + suffix), '', 'asset URL must reject traversal/encoding/query/hash and unknown routes: ' + suffix)
    }
    assert.equal(config.assetUrl(prefix + 'avatars/' + String.fromCharCode(92) + 'fixture.png'), '')
    assert.equal(config.assetUrl(prefix + 'avatars/fixture.png' + String.fromCharCode(10)), '')
  }
  assert.equal(config.assetUrl(''), '')
  for (const unsafe of ['http://makerhao.cn/api/v1/assets/a.png', '//other.example/a.png', 'javascript:alert(1)', 'https://other.example/a.png']) assert.equal(config.assetUrl(unsafe), '')
  respond = () => ({ statusCode: 200, header: {}, data: { ...wireUser, avatar_url: '' } })
  await me.restore()
  assert.equal(me.data.account.avatarUrl, '')
  assert.equal(me.data.account.displayName, wireUser.display_name)
  storage.set('auth:expireAt', 0)
  respond = options => ({ statusCode: 200, header: {}, data: options.url.endsWith('/auth/refresh') ? loginReply : wireUser })
  await me.restore()
  assert.deepEqual(me.data.account, expectedAccount, 'expired session restore must map both refresh and getMe responses')
  assert.match(fs.readFileSync(path.join(root, 'miniprogram/pages/me/me.wxml'), 'utf8'), /src="{{account\.avatarUrl}}"/)
  auth.clearSession()
  calls = []
  storage.set('auth:access', 'expired-fixture')
  storage.set('auth:refresh', 'refresh-fixture')
  auth.registerAuthBridge()
  let mutations = 0
  respond = options => {
    if (options.url.endsWith('/auth/refresh')) return { statusCode: 200, header: {}, data: { access_token: 'fresh-fixture', expires_in: '900', user: { id: 'fixture' } } }
    mutations++
    return { statusCode: mutations === 1 ? 401 : 200, header: {}, data: { id: 'comment-fixture' } }
  }
  const replayedComment = await api.createComment('fixture', 'one comment')
  assert.equal(replayedComment.id, 'comment-fixture')
  assert.equal(mutations, 2, '401 refresh must replay a POST exactly once')
  assert.equal(calls.filter(x => x.url.endsWith('/auth/refresh')).length, 1)
  assert.equal(calls.at(-1).url, 'https://makerhao.cn/api/v1/comments')
  assert.equal(calls.at(-1).header.Authorization, 'Bearer fresh-fixture')
  // A second 401 must stop, not enter an unbounded refresh loop.
  calls = []
  respond = options => options.url.endsWith('/auth/refresh')
    ? { statusCode: 200, header: {}, data: { access_token: 'fresh-fixture', expires_in: '900', user: {} } }
    : { statusCode: 401, header: {}, data: {} }
  await assert.rejects(api.createComment('fixture', 'denied'), request.UnauthorizedError)
  assert.equal(calls.length, 3)
  // Logout revokes the refresh session over the non-cookie header channel.
  calls = []
  storage.set('auth:refresh', 'logout-refresh-fixture')
  respond = () => ({ statusCode: 200, header: {}, data: {} })
  await auth.logout()
  assert.equal(calls.length, 1)
  assert.equal(calls[0].header['X-Refresh-Token'], 'logout-refresh-fixture')
  assert.equal(auth.currentAccount(), null)
  assert.equal(request.accessToken, '')

  // An in-flight rotation must not resurrect local auth or evade revocation.
  const rotating = deferred()
  const refreshStarted = deferred()
  calls = []
  storage.set('auth:refresh', 'before-rotation-fixture')
  storage.set('auth:access', 'before-rotation-access')
  auth.registerAuthBridge()
  respond = options => {
    if (options.url.endsWith('/auth/refresh')) {
      refreshStarted.resolve()
      return rotating.promise
    }
    return { statusCode: 200, header: {}, data: {} }
  }
  const oldRefresh = auth.refreshSession()
  await refreshStarted.promise
  const exiting = auth.logout()
  assert.equal(request.accessToken, '', 'logout clears local auth immediately')
  assert.equal(storage.has('auth:refresh'), false)
  assert.equal(await auth.refreshSession(), false, 'logout blocks new refresh attempts')
  rotating.resolve({ statusCode: 200, header: { 'Set-Cookie': 'refresh_token=after-rotation-fixture; HttpOnly' }, data: loginReply })
  assert.equal(await oldRefresh, false, 'old refresh cannot restore a logged-out session')
  await exiting
  assert.equal(calls.filter(x => x.url.endsWith('/auth/refresh')).length, 1)
  assert.equal(calls.at(-1).header['X-Refresh-Token'], 'after-rotation-fixture', 'logout must revoke the newly rotated token')
  assert.equal(auth.currentAccount(), null)
  assert.equal(request.accessToken, '')

  // Concurrent logout taps share one revoke, including a failed old refresh.
  const failedRotation = deferred()
  const failedRefreshStarted = deferred()
  calls = []
  storage.set('auth:refresh', 'failed-rotation-fixture')
  respond = options => {
    if (options.url.endsWith('/auth/refresh')) {
      failedRefreshStarted.resolve()
      return failedRotation.promise
    }
    return { statusCode: 200, header: {}, data: {} }
  }
  const failingRefresh = auth.refreshSession()
  await failedRefreshStarted.promise
  const firstLogout = auth.logout()
  assert.equal(auth.logout(), firstLogout, 'concurrent logout must not duplicate revoke')
  failedRotation.resolve({ statusCode: 503, header: {}, data: {} })
  assert.equal(await failingRefresh, false)
  await firstLogout
  assert.equal(calls.filter(x => x.url.endsWith('/auth/logout')).length, 1)
  assert.equal(calls.at(-1).header['X-Refresh-Token'], 'failed-rotation-fixture')

  // A stale refresh failure must not clear an explicit newer login.
  const staleFailure = deferred()
  const staleStarted = deferred()
  storage.set('auth:refresh', 'stale-refresh-fixture')
  respond = options => {
    if (options.url.endsWith('/auth/refresh')) {
      staleStarted.resolve()
      return staleFailure.promise
    }
    return { statusCode: 200, header: { 'Set-Cookie': 'refresh_token=new-login-fixture; HttpOnly' }, data: loginReply }
  }
  const staleRefresh = auth.refreshSession()
  await staleStarted.promise
  await auth.bindWechat('synthetic-ticket', 'profile@example.invalid', 'synthetic-password')
  staleFailure.resolve({ statusCode: 503, header: {}, data: {} })
  assert.equal(await staleRefresh, false)
  assert.equal(storage.get('auth:refresh'), 'new-login-fixture')
  assert.deepEqual(auth.currentAccount(), expectedAccount)

  // Logout after a new login must revoke that session, not an older rotation.
  const unrelatedRotation = deferred()
  const unrelatedStarted = deferred()
  calls = []
  storage.set('auth:refresh', 'unrelated-old-refresh-fixture')
  respond = options => {
    if (options.url.endsWith('/auth/refresh')) {
      unrelatedStarted.resolve()
      return unrelatedRotation.promise
    }
    if (options.url.endsWith('/auth/wechat/bind')) {
      return { statusCode: 200, header: { 'Set-Cookie': 'refresh_token=current-login-fixture; HttpOnly' }, data: loginReply }
    }
    return { statusCode: 200, header: {}, data: {} }
  }
  const unrelatedRefresh = auth.refreshSession()
  await unrelatedStarted.promise
  await auth.bindWechat('synthetic-ticket', 'profile@example.invalid', 'synthetic-password')
  const currentLogout = auth.logout()
  await Promise.resolve()
  const currentRevoke = calls.find(x => x.url.endsWith('/auth/logout'))
  assert.ok(currentRevoke, 'logout must not wait for an unrelated old session refresh')
  assert.equal(currentRevoke.header['X-Refresh-Token'], 'current-login-fixture')
  unrelatedRotation.resolve({ statusCode: 200, header: { 'Set-Cookie': 'refresh_token=unrelated-rotated-fixture; HttpOnly' }, data: loginReply })
  assert.equal(await unrelatedRefresh, false)
  await currentLogout
  assert.equal(auth.currentAccount(), null)
  assert.equal(request.accessToken, '')
  assert.equal(calls.filter(x => x.url.endsWith('/auth/logout')).length, 1)

  // Transport/401 failures never trigger an auth refresh and still clear local auth.
  const logoutWarnings = []
  const originalWarn = console.warn
  console.warn = (...args) => logoutWarnings.push(args)
  try {
    for (const failure of ['network', 'unauthorized']) {
      calls = []
      storage.set('auth:refresh', 'failure-refresh-fixture')
      respond = () => {
        if (failure === 'network') return Promise.reject(new Error('synthetic transport failure'))
        return { statusCode: 401, header: {}, data: {} }
      }
      await auth.logout()
      assert.equal(calls.length, 1)
      assert.equal(storage.has('auth:refresh'), false)
      assert.equal(request.accessToken, '')
    }
  } finally {
    console.warn = originalWarn
  }
  assert.deepEqual(logoutWarnings, [['logout revoke failed'], ['logout revoke failed']], 'logout failure diagnostics must never include transport errors or credentials')
  respond = () => ({ statusCode: 200, header: {}, data: loginReply })
  await auth.bindWechat('synthetic-ticket', 'profile@example.invalid', 'synthetic-password')
  assert.equal(favorites.toggleFavorite('fixture', 'Title'), true)
  assert.equal(favorites.isFavorite('fixture'), true)
  assert.equal(favorites.toggleFavorite('fixture', 'Title'), false)
  assert.equal(favorites.listFavorites().length, 0)
  const post = loadPage('post')
  post.data.slug = 'image-fixture'
  respond = () => ({ statusCode: 200, header: {}, data: { title: '正文图片测试', content_html: `<p>Before</p><img src="${relativeImage}" alt="正文图"><p>After</p>` } })
  await post.fetch()
  assert.equal(post.data.error, '')
  assert.ok(post.data.html.includes(`src="${absoluteImage}"`), 'actual article fetch must normalize image URLs before rich-text receives them')
  assert.ok(post.data.html.includes('max-width:100%'), 'body images must stay within the content width')
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
  storage.set('auth:account', { id: 'account-a', displayName: 'A' })
  storage.set('auth:expireAt', Date.now() + 60_000)
  post.refreshComposer()
  assert.equal(post.data.canComment, true)

  // Public GET contains approved comments only. Duplicate page taps must share one request,
  // and repeated IDs across token boundaries must not duplicate rendered comments.
  calls = []
  const firstPage = deferred()
  respond = options => options.url.includes('/comments?')
    ? firstPage.promise
    : ({ statusCode: 200, header: {}, data: {} })
  const firstLoad = post.fetchComments()
  const duplicateLoad = post.fetchComments()
  assert.equal(calls.filter(x => x.method === 'GET' && x.url.includes('/comments?')).length, 1)
  firstPage.resolve({ statusCode: 200, header: {}, data: {
    comments: [{ id: 'approved-1', content: 'approved one', status: 2 }],
    next_page_token: 'page-2',
  } })
  await Promise.all([firstLoad, duplicateLoad])
  assert.equal(post.data.commentLoading, false)
  assert.equal(post.data.commentCount, 1)
  respond = () => ({ statusCode: 200, header: {}, data: {
    comments: [
      { id: 'approved-1', content: 'approved one', status: 2 },
      { id: 'approved-2', content: 'approved two', status: 2 },
    ],
  } })
  await post.fetchComments()
  assert.deepEqual(post.data.comments.map(x => x.id), ['approved-1', 'approved-2'])
  assert.equal(post.data.commentCount, 2, 'count is the number of loaded public comments')

  // An approved create response is rendered immediately, then reconciled with a fresh
  // first page. A stale in-flight page must not overwrite the reset list.
  const stalePage = deferred()
  let commentGets = 0
  respond = options => options.method === 'GET'
    ? (++commentGets === 1
        ? stalePage.promise
        : ({ statusCode: 200, header: {}, data: { comments: [
            { id: 'approved-created', content: 'ordinary comment', status: 2 },
          ] } }))
    : ({ statusCode: 200, header: {}, data: {
        id: 'approved-created', article_slug: 'fixture', display_name: 'A',
        content: 'ordinary comment', status: 2, created_at: '2026-01-01T00:00:00Z',
      } })
  post.data.commentToken = 'stale-token'
  post.data.commentsFinished = false
  const staleLoad = post.fetchComments()
  post.data.commentInput = 'ordinary comment'
  const approvedSubmit = post.submitComment()
  await new Promise(resolve => setImmediate(resolve))
  assert.equal(post.data.comments.some(x => x.id === 'approved-created'), true)
  assert.equal(post.data.commentInput, '')
  stalePage.resolve({ statusCode: 200, header: {}, data: {
    comments: [{ id: 'stale', content: 'stale page', status: 2 }],
  } })
  await Promise.all([staleLoad, approvedSubmit])
  assert.equal(post.data.comments.some(x => x.id === 'stale'), false)
  assert.equal(post.data.comments.filter(x => x.id === 'approved-created').length, 1)

  // A failed first-page refresh keeps the last good public list. A later pending
  // submission must merge with that retained list instead of erasing it.
  respond = () => ({ statusCode: 503, header: {}, data: { message: 'refresh unavailable' } })
  await post.fetchComments(true)
  assert.equal(post.data.comments.some(x => x.id === 'approved-created'), true)

  // A pending response remains visible only as this account's session-local item;
  // the following public GET intentionally does not include it.
  let requestKind = 'create-pending'
  respond = options => {
    if (options.method === 'POST' && requestKind === 'create-pending') return {
      statusCode: 200, header: {}, data: {
        id: 'pending-created', article_slug: 'fixture', display_name: '😀作者', user_id: 'account-a',
        content: 'suspicious comment', status: 1, created_at: '2026-01-01T00:00:00Z',
      },
    }
    return { statusCode: 200, header: {}, data: { comments: [
      { id: 'approved-created', content: 'ordinary comment', status: 2 },
    ] } }
  }
  post.data.commentInput = 'suspicious comment'
  await post.submitComment()
  assert.equal(post.data.comments.some(x => x.id === 'approved-created'), true)
  assert.equal(post.data.comments.some(x => x.id === 'pending-created'), true)
  requestKind = 'public'
  await post.fetchComments(true)
  assert.equal(post.data.comments.find(x => x.id === 'pending-created').pending, true)
  assert.equal(post.data.comments.find(x => x.id === 'pending-created').initial, '😀')
  assert.equal(post.data.commentCount, 1)

  // Once moderation publishes the same ID, the public version replaces the local pending
  // version instead of being hidden behind it.
  respond = () => ({ statusCode: 200, header: {}, data: { comments: [
    { id: 'pending-created', content: 'suspicious comment', status: 2 },
  ] } })
  await post.fetchComments(true)
  assert.equal(post.data.comments.filter(x => x.id === 'pending-created').length, 1)
  assert.equal(post.data.comments.find(x => x.id === 'pending-created').pending, false)
  const callsAtFinishedPage = calls.length
  await post.loadMoreComments()
  assert.equal(calls.length, callsAtFinishedPage, 'finished pagination must not restart from page one')

  // A POST started by account A must not insert its pending result after account B takes
  // over the page. Account changes also clear private draft and error state.
  const oldAccountCreate = deferred()
  respond = options => options.method === 'POST'
    ? oldAccountCreate.promise
    : ({ statusCode: 200, header: {}, data: { comments: [] } })
  post.data.commentInput = 'account A private draft'
  post.data.commentLength = Array.from(post.data.commentInput).length
  const oldAccountSubmit = post.submitComment()
  post.data.commentError = 'account A private error'
  storage.set('auth:account', { id: 'account-b', displayName: 'B' })
  post.refreshComposer()
  assert.equal(post.data.commentInput, '')
  assert.equal(post.data.commentError, '')
  oldAccountCreate.resolve({ statusCode: 200, header: {}, data: {
    id: 'old-account-pending', article_slug: 'fixture', display_name: 'A', user_id: 'account-a',
    content: 'account A private draft', status: 1,
  } })
  await oldAccountSubmit
  assert.equal(post.data.comments.some(x => x.id === 'old-account-pending'), false)

  // Even without a concurrent switch, a mismatched response author cannot become a
  // session-local pending item for the current account.
  post.data.commentInput = 'mismatched author'
  respond = () => ({ statusCode: 200, header: {}, data: {
    id: 'wrong-author-pending', article_slug: 'fixture', display_name: 'A', user_id: 'account-a',
    content: 'mismatched author', status: 1,
  } })
  await post.submitComment()
  assert.equal(post.data.comments.some(x => x.id === 'wrong-author-pending'), false)

  // Failed sends retain the draft and expose the inline error.
  post.data.commentInput = 'keep this draft'
  respond = () => ({ statusCode: 503, header: {}, data: { message: 'unavailable' } })
  await post.submitComment()
  assert.equal(post.data.commentInput, 'keep this draft')
  assert.match(post.data.commentError, /unavailable/)

  // Comment length is measured in Unicode code points: 1000 emoji pass, 1001 fail.
  const emoji = '😀'
  post.onCommentInput({ detail: { value: emoji.repeat(1000) } })
  assert.equal(post.data.commentLength, 1000)
  respond = () => ({ statusCode: 200, header: {}, data: {
    id: 'emoji-ok', article_slug: 'fixture', content: emoji.repeat(1000), status: 2,
  } })
  await post.submitComment()
  assert.equal(post.data.commentError, '')
  const callsAfter1000 = calls.length
  post.onCommentInput({ detail: { value: emoji.repeat(1001) } })
  assert.equal(post.data.commentLength, 1001)
  await post.submitComment()
  assert.equal(calls.length, callsAfter1000, '1001 code points must fail before transport')
  assert.match(post.data.commentError, /1000/)
  // Index filters must preserve literal tag/category values and ignore responses
  // from an older selection, including old pagination tokens and failures.
  const index = loadPage('index')
  const oddTag = 'C++ "quoted" \\ path'
  index.data.activeTag = oddTag
  assert.equal(index.currentFilter(), `tag:${JSON.stringify(oddTag)}`)
  index.data.activeCategory = 'category"\\value'
  assert.equal(index.currentFilter(), `category:${JSON.stringify(index.data.activeCategory)}`)
  index.data.activeCategory = ''
  const articleReply = (slugs, token = '') => ({ statusCode: 200, header: {}, data: {
    articles: slugs.map(slug => ({ slug, title: slug, status: 2, tags: [] })), next_page_token: token,
  } })
  const indexA = deferred()
  const indexB = deferred()
  respond = options => new URL(options.url).searchParams.get('filter') === 'tag:"A"' ? indexA.promise : indexB.promise
  index.data.activeTag = 'A'
  const loadA = index.loadFirst()
  index.data.activeTag = 'B'
  const loadB = index.loadFirst()
  indexB.resolve(articleReply(['b'], 'b-page-2'))
  await loadB
  indexA.resolve(articleReply(['stale-a'], 'a-page-2'))
  await loadA
  assert.deepEqual(index.data.articles.map(x => x.slug), ['b'], 'old filter must not overwrite a newer list')
  assert.equal(index.data.nextToken, 'b-page-2')
  const failedA = deferred()
  respond = options => new URL(options.url).searchParams.get('filter') === 'tag:"A"' ? failedA.promise : articleReply(['b'])
  index.data.activeTag = 'A'
  const failedLoadA = index.loadFirst()
  index.data.activeTag = 'B'
  await index.loadFirst()
  failedA.resolve({ statusCode: 503, header: {}, data: { message: 'old failure' } })
  await failedLoadA
  assert.equal(index.data.error, '', 'old filter errors must not cover the current successful list')

  index.data.nextToken = 'b-page-2'
  index.data.finished = false
  const staleMore = deferred()
  const freshFirst = deferred()
  respond = options => new URL(options.url).searchParams.get('page_token') ? staleMore.promise : freshFirst.promise
  const oldMore = index.loadMore()
  index.data.activeTag = 'C'
  const freshLoad = index.loadFirst()
  const callsBeforeBlockedMore = calls.length
  await index.loadMore()
  assert.equal(calls.length, callsBeforeBlockedMore, 'cannot send an old paging token while changing the filter')
  freshFirst.resolve(articleReply(['c', 'c'], 'c-page-2'))
  await freshLoad
  staleMore.resolve(articleReply(['stale-b'], 'stale-page-3'))
  await oldMore
  assert.deepEqual(index.data.articles.map(x => x.slug), ['c'])
  assert.equal(index.data.nextToken, 'c-page-2')
  respond = () => articleReply(['c', 'd', 'd'])
  await index.loadMore()
  assert.deepEqual(index.data.articles.map(x => x.slug), ['c', 'd'], 'duplicate slugs across pages render once')
  assert.equal(index.data.finished, true)
  const callsWhenFinished = calls.length
  await index.loadMore()
  assert.equal(calls.length, callsWhenFinished)
  respond = () => articleReply([])
  await index.setTag({ currentTarget: { dataset: { tag: 'missing-tag' } } })
  assert.equal(index.data.error, '')
  assert.equal(index.data.finished, true)
  assert.deepEqual(index.data.articles, [])

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
  console.log('Mini logic PASS: wire account/avatar mapping, auth replay/logout revocation and rotation races, comment status/rendering, index/comment paging races/dedup, Unicode limits, draft retention, search, chat')
}
main().catch(error => { console.error(error); process.exitCode = 1 }).finally(() => fs.rmSync(out, { recursive: true, force: true }))
