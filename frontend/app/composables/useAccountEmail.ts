const TOKEN_PATTERN = /^[A-Za-z0-9_-]{20,256}$/
export const PASSWORD_MAX_BYTES = 1024

export function passwordByteLength(value: string): number {
  return new TextEncoder().encode(value).length
}

export function takeHashToken(): string {
  if (!import.meta.client) return ''
  const hash = window.location.hash.slice(1)
  const token = new URLSearchParams(hash).get('token') ?? ''
  // Tokens never survive in query, history, localStorage, or SSR payload.
  window.history.replaceState(window.history.state, '', window.location.pathname)
  return TOKEN_PATTERN.test(token) ? token : ''
}

export function accountEmailReason(err: unknown): string {
  return (err as { data?: { reason?: string } } | null)?.data?.reason ?? ''
}

export function accountEmailStatus(err: unknown): number | undefined {
  const value = err as { response?: { status?: number }, status?: number } | null
  return value?.response?.status ?? value?.status
}

export function useAccountEmail() {
  const { authFetch } = useAuth()
  const jsonHeaders = { 'Content-Type': 'application/json' }

  const requestVerification = () => authFetch('/api/v1/auth/email-verification/request', {
    method: 'POST', headers: jsonHeaders, body: {},
  })
  const confirmVerification = (token: string) => $fetch('/api/v1/auth/email-verification/confirm', {
    method: 'POST', headers: jsonHeaders, body: { token },
  })
  const requestPasswordReset = (email: string) => $fetch('/api/v1/auth/password-reset/request', {
    method: 'POST', headers: jsonHeaders, body: { email },
  })
  const confirmPasswordReset = (token: string, newPassword: string) => $fetch('/api/v1/auth/password-reset/confirm', {
    method: 'POST', headers: jsonHeaders, body: { token, new_password: newPassword },
  })

  return { requestVerification, confirmVerification, requestPasswordReset, confirmPasswordReset }
}
