// Theme state for the dark mode toggle: light / dark / system. The choice is
// persisted in localStorage and reflected as a `dark` class on <html>. An
// inline head script (app.vue) applies the class pre-hydration so there is
// no first-paint flash.

export type ThemePreference = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'theme'

export function useTheme() {
  const preference = useState<ThemePreference>('theme-preference', () => 'light')

  const apply = (pref: ThemePreference) => {
    if (!import.meta.client) return
    const systemDark = pref === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches
    const dark = pref === 'dark' || (pref === 'system' && systemDark)
    document.documentElement.classList.toggle('dark', dark)
    document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
  }

  const set = (pref: ThemePreference) => {
    preference.value = pref
    if (import.meta.client) {
      try { localStorage.setItem(STORAGE_KEY, pref) } catch { /* Keep the in-memory choice when storage is unavailable. */ }
    }
    apply(pref)
  }

  const toggle = () => {
    const systemDark = import.meta.client && window.matchMedia('(prefers-color-scheme: dark)').matches
    const currentDark = import.meta.client && document.documentElement.classList.contains('dark')
    const dark = currentDark ?? (preference.value === 'dark' || (preference.value === 'system' && systemDark))
    set(dark ? 'light' : 'dark')
  }

  const init = () => {
    if (!import.meta.client) return
    let stored: string | null = null
    try { stored = localStorage.getItem(STORAGE_KEY) } catch { /* Use the same light fallback as the head script. */ }
    preference.value = stored === 'dark' || stored === 'system' ? stored : 'light'
    apply(preference.value)
  }

  return { preference, set, toggle, init }
}
