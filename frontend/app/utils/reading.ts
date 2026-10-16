import { assetUrl } from './assets'

// Only map stored backend asset paths. Other attributes and authored links
// keep the backend's sanitized HTML unchanged, including external image URLs.
export function readingAssetHtml(html: string): string {
  return html.replace(/<img\b[^>]*>/gi, image => image.replace(
    /(\s+src\s*=\s*)(["'])(\/v1\/assets\/[^"']*)\2/i,
    (_match, prefix: string, quote: string, path: string) => `${prefix}${quote}${assetUrl(path)}${quote}`,
  ))
}
