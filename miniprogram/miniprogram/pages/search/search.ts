import { searchArticles } from '../../utils/api'
import { toMessage } from '../../utils/request'
import { formatDate } from '../../utils/format'
import { ARTICLE_STATUS_PUBLISHED, type Article } from '../../utils/types'

interface ResultItem {
  slug: string
  title: string
  summary: string
  dateText: string
  tags: string[]
}

Page({
  data: {
    query: '',
    results: [] as ResultItem[],
    searched: false,
    loading: false,
    error: '',
  },

  onInput(event: WechatMiniprogram.Input) {
    this.setData({ query: event.detail.value })
  },

  // doSearch 回车或点按钮触发；空词不查。
  async doSearch() {
    const query = this.data.query.trim()
    if (!query || this.data.loading) return
    this.setData({ loading: true, error: '', searched: true })
    try {
      const set = await searchArticles(query)
      const results = (set.articles ?? [])
        .filter((article: Article) => article.status === ARTICLE_STATUS_PUBLISHED)
        .map((article: Article) => ({
          slug: article.slug,
          title: article.title,
          summary: article.summary,
          dateText: formatDate(article.published_at),
          tags: article.tags ?? [],
        }))
      this.setData({ results, loading: false })
    } catch (error) {
      this.setData({ loading: false, error: toMessage(error) })
    }
  },

  openPost(event: WechatMiniprogram.TouchEvent) {
    const slug = event.currentTarget.dataset.slug as string
    wx.navigateTo({ url: `/pages/post/post?slug=${encodeURIComponent(slug)}` })
  },
})
