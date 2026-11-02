import { getArticle, listComments, listMyComments, recheckComment, createComment, likeArticle, reportArticleViewed } from '../../utils/api'
import { toMessage } from '../../utils/request'
import { formatDate } from '../../utils/format'
import { articleContentBlocks, decorateArticleHtml, type ArticleContentBlock } from '../../utils/html'
import { isFavorite, toggleFavorite, isLiked, markLiked, unmarkLiked } from '../../utils/favorites'
import { currentAccount, hasRefreshSession, isLoggedIn, refreshSession } from '../../utils/auth'
import { assetUrl } from '../../utils/config'
import { COMMENT_STATUS_APPROVED, COMMENT_STATUS_PENDING, type Comment, type MiniCommentRecord } from '../../utils/types'

interface CommentItem {
  id: string
  name: string
  dateText: string
  content: string
  avatarUrl: string
  initial: string
  pending: boolean
  safetyLabel?: string
  canRecheck?: boolean
}

// toCommentItem 把 wire 评论转成视图模型；无名评论回退「访客」。
function toCommentItem(c: Comment): CommentItem {
  const displayName = c.display_name || '访客'
  return {
    id: c.id,
    name: displayName,
    dateText: formatDate(c.created_at),
    content: c.content,
    avatarUrl: assetUrl(c.avatar_url),
    initial: Array.from(displayName)[0] || '访',
    pending: c.status === COMMENT_STATUS_PENDING,
  }
}

function privateComment(record: MiniCommentRecord): CommentItem {
  const item = toCommentItem(record.comment)
  item.pending = !record.visible
  item.canRecheck = record.can_recheck
  item.safetyLabel = record.visible ? '' : ({
    0: '尚未检测，仅你可见', 1: '安全检测中，仅你可见',
    2: '安全检测通过，待管理员审核', 3: '待进一步审核，仅你可见',
    4: '未通过安全检测，仅你可见', 5: '检测暂不可用，仅你可见', 6: '已移除',
  } as Record<number, string>)[record.safety_state] || '待检测，仅你可见'
  return item
}

function pendingKey(accountId: string, slug: string): string {
  return `mini:comment-intent:${accountId}:${slug}`
}

function uniqueComments(comments: CommentItem[]): CommentItem[] {
  const ids = new Set<string>()
  return comments.filter((comment) => {
    if (ids.has(comment.id)) return false
    ids.add(comment.id)
    return true
  })
}

Page({
  data: {
    slug: '',
    loading: true,
    error: '',
    title: '',
    dateText: '',
    viewCount: 0,
    categoryName: '',
    tags: [] as string[],
    html: '',
    contentBlocks: [] as ArticleContentBlock[],
    imageUrls: [] as string[],
    likeCount: 0,
    liked: false,
    favorited: false,
    comments: [] as CommentItem[],
    publicComments: [] as CommentItem[],
    sessionComments: [] as CommentItem[],
    commentToken: '',
    commentsFinished: false,
    commentCount: 0,
    commentLoading: false,
    commentRevision: 0,
    lifecycleRevision: 0,
    commentSubmitRevision: 0,
    commentAccountId: '',
    mineRevision: 0,
    mineToken: '',
    mineFinished: false,
    mineLoading: false,
    retryingCommentId: '',
    pendingIntents: {} as Record<string, { content: string, requestId: string }>,
    commentInput: '',
    commentLength: 0,
    commentBusy: false,
    commentError: '',
    canComment: false,
    commentAuthLoading: false,
    commentAuthRevision: 0,
  },

  onLoad(options: Record<string, string | undefined>) {
    const slug = options.slug ?? ''
    this.setData({ slug })
    if (slug) {
      this.fetch()
      this.fetchComments()
      this.refreshComposer()
    } else {
      this.setData({ loading: false, error: '缺少文章标识' })
    }
  },

  onShow() {
    // 从「我的」页登录回来后刷新输入框可用态。
    this.refreshComposer()
  },

  onHide() {
    this.setData({ commentAuthRevision: this.data.commentAuthRevision + 1, commentAuthLoading: false })
  },

  onUnload() {
    // Invalidate callbacks without updating the destroyed view.
    this.data.commentAuthRevision += 1
    this.data.mineRevision += 1
    this.data.commentSubmitRevision += 1
    this.data.commentRevision += 1
    this.data.lifecycleRevision += 1
  },

  async refreshComposer() {
    const revision = this.data.commentAuthRevision + 1
    this.setData({ commentAuthRevision: revision })
    if (!isLoggedIn() && hasRefreshSession()) {
      this.setData({ canComment: false, commentAuthLoading: true })
      try {
        // onLoad/onShow and request-layer 401 recovery share auth's single flight.
        await refreshSession()
      } catch { /* Failed restoration falls back to the current local auth state. */ }
      if (revision !== this.data.commentAuthRevision) return
    }
    // Read current state after waiting: a newer login/logout always wins.
    this.applyComposerState()
  },

  applyComposerState() {
    const accountId = currentAccount()?.id ?? ''
    const previousAccountId = this.data.commentAccountId
    const accountChanged = this.data.commentAccountId !== '' && this.data.commentAccountId !== accountId
    if (accountChanged) {
      const comments = uniqueComments(this.data.publicComments)
      this.setData({
        canComment: isLoggedIn(),
        commentAuthLoading: false,
        commentAccountId: accountId,
        commentSubmitRevision: this.data.commentSubmitRevision + 1,
        mineRevision: this.data.mineRevision + 1, mineToken: '', mineFinished: false, mineLoading: false, retryingCommentId: '',
        sessionComments: [],
        comments,
        commentCount: comments.length,
        commentInput: '',
        commentLength: 0,
        commentBusy: false,
        commentError: '',
      })
      this.restoreCommentIntent(accountId)
      if (accountId) void this.fetchMyComments(true)
      return
    }
    this.setData({ canComment: isLoggedIn(), commentAccountId: accountId, commentAuthLoading: false })
    if (accountId !== previousAccountId) this.restoreCommentIntent(accountId)
    if (accountId && !this.data.mineLoading) void this.fetchMyComments(true)
  },

  restoreCommentIntent(owner: string) {
    if (!owner || this.data.commentInput) return
    try {
      const intent = wx.getStorageSync(pendingKey(owner, this.data.slug))
      if (typeof intent?.content === 'string' && typeof intent?.requestId === 'string' && Array.from(intent.content).length <= 1000) {
        this.setData({ commentInput: intent.content, commentLength: Array.from(intent.content).length })
      }
    } catch { /* storage unavailable: keep the current in-page draft */ }
  },

  async fetch() {
    const { slug } = this.data
    this.setData({ loading: true, error: '' })
    try {
      const article = await getArticle(slug)
      const html = decorateArticleHtml(article.content_html ?? '')
      const contentBlocks = articleContentBlocks(html)
      wx.setNavigationBarTitle({ title: article.title })
      this.setData({
        loading: false,
        title: article.title,
        dateText: formatDate(article.published_at),
        viewCount: Number(article.view_count ?? 0),
        likeCount: Number(article.like_count ?? 0),
        categoryName: article.category?.name ?? '',
        tags: article.tags ?? [],
        html,
        contentBlocks,
        imageUrls: Array.from(new Set(contentBlocks.flatMap(block => block.imageUrls))),
        liked: isLiked(slug),
        favorited: isFavorite(slug),
      })
      // 与 Web 端口径一致：进详情即上报，服务端 24h 去重，失败静默。
      reportArticleViewed(slug).catch(() => {})
    } catch (error) {
      this.setData({ loading: false, error: toMessage(error) })
    }
  },

  retry() {
    this.fetch()
    this.fetchComments()
  },

  previewPostImage(event: WechatMiniprogram.TouchEvent) {
    const src: unknown = event.currentTarget.dataset.src
    if (typeof src !== 'string' || !this.data.imageUrls.includes(src)) return
    wx.previewImage({ current: src, urls: this.data.imageUrls })
  },

  async toggleLike() {
    const { slug, liked, likeCount } = this.data
    if (liked) return
    markLiked(slug)
    this.setData({ liked: true, likeCount: likeCount + 1 })
    try {
      await likeArticle(slug)
    } catch (error) {
      // 后端失败就回滚本地态（24h 去重失败极少见，多数是网络）。
      unmarkLiked(slug)
      this.setData({ liked: false, likeCount })
      wx.showToast({ title: toMessage(error), icon: 'none' })
    }
  },

  toggleFavorite() {
    const { slug, title } = this.data
    const next = toggleFavorite(slug, title || '文章')
    this.setData({ favorited: next })
    wx.showToast({ title: next ? '已收藏' : '已取消收藏', icon: 'none' })
  },

  async fetchComments(reset = false) {
    if (!reset && (this.data.commentLoading || this.data.commentsFinished)) return

    let revision = this.data.commentRevision
    if (reset) {
      revision += 1
      this.setData({ commentRevision: revision })
    }

    const { slug } = this.data
    const commentToken = reset ? '' : this.data.commentToken
    const publicComments = reset ? [] : this.data.publicComments
    this.setData({ commentLoading: true })
    try {
      const set = await listComments(slug, commentToken)
      if (revision !== this.data.commentRevision) return
      if (this.data.commentAccountId !== (currentAccount()?.id ?? '')) this.applyComposerState()
      const more = (set.comments ?? [])
        .filter((comment) => comment.status === COMMENT_STATUS_APPROVED)
        .map(toCommentItem)
      const nextPublic = uniqueComments(publicComments.concat(more))
      const publicIds = new Set(nextPublic.map((comment) => comment.id))
      const sessionComments = this.data.sessionComments.filter((comment) => !publicIds.has(comment.id))
      const comments = uniqueComments(sessionComments.concat(nextPublic))
      const nextToken = set.next_page_token ?? ''
      this.setData({
        publicComments: nextPublic,
        sessionComments,
        comments,
        commentToken: nextToken,
        commentsFinished: nextToken === '',
        commentCount: nextPublic.length,
      })
    } catch (error) {
      if (revision === this.data.commentRevision) {
        if (this.data.commentAccountId !== (currentAccount()?.id ?? '')) this.applyComposerState()
        wx.showToast({ title: toMessage(error), icon: 'none' })
      }
    } finally {
      if (revision === this.data.commentRevision) {
        this.setData({ commentLoading: false })
      }
    }
  },

  async fetchMyComments(reset = false) {
    const owner = currentAccount()?.id ?? ''
    if (!owner || (!reset && (this.data.mineLoading || this.data.mineFinished))) return
    const revision = this.data.mineRevision + (reset ? 1 : 0)
    this.setData({ mineRevision: revision, mineLoading: true })
    const previous = reset ? [] : this.data.sessionComments
    try {
      const result = await listMyComments(this.data.slug, reset ? '' : this.data.mineToken)
      if (revision !== this.data.mineRevision) return
      if (owner !== (currentAccount()?.id ?? '')) { this.applyComposerState(); return }
      const items = (result.records ?? []).filter(r => r.comment?.user_id === owner && r.safety_state !== 6).map(privateComment)
      const sessionComments = uniqueComments(previous.concat(items))
      const privateIds = new Set(sessionComments.map(c => c.id))
      const heldIds = new Set(sessionComments.filter(c => c.pending).map(c => c.id))
      const publicComments = this.data.publicComments.filter(c => !heldIds.has(c.id))
      this.setData({ publicComments, commentCount: publicComments.length, sessionComments, comments: uniqueComments(sessionComments.concat(publicComments.filter(c => !privateIds.has(c.id)))), mineToken: result.next_page_token ?? '', mineFinished: !result.next_page_token })
    } catch {
      if (revision === this.data.mineRevision && owner !== (currentAccount()?.id ?? '')) this.applyComposerState()
    }
    finally {
      if (revision === this.data.mineRevision && owner === (currentAccount()?.id ?? '')) this.setData({ mineLoading: false })
    }
  },

  async retryCommentSafety(event: WechatMiniprogram.TouchEvent) {
    const id = event.currentTarget.dataset.id
    const owner = currentAccount()?.id ?? ''
    if (!owner || typeof id !== 'string' || this.data.retryingCommentId || !this.data.sessionComments.some(c => c.id === id && c.canRecheck)) return
    const revision = this.data.mineRevision
    const lifecycle = this.data.lifecycleRevision
    this.setData({ retryingCommentId: id })
    try {
      const result = await recheckComment(id)
      if (revision !== this.data.mineRevision) return
      if (owner !== (currentAccount()?.id ?? '')) { this.applyComposerState(); return }
      if (result.comment?.user_id !== owner) return
      const item = privateComment(result)
      const sessionComments = [item].concat(this.data.sessionComments.filter(c => c.id !== id))
      const publicComments = item.pending ? this.data.publicComments.filter(c => c.id !== id) : this.data.publicComments
      this.setData({ publicComments, commentCount: publicComments.length, sessionComments, comments: uniqueComments(sessionComments.concat(publicComments)) })
      if (result.visible) await this.fetchComments(true)
    } catch (error) {
      if (revision === this.data.mineRevision && owner !== (currentAccount()?.id ?? '')) this.applyComposerState()
      if (owner === (currentAccount()?.id ?? '') && revision === this.data.mineRevision) wx.showToast({ title: toMessage(error), icon: 'none' })
    } finally {
      if (lifecycle === this.data.lifecycleRevision && owner === (currentAccount()?.id ?? '') && this.data.retryingCommentId === id) this.setData({ retryingCommentId: '' })
    }
  },

  loadMoreComments() {
    void this.fetchMyComments()
    return this.fetchComments()
  },

  onCommentInput(event: WechatMiniprogram.Input) {
    const commentInput = event.detail.value
    const commentLength = Array.from(commentInput).length
    const owner = currentAccount()?.id ?? ''
    if (owner && commentInput.trim() !== this.data.commentInput.trim()) {
      const key = pendingKey(owner, this.data.slug)
      delete this.data.pendingIntents[key]
      try { wx.removeStorageSync(key) } catch { /* the current new draft stays in memory */ }
    }
    this.setData({
      commentInput,
      commentLength,
      commentError: commentLength > 1000 ? '评论不能超过 1000 个字' : '',
    })
  },

  async submitComment() {
    if (this.data.commentBusy) return
    const { slug, commentInput } = this.data
    const content = commentInput.trim()
    if (!content || !isLoggedIn()) return
    if (Array.from(content).length > 1000) {
      this.setData({ commentError: '评论不能超过 1000 个字' })
      return
    }
    const submitAccountId = currentAccount()?.id ?? ''
    const submitRevision = this.data.commentSubmitRevision + 1
    this.setData({ commentBusy: true, commentError: '', commentSubmitRevision: submitRevision })
    try {
      const key = pendingKey(submitAccountId, slug)
      let intent = this.data.pendingIntents[key]
      try { intent = wx.getStorageSync(key) || intent } catch { /* storage can be unavailable */ }
      if (!intent || intent.content !== content || typeof intent.requestId !== 'string') {
        intent = { content, requestId: `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}-${Math.random().toString(36).slice(2)}` }
        try { wx.setStorageSync(key, intent) } catch { /* retain in-page below */ }
      }
      this.data.pendingIntents[key] = intent
      const record = await createComment(slug, content, intent.requestId)
      const createdComment = record.comment
      if (!createdComment) throw new Error('评论状态暂不可用，请稍后查看我的评论')
      const currentAccountId = currentAccount()?.id ?? ''
      if (submitRevision !== this.data.commentSubmitRevision) return
      if (currentAccountId !== submitAccountId) { this.applyComposerState(); return }
      if (!createdComment.id || createdComment.user_id !== submitAccountId) {
        this.setData({ commentBusy: false, commentError: '评论账号信息异常，请重试' })
        return
      }
      this.setData({ mineRevision: this.data.mineRevision + 1, mineLoading: false })
      const unchangedDraft = this.data.commentInput.trim() === content
      const created = privateComment(record)
      if (this.data.pendingIntents[key]?.requestId === intent.requestId) delete this.data.pendingIntents[key]
      try {
        if (wx.getStorageSync(key)?.requestId === intent.requestId) wx.removeStorageSync(key)
      } catch { /* no credentials are stored here */ }
      const sessionComments = uniqueComments([created].concat(this.data.sessionComments))
      const comments = uniqueComments(sessionComments.concat(this.data.publicComments))
      this.setData({
        sessionComments,
        comments,
        commentCount: this.data.publicComments.length + (record.visible && !this.data.publicComments.some(c => c.id === created.id) ? 1 : 0),
        commentInput: unchangedDraft ? '' : this.data.commentInput,
        commentLength: unchangedDraft ? 0 : Array.from(this.data.commentInput).length,
        commentBusy: false,
      })
      if (created.pending) {
        wx.showToast({ title: created.safetyLabel || '评论待审核，仅你可见', icon: 'none' })
      } else {
        wx.showToast({ title: '评论已发布', icon: 'success' })
        await this.fetchComments(true)
      }
    } catch (error) {
      const currentAccountId = currentAccount()?.id ?? ''
      if (submitRevision !== this.data.commentSubmitRevision) return
      if (currentAccountId !== submitAccountId) { this.applyComposerState(); return }
      if (currentAccountId === submitAccountId) {
        this.setData({ commentBusy: false, commentError: toMessage(error) })
      }
    }
  },

  goLogin() {
    wx.switchTab({ url: '/pages/me/me' })
  },

  onShareAppMessage() {
    return {
      title: this.data.title || '罗浩的博客',
      path: `/pages/post/post?slug=${encodeURIComponent(this.data.slug)}`,
    }
  },
})
