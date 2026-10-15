// Route guard for /admin/**. Session state is memory + httpOnly cookie, so
// the check runs client-side: ensureSession() tries one refresh, then visitors
// without the ADMIN role are sent to the login page with a return path.
export default defineNuxtRouteMiddleware(async (to) => {
  if (import.meta.server)
    return
  const { user, ensureSession } = useAuth()
  await ensureSession()
  if (user.value?.role !== 1) {
    const nuxtApp = useNuxtApp()
    // Initial SSR HTML belongs to the requested management page. A full
    // navigation prevents hydrating that HTML with the login page's tree.
    return navigateTo({ path: '/admin/login', query: { redirect: to.fullPath } }, {
      replace: true,
      external: nuxtApp.isHydrating && nuxtApp.payload.serverRendered,
    })
  }
})
