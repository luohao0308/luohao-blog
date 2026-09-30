// Typed access to the comment API. Public reads/listings go through the BFF
// proxy like the article API; moderation calls carry the admin bearer token
// via useAuth's authFetch. The Kratos HTTP codec emits snake_case fields and
// numeric enums, matching backend/api/blog/v1/comment.proto.

export type CommentStatus = 0 | 1 | 2

export const COMMENT_STATUS = {
  UNSPECIFIED: 0,
  PENDING: 1,
  APPROVED: 2,
} as const

export const COMMENT_STATUS_LABEL: Record<number, string> = {
  0: '未知',
  1: '待审核',
  2: '已通过',
}

export interface Comment {
  id: string
  article_slug: string
  display_name: string
  content: string
  status: CommentStatus
  created_at: import('./useArticles').ArticleTimestamp
  updated_at: import('./useArticles').ArticleTimestamp
}

export interface CommentSet {
  comments: Comment[]
  next_page_token?: string
}

// Limits mirror the backend biz rules; enforcing them here only saves a
// round trip, the server is the authority.
export const COMMENT_NAME_MAX = 32
export const COMMENT_CONTENT_MAX = 1000

export function useArticleComments(slug: string, opts?: { pageSize?: number }) {
  return useFetch<CommentSet>(`/api/v1/articles/${encodeURIComponent(slug)}/comments`, {
    query: { page_size: opts?.pageSize ?? 20 },
  })
}
