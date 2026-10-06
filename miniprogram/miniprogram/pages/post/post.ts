import { getArticle, listComments, createComment, likeArticle, reportArticleViewed } from '../../utils/api'
import { toMessage } from '../../utils/request'
import { formatDate } from '../../utils/format'
import { decorateArticleHtml } from '../../utils/html'
import { isFavorite, toggleFavorite, isLiked, markLiked } from '../../utils/favorites'
import { isLoggedIn } from '../../utils/auth'
import { assetUrl } from '../../utils/config'
import { COMMENT_STATUS_PENDING, type Comment } from '../../utils/types'

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
  return {
    id: c.id,
    name: c.display_name || '访客',
    dateText: formatDate(c.created_at),
    content: c.content,
    avatarUrl: assetUrl(c.avatar_url),
    initial: (c.display_name || '访')[0],
    pending: c.status === COMMENT_STATUS_PENDING,
  }
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
    likeCount: 0,
    liked: false,
    favorited: false,
    comments: [] as CommentItem[],
    commentToken: '',
    commentsFinished: false,
    commentCount: 0,
    commentInput: '',
    commentBusy: false,
    commentError: '',
    canComment: false,
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

  refreshComposer() {
    this.setData({ canComment: isLoggedIn() })
  },

  async fetch() {
    const { slug } = this.data
    this.setData({ loading: true, error: '' })
    try {
      const article = await getArticle(slug)
      wx.setNavigationBarTitle({ title: article.title })
      this.setData({
        loading: false,
        title: article.title,
        dateText: formatDate(article.published_at),
        viewCount: article.view_count ?? 0,
        likeCount: article.like_count ?? 0,
        categoryName: article.category?.name ?? '',
        tags: article.tags ?? [],
        html: decorateArticleHtml(article.content_html ?? ''),
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

  async toggleLike() {
    const { slug, liked, likeCount } = this.data
    if (liked) return
    markLiked(slug)
    this.setData({ liked: true, likeCount: likeCount + 1 })
    try {
      await likeArticle(slug)
    } catch (error) {
      // 后端失败就回滚本地态（24h 去重失败极少见，多数是网络）。
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

  async fetchComments() {
    const { slug, commentToken, comments } = this.data
    try {
      const set = await listComments(slug, commentToken)
      const more = (set.comments ?? []).map(toCommentItem)
      const nextToken = set.next_page_token ?? ''
      this.setData({
        comments: comments.concat(more),
        commentToken: nextToken,
        commentsFinished: nextToken === '',
        commentCount: comments.concat(more).filter((c) => !c.pending).length,
      })
    } catch (error) {
      wx.showToast({ title: toMessage(error), icon: 'none' })
    }
  },

  onCommentInput(event: WechatMiniprogram.Input) {
    this.setData({ commentInput: event.detail.value })
  },

  async submitComment() {
    if (this.data.commentBusy) return
    const { slug, commentInput } = this.data
    const content = commentInput.trim()
    if (!content) return
    this.setData({ commentBusy: true, commentError: '' })
    try {
      await createComment(slug, content)
      this.setData({ commentInput: '', commentBusy: false })
      wx.showToast({ title: '已提交，审核通过后展示', icon: 'none' })
      // 重新拉取：登录态下列表会带回自己的待审评论（带「审核中」标识）。
      this.setData({ comments: [], commentToken: '', commentsFinished: false })
      await this.fetchComments()
    } catch (error) {
      this.setData({ commentBusy: false, commentError: toMessage(error) })
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
