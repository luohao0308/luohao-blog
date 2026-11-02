import type { Article } from '~/composables/useArticles'

export interface ArticleSeries {
  slug: string
  title: string
  description: string
  articleSlugs: string[]
}

export const articleSeries: ArticleSeries[] = [
  {
    slug: 'blog-engineering',
    title: '博客工程实践',
    description: '从后台验收到数据约束，再到并发编辑和搜索同步，沿着真实实现阅读。',
    articleSlugs: ['fullstack-smoke-pitfalls', 'ent-m2m-edge-case-study', 'mysql-varchar-191-case-study', 'article-cas-revisions-case-study', 'article-search-sync-case-study'],
  },
  {
    slug: 'ai-delivery',
    title: 'AI 与交付实践',
    description: '从仓库规则、交付边界到 Agent 运行时与 AIGC 业务，理解项目中的取舍。',
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
