// Only a successfully loaded public article supplies this context. The global
// chat reuses it without issuing a second article request.
export interface ChatArticle {
  slug: string
  title: string
}
export type ChatRequestStatus = 'idle' | 'working' | 'done' | 'failure'

export function useChatArticle() {
  return useState<ChatArticle | null>('chat-article', () => null)
}
