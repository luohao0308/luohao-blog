import { listArticles } from '../../utils/api'
import { toMessage } from '../../utils/request'
import { formatDate } from '../../utils/format'
import { ARTICLE_STATUS_PUBLISHED, type Article } from '../../utils/types'

interface PostListItem {
  slug: string
  title: string
  summary: string
  tags: string[]
  dateText: string
  viewCount: number
  categoryName: string
}

function toListItem(article: Article): PostListItem {
  return {
    slug: article.slug,
    title: article.title,
    summary: article.summary,
    tags: article.tags ?? [],
    dateText: formatDate(article.published_at),
    viewCount: article.view_count ?? 0,
    categoryName: article.category?.name ?? '',
  }
}

Page({
  data: {
    articles: [] as PostListItem[],
    nextToken: '',
    loading: true,
    loadingMore: false,
    finished: false,
    error: '',
  },

  onLoad() {
    this.loadFirst()
  },

  async loadFirst() {
    this.setData({ loading: true, error: '' })
    try {
      const set = await listArticles('')
      // 服务端对匿名读取已限定已发布，这里按 Web 端习惯再兜底过滤一次。
      const articles = (set.articles ?? [])
        .filter((article) => article.status === ARTICLE_STATUS_PUBLISHED)
        .map(toListItem)
      const nextToken = set.next_page_token ?? ''
      this.setData({
        articles,
        nextToken,
        loading: false,
        finished: nextToken === '',
      })
    } catch (error) {
      this.setData({ loading: false, error: toMessage(error) })
    }
  },

  async loadMore() {
    if (this.data.loadingMore || this.data.finished || this.data.nextToken === '') return
    this.setData({ loadingMore: true })
    try {
      const set = await listArticles(this.data.nextToken)
      const more = (set.articles ?? [])
        .filter((article) => article.status === ARTICLE_STATUS_PUBLISHED)
        .map(toListItem)
      const nextToken = set.next_page_token ?? ''
      this.setData({
        articles: this.data.articles.concat(more),
        nextToken,
        loadingMore: false,
        finished: nextToken === '',
      })
    } catch (error) {
      this.setData({ loadingMore: false })
      wx.showToast({ title: toMessage(error), icon: 'none' })
    }
  },

  onPullDownRefresh() {
    this.loadFirst().then(() => wx.stopPullDownRefresh())
  },

  onReachBottom() {
    this.loadMore()
  },

  openPost(event: WechatMiniprogram.TouchEvent) {
    const slug = event.currentTarget.dataset.slug as string
    wx.navigateTo({ url: `/pages/post/post?slug=${encodeURIComponent(slug)}` })
  },
})
