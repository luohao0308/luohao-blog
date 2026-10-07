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
  // protojson 规则：uint64 在 JSON 里编码为十进制字符串，用时必须 Number() 转换。
  view_count?: number | string
  like_count?: number | string
}

export interface ArticleSet {
  articles: Article[]
  next_page_token?: string
}

export const ARTICLE_STATUS_PUBLISHED = 2

// --- 认证 wire 类型（snake_case，与后端契约一致） ---

export interface Account {
  id: string
  email: string
  displayName: string
  role: number
  avatarUrl?: string
}

export interface LoginReply {
  access_token: string
  token_type: string
  // uint64 同样以字符串下发。
  expires_in: number | string
  user: Account
}

export interface WechatLoginReply {
  // WechatLoginStatus：1 = OK（login 可用），2 = BINDING_REQUIRED（binding_ticket 可用）
  status: number
  login?: LoginReply
  binding_ticket?: string
}

export const WECHAT_STATUS_OK = 1
export const WECHAT_STATUS_BINDING_REQUIRED = 2

// --- 评论 wire 类型 ---

export interface Comment {
  id: string
  article_slug: string
  display_name: string
  content: string
  // CommentStatus：1 = PENDING（待审），2 = APPROVED（已过审）
  status: number
  created_at?: string
  user_id?: string
  avatar_url?: string
}

export interface CommentSet {
  comments: Comment[]
  next_page_token?: string
}

export const COMMENT_STATUS_PENDING = 1
export const COMMENT_STATUS_APPROVED = 2

// --- 搜索 / AI 问答 wire 类型 ---

export interface CitedArticle {
  slug: string
  title: string
  summary: string
}

export interface ChatReply {
  answer: string
  citations: CitedArticle[]
}
