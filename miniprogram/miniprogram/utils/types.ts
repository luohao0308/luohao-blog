// 与 Kratos HTTP codec 的 wire 格式对齐：snake_case 字段、枚举数字、
// RFC3339 时间字符串。类型镜像自 frontend/app/composables/useArticles.ts。

export interface CategoryBrief {
  slug: string
  name: string
}

export interface Article {
  id: string
  slug: string
  title: string
  summary: string
  content_md: string
  content_html: string
  tags: string[]
  category?: CategoryBrief | null
  // ArticleStatus 枚举数字：2 = PUBLISHED。
  status: number
  published_at?: string
  created_at?: string
  updated_at?: string
  view_count?: number
  like_count?: number
}

export interface ArticleSet {
  articles: Article[]
  next_page_token?: string
}

export const ARTICLE_STATUS_PUBLISHED = 2
