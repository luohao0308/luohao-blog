// Backend asset paths (avatars) are stored site-relative against the API
// origin (/v1/assets/...). Browsers never hit the backend directly — every
// request goes through the Nuxt BFF, which strips the /api prefix — so
// rendering prefixes /api. Empty input yields an empty string so callers can
// fall back to an initials avatar.
export function assetUrl(path?: string | null): string {
  if (!path)
    return ''
  return path.startsWith('/') ? `/api${path}` : `/api/${path}`
}
