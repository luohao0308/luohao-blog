// Route guard for /admin/**. Session state is memory + httpOnly cookie, so
// the check must run after hydration: ensureSession() tries one refresh, then
// unauthenticated visitors are sent to the login page with a return path.
export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server)
    return
  const { user, ensureSession } = useAuth()
  await ensureSession()
  if (!user.value) {
    return navigateTo({ path: '/admin/login', query: { redirect: to.fullPath } })
  }
})
