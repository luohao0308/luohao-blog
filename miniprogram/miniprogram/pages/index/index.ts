import { listArticles, listCategories } from '../../utils/api'
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

Page({
  data: {
    articles: [] as PostListItem[],
    // next_token 非渲染数据，放在 data 里以便 Page 类型收拢。
    nextToken: '',
    loading: true,
    loadingMore: false,
    finished: false,
    error: '',
    categories: [] as Array<{ slug: string, name: string }>,
    tags: [] as string[],
    activeCategory: '',
    activeTag: '',
  },

  onLoad() {
    this.loadFirst()
    this.loadCategories()
  },

  // currentFilter 由激活的分类/标签拼出；两者互斥，分类优先。
  currentFilter(): string {
    const { activeCategory, activeTag } = this.data
    if (activeCategory) return `category:"${activeCategory}"`
    if (activeTag) return `tag:"${activeTag}"`
    return ''
  },

  async loadFirst() {
    this.setData({ loading: true, error: '' })
    try {
      const set = await listArticles('', 20, this.currentFilter())
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
      this.collectTags(articles)
    } catch (error) {
      this.setData({ loading: false, error: toMessage(error) })
    }
  },

  async loadMore() {
    if (this.data.loadingMore || this.data.finished || this.data.nextToken === '') return
    this.setData({ loadingMore: true })
    try {
      const set = await listArticles(this.data.nextToken, 20, this.currentFilter())
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
      this.collectTags(more)
    } catch (error) {
      this.setData({ loadingMore: false })
      wx.showToast({ title: toMessage(error), icon: 'none' })
    }
  },

  // collectTags 从已加载文章里收集标签（去重、最多 12 个）做筛选 chips。
  collectTags(items: PostListItem[]) {
    const seen = new Set(this.data.tags)
    for (const item of items) {
      for (const tag of item.tags) {
        if (!seen.has(tag) && seen.size < 12) {
          seen.add(tag)
        }
      }
    }
    this.setData({ tags: [...seen] })
  },

  async loadCategories() {
    try {
      const set = await listCategories()
      this.setData({ categories: set.categories ?? [] })
    } catch {
      // 分类空/接口异常不阻塞首页。
    }
  },

  onPullDownRefresh() {
    this.loadFirst().then(() => wx.stopPullDownRefresh())
  },

  onReachBottom() {
    this.loadMore()
  },

  async setCategory(event: WechatMiniprogram.TouchEvent) {
    const slug = (event.currentTarget.dataset.slug as string) || ''
    this.setData({ activeCategory: slug, activeTag: '' })
    await this.loadFirst()
  },

  async setTag(event: WechatMiniprogram.TouchEvent) {
    const tag = (event.currentTarget.dataset.tag as string) || ''
    this.setData({ activeTag: tag, activeCategory: '' })
    await this.loadFirst()
  },

  openSearch() {
    wx.navigateTo({ url: '/pages/search/search' })
  },

  openChat() {
    wx.navigateTo({ url: '/pages/chat/chat' })
  },

  openPost(event: WechatMiniprogram.TouchEvent) {
    const slug = event.currentTarget.dataset.slug as string
    wx.navigateTo({ url: `/pages/post/post?slug=${encodeURIComponent(slug)}` })
  },
})

function toListItem(article: Article): PostListItem {
  return {
    slug: article.slug,
    title: article.title,
    summary: article.summary,
    tags: article.tags ?? [],
    dateText: formatDate(article.published_at),
    viewCount: Number(article.view_count ?? 0),
    categoryName: article.category?.name ?? '',
  }
}
