import { getArticle, listComments, createComment, likeArticle, reportArticleViewed } from '../../utils/api'
import { toMessage } from '../../utils/request'
import { formatDate } from '../../utils/format'
import { articleContentBlocks, decorateArticleHtml, type ArticleContentBlock } from '../../utils/html'
import { isFavorite, toggleFavorite, isLiked, markLiked, unmarkLiked } from '../../utils/favorites'
import { currentAccount, hasRefreshSession, isLoggedIn, refreshSession } from '../../utils/auth'
import { assetUrl } from '../../utils/config'
import { COMMENT_STATUS_APPROVED, COMMENT_STATUS_PENDING, type Comment } from '../../utils/types'

interface CommentItem {
  id: string
  name: string
  dateText: string
  content: string
  avatarUrl: string
  initial: string
  pending: boolean
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
    commentSubmitRevision: 0,
    commentAccountId: '',
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
    const accountChanged = this.data.commentAccountId !== '' && this.data.commentAccountId !== accountId
    if (accountChanged) {
      const comments = uniqueComments(this.data.publicComments)
      this.setData({
        canComment: isLoggedIn(),
        commentAuthLoading: false,
        commentAccountId: accountId,
        commentSubmitRevision: this.data.commentSubmitRevision + 1,
        sessionComments: [],
        comments,
        commentCount: comments.length,
        commentInput: '',
        commentLength: 0,
        commentBusy: false,
        commentError: '',
      })
      return
    }
    this.setData({ canComment: isLoggedIn(), commentAccountId: accountId, commentAuthLoading: false })
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
        commentCount: comments.filter((comment) => !comment.pending).length,
      })
    } catch (error) {
      if (revision === this.data.commentRevision) {
        wx.showToast({ title: toMessage(error), icon: 'none' })
      }
    } finally {
      if (revision === this.data.commentRevision) {
        this.setData({ commentLoading: false })
      }
    }
  },

  loadMoreComments() {
    return this.fetchComments()
  },

  onCommentInput(event: WechatMiniprogram.Input) {
    const commentInput = event.detail.value
    const commentLength = Array.from(commentInput).length
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
    if (!content) return
    if (Array.from(content).length > 1000) {
      this.setData({ commentError: '评论不能超过 1000 个字' })
      return
    }
    const submitAccountId = currentAccount()?.id ?? ''
    const submitRevision = this.data.commentSubmitRevision + 1
    this.setData({ commentBusy: true, commentError: '', commentSubmitRevision: submitRevision })
    try {
      const createdComment = await createComment(slug, content)
      const currentAccountId = currentAccount()?.id ?? ''
      if (submitRevision !== this.data.commentSubmitRevision || currentAccountId !== submitAccountId) return
      if (createdComment.user_id && createdComment.user_id !== submitAccountId) {
        this.setData({ commentBusy: false, commentError: '评论账号信息异常，请重试' })
        return
      }
      const created = toCommentItem(createdComment)
      const sessionComments = uniqueComments([created].concat(this.data.sessionComments))
      const comments = uniqueComments(sessionComments.concat(this.data.publicComments))
      this.setData({
        sessionComments,
        comments,
        commentCount: comments.filter((comment) => !comment.pending).length,
        commentInput: '',
        commentLength: 0,
        commentBusy: false,
      })
      if (created.pending) {
        wx.showToast({ title: '评论审核中，仅你当前可见', icon: 'none' })
      } else {
        wx.showToast({ title: '评论已发布', icon: 'success' })
        await this.fetchComments(true)
      }
    } catch (error) {
      const currentAccountId = currentAccount()?.id ?? ''
      if (submitRevision === this.data.commentSubmitRevision && currentAccountId === submitAccountId) {
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
