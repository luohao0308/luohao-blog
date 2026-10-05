// Admin session state. The access token is a 15-minute JWT kept in
// module-scope memory only: never in localStorage (XSS) and never in the SSR
// payload. A page reload drops it, so ensureSession() recovers the session
// through the httpOnly refresh cookie (POST /api/v1/auth/refresh — the BFF
// rewrites the backend's Path=/v1/auth cookie to /api/v1/auth so the browser
// carries it). authFetch() attaches the bearer token and transparently
// refreshes once on 401, covering mid-writing token expiry.
//
// Every function here runs client-side only: the admin pages are guarded by
// the admin-auth middleware (no-op on the server) and never SSR-fetch data.

export interface AuthUser {
  id: string
  email: string
  display_name: string
  role: number
  // Site-relative avatar path (/v1/assets/avatars/<name>) since the reader
  // accounts slice; empty until the first upload. Rendered through assetUrl().
  avatar_url?: string
}

export interface LoginReply {
  access_token: string
  token_type: string
  expires_in: number
  user: AuthUser
}

export interface RegisterPayload {
  email: string
  password: string
  display_name: string
}

// Module-scope client state, shared across components like a store singleton.
// Never written on the server, so cross-request leakage is not a concern.
let accessToken = ''
let restorePromise: Promise<void> | null = null
let refreshPromise: Promise<void> | null = null

export function useAuth() {
  const user = useState<AuthUser | null>('auth:user', () => null)

  function applyReply(reply: LoginReply) {
    accessToken = reply.access_token
    user.value = reply.user
  }

  // External updates (profile rename, avatar upload) patch the shared
  // account state in place so the header menu reflects them immediately.
  function setUser(next: AuthUser) {
    user.value = next
  }

  function clearSession() {
    accessToken = ''
    user.value = null
  }

  // ofetch surfaces the HTTP status on err.response.status; the Kratos error
  // body also carries { code, reason }.
  function isUnauthorized(err: unknown): boolean {
    const e = err as { response?: { status?: number }, status?: number, data?: { code?: number } } | null
    return e?.response?.status === 401 || e?.status === 401 || e?.data?.code === 401
  }

  function errReason(err: unknown): string {
    return (err as { data?: { reason?: string } } | null)?.data?.reason ?? ''
  }

  // Body-less POSTs must still declare JSON: the Kratos decoder rejects
  // requests without a registered Content-Type.
  const jsonHeaders = { 'Content-Type': 'application/json' }

  async function login(email: string, password: string): Promise<void> {
    const reply = await $fetch<LoginReply>('/api/v1/auth/login', {
      method: 'POST',
      headers: jsonHeaders,
      body: { email, password },
    })
    applyReply(reply)
  }

  // Register creates a READER account; the backend signs the caller in with
  // the same reply shape as login, so the session state updates identically.
  async function register(payload: RegisterPayload): Promise<void> {
    const reply = await $fetch<LoginReply>('/api/v1/auth/register', {
      method: 'POST',
      headers: jsonHeaders,
      body: payload,
    })
    applyReply(reply)
  }

  async function refresh(): Promise<void> {
    const reply = await $fetch<LoginReply>('/api/v1/auth/refresh', { method: 'POST', headers: jsonHeaders })
    applyReply(reply)
  }

  // Single-flight refresh: the backend consumes the refresh token atomically
  // (GETDEL rotation), so two concurrent 401 retries each firing their own
  // refresh would race — the loser gets a guaranteed 401 and would wipe the
  // session the winner just restored (the "fake logout" from the 2026-10
  // review). All refresh callers share one in-flight promise instead.
  function refreshOnce(): Promise<void> {
    if (!refreshPromise) {
      refreshPromise = refresh().finally(() => {
        refreshPromise = null
      })
    }
    return refreshPromise
  }

  // Recover a session after a page reload. Singleton so concurrent callers
  // (middleware + page) share one refresh attempt and do not race the
  // single-use rotation; failure settles into the logged-out state.
  function ensureSession(): Promise<void> {
    if (import.meta.server)
      return Promise.resolve()
    // Already holding an access token: nothing to restore. This also keeps
    // layout-level ensureSession() calls from burning a rotation after login.
    if (accessToken)
      return Promise.resolve()
    if (!restorePromise) {
      restorePromise = refreshOnce()
        .catch(() => {
          clearSession()
        })
        .finally(() => {
          // Settled restores must not be cached for the SPA lifetime: a
          // transient backend blip otherwise blocks every later navigation.
          restorePromise = null
        })
    }
    return restorePromise
  }

  async function authFetch<T>(path: string, opts: Record<string, unknown> = {}): Promise<T> {
    const run = () => $fetch<T>(path, {
      ...opts,
      headers: {
        ...(opts.headers as Record<string, string> | undefined),
        ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
      },
    })
    try {
      return await run()
    }
    catch (err) {
      if (!isUnauthorized(err))
        throw err
      try {
        await refreshOnce()
      }
      catch {
        clearSession()
        throw err
      }
      return await run()
    }
  }

  async function logout(): Promise<void> {
    try {
      // An in-flight refresh is rotating the session server-side: revoking
      // the stale cookie now would be a no-op, and the rotation's reply
      // would re-apply the token plus its fresh Set-Cookie right after the
      // logout's clearing cookie — resurrecting the session on the next
      // page load (the "fake login" mirror of the fake-logout race). Let
      // the rotation land first, then revoke the live session.
      if (refreshPromise) {
        await refreshPromise.catch(() => {})
      }
      await $fetch('/api/v1/auth/logout', { method: 'POST', headers: jsonHeaders })
    }
    finally {
      clearSession()
    }
  }

  return { user, errReason, login, register, logout, refresh, ensureSession, authFetch, setUser }
}
