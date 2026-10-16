import { chatAsk } from '../../utils/api'
import { toMessage } from '../../utils/request'

interface Citation {
  slug: string
  title: string
}

interface ChatMessage {
  role: 'user' | 'assistant'
  text: string
  citations: Citation[]
  error?: boolean
  question?: string
}

// 开场引导：让访客知道可以问什么。
const GREETING: ChatMessage = {
  role: 'assistant',
  text: '我是博客读者助手，根据已发布文章回答。每次问题独立回答，例如「写过哪些关于 Ent 的内容？」',
  citations: [],
}

Page({
  data: {
    messages: [GREETING],
    input: '',
    busy: false,
  },

  onInput(event: WechatMiniprogram.Input) {
    this.setData({ input: event.detail.value })
  },

  async send() {
    if (this.data.busy) return
    const question = this.data.input.trim()
    if (!question) return
    if (Array.from(question).length > 500) return
    const messages = this.data.messages.concat([{ role: 'user' as const, text: question, citations: [] }])
    messages.push({ role: 'assistant', text: '', citations: [] })
    this.setData({ messages, input: '' })
    await this.requestAnswer(question, messages.length - 1)
  },

  async retry(event: WechatMiniprogram.TouchEvent) {
    const index = Number(event.currentTarget.dataset.index)
    const message = this.data.messages[index]
    if (this.data.busy || !message?.error || !message.question) return
    await this.requestAnswer(message.question, index)
  },

  async requestAnswer(question: string, index: number) {
    this.setData({ busy: true })
    try {
      const reply = await chatAsk(question)
      this.setData({
        messages: (this.data.messages as ChatMessage[]).map((message, i) => i === index ? {
          role: 'assistant',
          question,
          error: reply.outcome === 3 || reply.outcome === 4 || reply.outcome === 'CHAT_OUTCOME_RETRIEVAL_UNAVAILABLE' || reply.outcome === 'CHAT_OUTCOME_GENERATION_UNAVAILABLE',
          text: reply.answer || '（没有生成内容）',
          citations: (reply.citations ?? []).map((c) => ({ slug: c.slug, title: c.title })),
        } : message),
        busy: false,
      })
      this.scrollBottom()
    } catch (error) {
      this.setData({
        messages: (this.data.messages as ChatMessage[]).map((message, i) => i === index ? {
          role: 'assistant',
          question,
          text: toMessage(error),
          citations: [],
          error: true,
        } : message),
        busy: false,
      })
      this.scrollBottom()
    }
  },

  scrollBottom() {
    wx.pageScrollTo({ scrollTop: 999999, duration: 200 })
  },

  openCitation(event: WechatMiniprogram.TouchEvent) {
    const slug = event.currentTarget.dataset.slug as string
    wx.navigateTo({ url: `/pages/post/post?slug=${encodeURIComponent(slug)}` })
  },
})
