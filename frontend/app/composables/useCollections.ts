// Local-only bookmarks. The site has no visitor accounts, so collections
// live in this browser's localStorage; the UI labels that explicitly. The
// server is never involved.

export const COLLECTIONS_KEY = 'blog:collections'

export interface CollectionEntry {
  slug: string
  title: string
  saved_at: string
}

export function readCollections(): CollectionEntry[] {
  if (import.meta.server)
    return []
  try {
    const list = JSON.parse(localStorage.getItem(COLLECTIONS_KEY) ?? '[]')
    return Array.isArray(list) ? list : []
  }
  catch {
    return []
  }
}

export function writeCollections(list: CollectionEntry[]) {
  if (import.meta.server)
    return
  localStorage.setItem(COLLECTIONS_KEY, JSON.stringify(list))
}

export function isCollected(slug: string): boolean {
  return readCollections().some(entry => entry.slug === slug)
}

export function toggleCollection(slug: string, title: string): boolean {
  const list = readCollections()
  const index = list.findIndex(entry => entry.slug === slug)
  if (index >= 0) {
    list.splice(index, 1)
    writeCollections(list)
    return false
  }
  list.unshift({ slug, title, saved_at: new Date().toISOString() })
  writeCollections(list)
  return true
}
