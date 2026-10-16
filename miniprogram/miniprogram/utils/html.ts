import { assetUrl } from './config'

// rich-text 只认白名单标签与内联 style，不解析 class。后端 content_html
// 面向 Web 端输出，这里给高频块级元素补内联样式；文字排版颜色等可继承
// 属性由页面容器兜底。
export function decorateArticleHtml(html: string): string {
  return html
    .replace(/<img\b(?:[^"'<>]|"[^"]*"|'[^']*')*>/gi, (tag) => tag
      .replace(/(\s+src\s*=\s*)(["'])(.*?)\2/gi, (attribute, prefix: string, quote: string, src: string) => {
        if (!src.startsWith('/v1/assets/') && !src.startsWith('/api/v1/assets/')) return attribute
        return `${prefix}${quote}${assetUrl(src)}${quote}`
      })
      .replace(/<img\b/i, '<img style="max-width:100%;border-radius:8px;vertical-align:middle;" '))
    .replace(/<pre\b/gi, '<pre style="background:#f6f8fa;padding:12px;border-radius:8px;overflow-x:auto;white-space:pre;font-size:13px;line-height:1.6;" ')
    .replace(/<code\b/gi, '<code style="font-family:Menlo,Consolas,monospace;" ')
    .replace(/<blockquote\b/gi, '<blockquote style="border-left:4px solid #d1d5db;margin:8px 0;padding:2px 12px;color:#6b7280;" ')
    .replace(/<table\b/gi, '<table style="border-collapse:collapse;width:100%;" ')
    .replace(/<(td|th)\b/gi, '<$1 style="border:1px solid #e5e7eb;padding:6px 8px;" ')
    .replace(/<a\b/gi, '<a style="color:#2563eb;" ')
}

export interface ArticleContentBlock {
  key: number
  type: 'html' | 'image'
  html: string
  src: string
  imageUrls: string[]
}

function imageSource(tag: string): string {
  const attributes = tag.replace(/^<img\b/i, '').matchAll(/([^\s=/>]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>`]+)))?/g)
  for (const attribute of attributes) {
    if (attribute[1].toLowerCase() !== 'src') continue
    const src = (attribute[2] ?? attribute[3] ?? attribute[4] ?? '').replace(/&amp;/gi, '&')
    return /^https?:\/\/[^\s]+$/i.test(src) ? src : ''
  }
  return ''
}

// rich-text suppresses image events. Only standalone image nodes become native
// images. Keep mixed text, lists/tables/quotes and unknown containers intact.
export function articleContentBlocks(html: string): ArticleContentBlock[] {
  if (!html) return []
  const blocks: ArticleContentBlock[] = []
  const parents: string[] = []
  const voidTags = new Set(['area', 'base', 'br', 'col', 'embed', 'hr', 'img', 'input', 'link', 'meta', 'param', 'source', 'track', 'wbr'])
  const transparentTags = new Set(['p', 'a', 'span', 'strong', 'em', 'b', 'i', 's', 'del', 'u', 'sup', 'sub'])
  let node: string[] = []
  const emit = () => {
    const fragment = node.join('')
    const sources = node.filter(token => /^<img\b/i.test(token)).map(imageSource).filter(Boolean)
    const standalone = sources.length > 0 && node.every(token => {
      const name = /^<\/?([a-z][a-z0-9-]*)\b/i.exec(token)?.[1].toLowerCase()
      return name === 'img' ? Boolean(imageSource(token)) : name ? transparentTags.has(name) : token.trim() === ''
    })
    if (standalone) {
      for (const src of sources) blocks.push({ key: blocks.length, type: 'image', html: '', src, imageUrls: [src] })
    } else {
      const last = blocks[blocks.length - 1]
      if (last?.type === 'html') {
        last.html += fragment
        last.imageUrls = Array.from(new Set([...last.imageUrls, ...sources]))
      } else if (fragment.trim()) {
        blocks.push({ key: blocks.length, type: 'html', html: fragment, src: '', imageUrls: Array.from(new Set(sources)) })
      }
    }
    node = []
  }
  const tokens = html.match(/<!--[^]*?-->|<\/?[a-z][a-z0-9-]*\b(?:[^"'<>]|"[^"]*"|'[^']*')*>|[^<]+|</gi) ?? []
  for (const token of tokens) {
    node.push(token)
    const tag = /^<(\/?)([a-z][a-z0-9-]*)\b/i.exec(token)
    const name = tag?.[2].toLowerCase()
    if (name && !voidTags.has(name)) {
      if (tag?.[1] === '/') {
        const index = parents.lastIndexOf(name)
        if (index >= 0) parents.splice(index)
      } else if (!/\/\s*>$/.test(token)) {
        parents.push(name)
      }
    }
    if (!parents.length) emit()
  }
  if (node.length) emit()
  // In particular, preserve a text-only/complex article byte-for-byte.
  if (!blocks.some(block => block.type === 'image')) {
    return [{ key: 0, type: 'html', html, src: '', imageUrls: blocks.flatMap(block => block.imageUrls) }]
  }
  return blocks
}
