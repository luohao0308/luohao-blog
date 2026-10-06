<script setup lang="ts">
// Header identity area. The public site restores the session client-side
// (ensureSession in the default layout), so SSR always renders the
// logged-out state and swaps to the avatar after hydration — the flash is an
// accepted trade-off (plan §5). The avatar is an img when uploaded, an
// initials disc otherwise.
const { user, logout } = useAuth()

const menuOpen = ref(false)
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
  <NuxtLink
    v-if="!user"
    to="/login"
    class="rounded-full border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 transition-colors hover:border-blue-500 hover:text-blue-600 dark:border-slate-600 dark:text-slate-200 dark:hover:border-blue-400 dark:hover:text-blue-300"
  >
    登录
  </NuxtLink>

  <div v-else ref="rootEl" class="relative">
    <button
      type="button"
      class="flex items-center justify-center rounded-full transition-opacity hover:opacity-80"
      :aria-expanded="menuOpen"
      aria-haspopup="menu"
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
        class="flex h-8 w-8 items-center justify-center rounded-full bg-[#3c5d85] text-sm font-semibold text-white dark:bg-blue-600"
      >{{ initial }}</span>
    </button>

    <div
      v-if="menuOpen"
      role="menu"
      class="absolute right-0 top-full z-50 mt-2 w-56 overflow-hidden rounded-xl border border-slate-200 bg-white shadow-lg dark:border-slate-700 dark:bg-slate-900"
    >
      <div class="border-b border-slate-100 px-4 py-3 dark:border-slate-800">
        <p class="truncate text-sm font-medium">{{ user.display_name }}</p>
        <p class="truncate text-xs text-slate-500 dark:text-slate-400">{{ user.email }}</p>
      </div>
      <NuxtLink
        to="/settings"
        role="menuitem"
        class="block px-4 py-2.5 text-sm text-slate-700 transition-colors hover:bg-slate-50 hover:text-slate-900 dark:text-slate-200 dark:hover:bg-slate-800 dark:hover:text-white"
        @click="menuOpen = false"
      >
        个人设置
      </NuxtLink>
      <NuxtLink
        v-if="isAdmin"
        to="/admin"
        role="menuitem"
        class="block px-4 py-2.5 text-sm text-slate-700 transition-colors hover:bg-slate-50 hover:text-slate-900 dark:text-slate-200 dark:hover:bg-slate-800 dark:hover:text-white"
        @click="menuOpen = false"
      >
        进入后台
      </NuxtLink>
      <button
        type="button"
        role="menuitem"
        class="block w-full px-4 py-2.5 text-left text-sm text-slate-700 transition-colors hover:bg-slate-50 hover:text-slate-900 dark:text-slate-200 dark:hover:bg-slate-800 dark:hover:text-white"
        @click="signOut"
      >
        退出登录
      </button>
    </div>
  </div>
</template>
