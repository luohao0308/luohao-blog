<script setup lang="ts">
// Header identity area. The public site restores the session client-side
// (ensureSession in the default layout), so SSR always renders the
// logged-out state and swaps to the avatar after hydration — the flash is an
// accepted trade-off (plan §5). The avatar is an img when uploaded, an
// initials disc otherwise.
const { user, logout } = useAuth()

const menuOpen = ref(false)
const route = useRoute()
const triggerEl = ref<HTMLButtonElement | null>(null)
const navigationId = useId()
watch(() => route.fullPath, () => { menuOpen.value = false })

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape' || event.isComposing || event.keyCode === 229 || !menuOpen.value) return
  event.preventDefault()
  menuOpen.value = false
  triggerEl.value?.focus()
}

function onFocusout(event: FocusEvent) {
  if (event.relatedTarget && !rootEl.value?.contains(event.relatedTarget as Node)) menuOpen.value = false
}

const rootEl = ref<HTMLElement | null>(null)

function onDocumentClick(event: MouseEvent) {
  if (rootEl.value && !rootEl.value.contains(event.target as Node))
    menuOpen.value = false
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', onDocumentClick)
})

const initial = computed(() => (user.value?.display_name || user.value?.email || '?').trim().charAt(0).toUpperCase())
// Role mirrors the api enum: 1 = ADMIN (the header only surfaces the admin
// entry for the site owner), 2 = READER.
const isAdmin = computed(() => user.value?.role === 1)

async function signOut() {
  menuOpen.value = false
  await logout()
}
</script>

<template>
  <div v-if="!user" class="flex items-center gap-2">
    <NuxtLink
      to="/login"
      class="inline-flex min-h-11 items-center rounded-full border border-[var(--border-strong)] px-3 py-1.5 text-sm font-medium text-[var(--text-primary)] transition-colors hover:border-[var(--accent)] hover:text-[var(--accent)]"
    >
      登录
    </NuxtLink>
  </div>

  <div v-else ref="rootEl" class="relative" @keydown="onKeydown" @focusout="onFocusout">
    <button
      ref="triggerEl"
      type="button"
      class="flex h-11 w-11 items-center justify-center rounded-full transition-opacity hover:opacity-80"
      :aria-expanded="menuOpen"
      :aria-controls="navigationId"
      :aria-label="`${user.display_name} 的账号菜单`"
      @click="menuOpen = !menuOpen"
    >
      <img
        v-if="user.avatar_url"
        :src="assetUrl(user.avatar_url)"
        :alt="user.display_name"
        class="h-8 w-8 rounded-full object-cover"
      >
      <span
        v-else
        class="flex h-8 w-8 items-center justify-center rounded-full bg-[var(--accent)] text-sm font-semibold text-[var(--on-accent)]"
      >{{ initial }}</span>
    </button>

    <nav
      v-if="menuOpen"
      :id="navigationId"
      aria-label="账号导航"
      class="absolute right-0 top-full z-50 mt-2 w-56 rounded-[var(--radius-panel)] border border-[var(--border-default)] bg-[var(--surface-overlay)] shadow-lg"
    >
      <div class="border-b border-[var(--border-default)] px-4 py-3">
        <p class="truncate text-sm font-medium">{{ user.display_name }}</p>
        <p class="truncate text-xs text-[var(--text-muted)]">{{ user.email }}</p>
      </div>
      <NuxtLink
        to="/settings"
        class="account-item px-4 py-2.5 text-sm transition-colors"
        @click="menuOpen = false"
      >
        个人设置（头像 / 昵称）
      </NuxtLink>
      <NuxtLink
        to="/login?switch=1"
        class="account-item px-4 py-2.5 text-sm transition-colors"
        @click="menuOpen = false"
      >
        切换账号
      </NuxtLink>
      <NuxtLink
        v-if="isAdmin"
        to="/admin"
        class="account-item px-4 py-2.5 text-sm transition-colors"
        @click="menuOpen = false"
      >
        进入后台
      </NuxtLink>
      <button
        type="button"
        class="account-item w-full px-4 py-2.5 text-left text-sm transition-colors"
        @click="signOut"
      >
        退出登录
      </button>
    </nav>
  </div>
</template>
