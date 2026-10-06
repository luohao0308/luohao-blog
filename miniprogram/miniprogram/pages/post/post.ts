import { getArticle, reportArticleViewed } from '../../utils/api'
import { toMessage } from '../../utils/request'
import { formatDate } from '../../utils/format'
import { decorateArticleHtml } from '../../utils/html'

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
  },

  onLoad(options: Record<string, string | undefined>) {
    const slug = options.slug ?? ''
    this.setData({ slug })
    if (slug) {
      this.fetch()
    } else {
      this.setData({ loading: false, error: '缺少文章标识' })
    }
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
        categoryName: article.category?.name ?? '',
        tags: article.tags ?? [],
        html: decorateArticleHtml(article.content_html ?? ''),
      })
      // 与 Web 端口径一致：进详情即上报，服务端 24h 去重，失败静默。
      reportArticleViewed(slug).catch(() => {})
    } catch (error) {
      this.setData({ loading: false, error: toMessage(error) })
    }
  },

  retry() {
    this.fetch()
  },

  onShareAppMessage() {
    return {
      title: this.data.title || '罗浩的博客',
      path: `/pages/post/post?slug=${encodeURIComponent(this.data.slug)}`,
    }
  },
})
