import type { Article } from '~/composables/useArticles'

export interface ArticleSeries {
  slug: string
  title: string
  description: string
  cover: string
  coverAlt: string
  topics: string[]
  articleSlugs: string[]
}

export const articleSeries: ArticleSeries[] = [
  {
    slug: 'blog-engineering',
    title: '博客工程实践',
    cover: '/images/series/blog-engineering.svg',
    coverAlt: '博客工程实践：从内容模型到可靠交付',
    topics: ['内容模型', '数据关系', '编辑保护', '搜索同步'],
    description: '后台验收、Ent 关系、MySQL 索引、并发编辑与搜索同步。',
    articleSlugs: ['fullstack-smoke-pitfalls', 'ent-m2m-edge-case-study', 'mysql-varchar-191-case-study', 'article-cas-revisions-case-study', 'article-search-sync-case-study'],
  },
  {
    slug: 'ai-delivery',
    title: 'AI 与交付实践',
    cover: '/images/series/ai-delivery.svg',
    coverAlt: 'AI 与交付实践：让能力进入真实工作流',
    topics: ['仓库规则', '交付守卫', 'Agent', 'AIGC'],
    description: 'dev-workflow、交付守卫、Forge Harness 和 AIGC 平台的开发记录。',
    articleSlugs: ['dev-workflow-project', 'ai-delivery-guard', 'forge-harness-project', 'aigc-backend-delivery'],
  },
]

// Editorial order is independent of publication dates. Missing/private entries
// never become placeholders, counts, or navigation destinations.
export function seriesArticles(series: ArticleSeries, articles: Article[]): Article[] {
  const published = new Map(articles.filter(article => article.status === 2).map(article => [article.slug, article]))
  return [...new Set(series.articleSlugs)].flatMap(slug => published.get(slug) ? [published.get(slug)!] : [])
}

export function seriesPosition(series: ArticleSeries, articles: Article[], slug: string) {
  const ordered = seriesArticles(series, articles)
  const index = ordered.findIndex(article => article.slug === slug)
  return index < 0 ? undefined : { index, total: ordered.length, previous: ordered[index - 1], next: ordered[index + 1] }
}
