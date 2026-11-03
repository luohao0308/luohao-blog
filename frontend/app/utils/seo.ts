export const DEFAULT_SEO_DESCRIPTION = '罗浩的个人博客，包含 Python、Go、全栈开发和 AI 应用的文章与项目。'
export const DEFAULT_SEO_IMAGE = '/og-default.jpg'

export function siteOrigin(value: string): string {
  try {
    const url = new URL(value)
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password
      || url.pathname !== '/' || url.search || url.hash) return ''
    return url.origin
  } catch { return '' }
}

export function siteURL(origin: string, path: string): string {
  const trustedOrigin = siteOrigin(origin)
  if (!trustedOrigin || path.startsWith('//') || path.includes('\\') || /[\r\n]/.test(path)) return ''
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  try {
    const url = new URL(normalizedPath, `${trustedOrigin}/`)
    return url.origin === trustedOrigin ? url.toString() : ''
  } catch { return '' }
}

export function publicAssetPath(path?: string | null): string {
  if (!path) return DEFAULT_SEO_IMAGE
  if (/^\/images\/editorial\/covers\/(?:workflow|guard|harness|aigc|fullstack|ent|mysql|workbench)\.webp$/.test(path)) return path
  if (/^\/images\/editorial\/covers\/(?:search-sync|cas-revisions)\.png$/.test(path)) return path
  if (/^\/images\/projects\/generated\/(?:dev-workflow|forge-harness|aigc|luohao-blog|job-application-workbench)-figma\.png$/.test(path)) return path
  if (/^\/images\/projects\/(?:dev-workflow|forge-harness|aigc|luohao-blog|job-application-workbench)\.png$/.test(path)) return path
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
