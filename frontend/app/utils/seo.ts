export const DEFAULT_SEO_DESCRIPTION = 'luohao 的个人技术博客，记录 Go、全栈开发与 AI Agent 的工程实践、取舍和复盘。'
export const DEFAULT_SEO_IMAGE = '/og-default.jpg'

export function siteOrigin(value: string): string {
  const url = new URL(value)
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password)
    throw new Error('invalid public site origin')
  return url.origin
}

export function siteURL(origin: string, path: string): string {
  if (path.startsWith('//') || path.includes('\\') || /[\r\n]/.test(path)) throw new Error('invalid site path')
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  const trustedOrigin = siteOrigin(origin)
  const url = new URL(normalizedPath, `${trustedOrigin}/`)
  if (url.origin !== trustedOrigin) throw new Error('site path escaped origin')
  return url.toString()
}

export function publicAssetPath(path?: string | null): string {
  if (!path) return DEFAULT_SEO_IMAGE
  if (/^\/v1\/assets\/(?:media|avatars)\/[0-9a-f]{32}\.(?:jpg|png|webp)$/.test(path)) return `/api${path}`
  return path === DEFAULT_SEO_IMAGE ? path : DEFAULT_SEO_IMAGE
}

export function xmlEscape(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll('\'', '&apos;')
}

export function sitemapLastmod(value?: string | { seconds?: string | number } | null): string | undefined {
  if (!value) return undefined
  const date = typeof value === 'string'
    ? new Date(value)
    : new Date(Number(value.seconds ?? 0) * 1000)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}
