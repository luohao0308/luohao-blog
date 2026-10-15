import type { Article, ArticleTimestamp } from '~/composables/useArticles'

export const COLLECTIONS_KEY = 'blog:collections'
export interface CollectionEntry { slug: string, title: string, saved_at: string }
export interface CloudCollection { article: Article, saved_at: ArticleTimestamp }
interface CollectionSet { collections: CloudCollection[], next_page_token?: string }

export function readCollections(): CollectionEntry[] {
  if (import.meta.server) return []
  try {
    const list = JSON.parse(localStorage.getItem(COLLECTIONS_KEY) ?? '[]')
    return Array.isArray(list) ? list.filter(item => item && typeof item.slug === 'string') : []
  }
  catch { return [] }
}
export function writeCollections(list: CollectionEntry[]) {
  if (import.meta.client) localStorage.setItem(COLLECTIONS_KEY, JSON.stringify(list))
}
export function isCollected(slug: string): boolean { return readCollections().some(entry => entry.slug === slug) }
export function toggleCollection(slug: string, title: string): boolean {
  const list = readCollections()
  const index = list.findIndex(entry => entry.slug === slug)
  if (index >= 0) list.splice(index, 1)
  else list.unshift({ slug, title, saved_at: new Date().toISOString() })
  writeCollections(list)
  return index < 0
}

export function useCollections() {
  const { user, ensureSession, authFetch } = useAuth()
  const guest = useState<CollectionEntry[]>('collections:guest', () => [])
  const cloud = useState<CloudCollection[]>('collections:cloud', () => [])
  const owner = useState<string>('collections:owner', () => '')
  const generation = useState<number>('collections:generation', () => 0)
  const mutationEpoch = useState<number>('collections:mutation-epoch', () => 0)
  const loadVersion = useState<number>('collections:load-version', () => 0)
  const loadedOwner = useState<string>('collections:loaded-owner', () => '')
  const ready = useState<boolean>('collections:ready', () => false)
  const loading = useState<boolean>('collections:loading', () => false)
  const error = useState<string>('collections:error', () => '')
  const busy = ref(new Set<string>())

  function refreshGuest() { guest.value = readCollections() }
  function syncOwner() {
    const id = user.value?.id ?? ''
    if (owner.value === id) return
    owner.value = id
    generation.value++
    mutationEpoch.value++
    loadVersion.value++
    cloud.value = []
    loadedOwner.value = ''
    ready.value = false
    loading.value = false
    busy.value = new Set()
    error.value = ''
  }

  async function loadCloud(force = false) {
    syncOwner()
    const id = owner.value
    if (!id) return true
    if (!force && loadedOwner.value === id) return true
    const requestGeneration = generation.value
    const requestEpoch = mutationEpoch.value
    const requestVersion = ++loadVersion.value
    loading.value = true
    error.value = ''
    try {
      const rows: CloudCollection[] = []
      const seen = new Set<string>()
      let token = ''
      do {
        const page = await authFetch<CollectionSet>('/api/v1/collections/list', {
          expectedUserId: id,
          query: { page_size: 100, page_token: token || undefined },
          cache: 'no-store',
        })
        if (owner.value !== id || generation.value !== requestGeneration || loadVersion.value !== requestVersion) return false
        rows.push(...(page.collections ?? []))
        token = page.next_page_token ?? ''
        if (token && seen.has(token)) throw new Error('收藏分页异常')
        if (token) seen.add(token)
      } while (token)
      if (owner.value !== id || generation.value !== requestGeneration || loadVersion.value !== requestVersion || mutationEpoch.value !== requestEpoch) return false
      cloud.value = rows
      loadedOwner.value = id
      ready.value = true
      return true
    }
    catch (err) {
      if ((err as Error)?.name !== 'AuthOwnerChangedError' && owner.value === id && generation.value === requestGeneration)
        error.value = '云端收藏加载失败，请重试。'
      return false
    }
    finally { if (owner.value === id && generation.value === requestGeneration && loadVersion.value === requestVersion) loading.value = false }
  }

  async function initialize(force = false) {
    refreshGuest()
    await ensureSession()
    syncOwner()
    const id = owner.value
    const loaded = id ? await loadCloud(force || loadedOwner.value !== id) : true
    if (owner.value === id) {
      const supersededByMutation = !loaded && loadedOwner.value === id && !error.value
      ready.value = supersededByMutation || (loaded && (!id || loadedOwner.value === id))
    }
    return ready.value
  }

  function collected(slug: string): boolean {
    return owner.value ? cloud.value.some(row => row.article.slug === slug) : guest.value.some(row => row.slug === slug)
  }

  async function toggle(slug: string, title: string): Promise<boolean> {
    await initialize()
    syncOwner()
    const id = owner.value
    if (!id) {
      const value = toggleCollection(slug, title)
      refreshGuest()
      return value
    }
    if (busy.value.has(slug)) return collected(slug)
    const requestBusy = busy.value
    requestBusy.add(slug)
    const requestGeneration = generation.value
    const currently = collected(slug)
    try {
      if (currently) {
        await authFetch(`/api/v1/collections/${encodeURIComponent(slug)}`, { method: 'DELETE', expectedUserId: id })
        if (owner.value === id && generation.value === requestGeneration) {
          mutationEpoch.value++
          cloud.value = cloud.value.filter(row => row.article.slug !== slug)
        }
      }
      else {
        const saved = await authFetch<CloudCollection>(`/api/v1/collections/${encodeURIComponent(slug)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: {}, expectedUserId: id })
        if (owner.value === id && generation.value === requestGeneration) {
          mutationEpoch.value++
          cloud.value = [saved, ...cloud.value.filter(row => row.article.slug !== slug)]
        }
      }
      return !currently
    }
    finally {
      if (busy.value === requestBusy && owner.value === id && generation.value === requestGeneration)
        requestBusy.delete(slug)
    }
  }

  async function importGuest() {
    await initialize()
    syncOwner()
    const id = owner.value
    if (!id) return { imported: 0, failed: guest.value.length }
    const requestGeneration = generation.value
    let imported = 0
    let failed = 0
    for (const entry of [...guest.value]) {
      if (owner.value !== id || generation.value !== requestGeneration) break
      try {
        const saved = await authFetch<CloudCollection>(`/api/v1/collections/${encodeURIComponent(entry.slug)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: {}, expectedUserId: id })
        if (owner.value !== id || generation.value !== requestGeneration) break
        const nextGuest = guest.value.filter(item => item.slug !== entry.slug)
        writeCollections(nextGuest)
        mutationEpoch.value++
        cloud.value = [saved, ...cloud.value.filter(row => row.article.slug !== entry.slug)]
        guest.value = nextGuest
        imported++
      }
      catch (err) {
        if ((err as Error)?.name === 'AuthOwnerChangedError') break
        failed++
      }
    }
    return { imported, failed }
  }

  watch(() => user.value?.id ?? '', () => {
    syncOwner()
    if (!owner.value) refreshGuest()
  }, { immediate: true, flush: 'sync' })

  return { guest, cloud, owner, ready, loading, error, busy, initialize, loadCloud, collected, toggle, importGuest, refreshGuest }
}
