<script setup lang="ts">
// Personal settings: avatar upload, display name, password rotation. The
// page is client-side only: the login guard restores the session first and
// bounces anonymous visitors to /login?redirect=/settings. Avatars are
// canvas-downscaled to 512px and encoded client-side (webp, falling back to
// the source format), then uploaded as base64 — the format the backend
// contract expects (≤2MiB, magic-byte sniffed jpeg/png/webp).
import type { AuthUser } from '~/composables/useAuth'

useHead({ title: '个人设置' })

const { user, authFetch, setUser, ensureSession } = useAuth()

const ready = ref(false)
onMounted(async () => {
  await ensureSession()
  if (!user.value) {
    navigateTo('/login?redirect=%2Fsettings', { replace: true })
    return
  }
  ready.value = true
})

// --- avatar --------------------------------------------------------------

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const previewUrl = ref('')
const uploading = ref(false)
const avatarNotice = ref<{ kind: 'ok' | 'err', text: string } | null>(null)

const MAX_AVATAR_BYTES = 2 * 1024 * 1024

function onFileChange(event: Event) {
  avatarNotice.value = null
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file)
    return
  if (!file.type.startsWith('image/')) {
    avatarNotice.value = { kind: 'err', text: '请选择图片文件' }
    return
  }
  if (previewUrl.value)
    URL.revokeObjectURL(previewUrl.value)
  selectedFile.value = file
  previewUrl.value = URL.createObjectURL(file)
}

function selectFile() {
  fileInput.value?.click()
}

// Downscale to a 512px square-bounded image and return raw bytes as base64.
// webp keeps photos small and preserves alpha; toBlob falls back to the
// source format when webp is unavailable.
function downscale(file: File, max = 512): Promise<string> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const img = new Image()
    img.onload = () => {
      URL.revokeObjectURL(url)
      const scale = Math.min(1, max / Math.max(img.width, img.height))
      const w = Math.max(1, Math.round(img.width * scale))
      const h = Math.max(1, Math.round(img.height * scale))
      const canvas = document.createElement('canvas')
      canvas.width = w
      canvas.height = h
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        reject(new Error('canvas unavailable'))
        return
      }
      ctx.drawImage(img, 0, 0, w, h)
      const handleBlob = (blob: Blob | null) => {
        if (!blob) {
          reject(new Error('encode failed'))
          return
        }
        if (blob.size > MAX_AVATAR_BYTES) {
          reject(new Error('encoded image still exceeds 2MiB'))
          return
        }
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result).split(',')[1] ?? '')
        reader.onerror = () => reject(new Error('read failed'))
        reader.readAsDataURL(blob)
      }
      // webp keeps photos small and preserves alpha; the second toBlob call
      // is a no-op unless the browser cannot encode webp (its callback then
      // runs with null and the retry encodes in the source format).
      canvas.toBlob(handleBlob, 'image/webp', 0.85)
      canvas.toBlob(
        blob => blob && handleBlob(blob),
        file.type === 'image/png' ? 'image/png' : 'image/jpeg',
        0.85,
      )
    }
    img.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error('not an image'))
    }
    img.src = url
  })
}

function mapAvatarError(err: unknown): string {
  const status = (err as { response?: { status?: number } } | null)?.response?.status
  if (status === 400)
    return '图片不符合要求：仅支持 jpg/png/webp，且不超过 2MB'
  if (status === 401)
    return '登录状态已失效，请重新登录'
  return '上传失败，请稍后再试'
}

async function uploadAvatar() {
  if (!selectedFile.value || uploading.value)
    return
  avatarNotice.value = null
  uploading.value = true
  try {
    const data = await downscale(selectedFile.value)
    const updated = await authFetch<AuthUser>('/api/v1/user/avatar', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: { data },
    })
    if (updated)
      setUser(updated)
    avatarNotice.value = { kind: 'ok', text: '头像已更新' }
    selectedFile.value = null
    if (previewUrl.value) {
      URL.revokeObjectURL(previewUrl.value)
      previewUrl.value = ''
    }
  }
  catch (err) {
    avatarNotice.value = { kind: 'err', text: (err as Error)?.message?.includes('2MiB')
      ? '压缩后仍超过 2MB，请换一张小一点的图片'
      : mapAvatarError(err) }
  }
  finally {
    uploading.value = false
  }
}

// --- display name --------------------------------------------------------

const displayName = ref('')
const nameSaving = ref(false)
const nameNotice = ref<{ kind: 'ok' | 'err', text: string } | null>(null)

watch(() => user.value?.display_name, (name) => {
  displayName.value = name ?? ''
}, { immediate: true })

async function saveDisplayName() {
  if (nameSaving.value)
    return
  nameNotice.value = null
  const name = displayName.value.trim()
  if (!name || name.length > 32) {
    nameNotice.value = { kind: 'err', text: '昵称需为 1-32 个字' }
    return
  }
  nameSaving.value = true
  try {
    const updated = await authFetch<AuthUser>('/api/v1/user/profile', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: { display_name: name },
    })
    if (updated)
      setUser(updated)
    nameNotice.value = { kind: 'ok', text: '昵称已更新' }
  }
  catch (err) {
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    nameNotice.value = { kind: 'err', text: status === 400 ? '昵称需为 1-32 个字' : status === 401 ? '登录状态已失效，请重新登录' : '保存失败，请稍后再试' }
  }
  finally {
    nameSaving.value = false
  }
}

// --- password ------------------------------------------------------------

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordSaving = ref(false)
const passwordNotice = ref<{ kind: 'ok' | 'err', text: string } | null>(null)

async function savePassword() {
  if (passwordSaving.value)
    return
  passwordNotice.value = null
  if (!oldPassword.value || !newPassword.value) {
    passwordNotice.value = { kind: 'err', text: '请填写当前密码和新密码' }
    return
  }
  if (newPassword.value.length < 8) {
    passwordNotice.value = { kind: 'err', text: '新密码至少 8 位' }
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    passwordNotice.value = { kind: 'err', text: '两次输入的新密码不一致' }
    return
  }
  passwordSaving.value = true
  try {
    await authFetch('/api/v1/auth/update-password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: { old_password: oldPassword.value, new_password: newPassword.value },
    })
    passwordNotice.value = { kind: 'ok', text: '密码已修改，下次登录请使用新密码' }
    oldPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  }
  catch (err) {
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    passwordNotice.value = { kind: 'err', text: status === 401 ? '当前密码不正确' : status === 400 ? '新密码至少 8 位' : '修改失败，请稍后再试' }
  }
  finally {
    passwordSaving.value = false
  }
}

const inputClass = 'w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none focus:border-blue-500 dark:border-slate-600 dark:bg-slate-900'
const primaryButtonClass = 'shrink-0 whitespace-nowrap rounded-full bg-blue-600 px-5 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-700 disabled:opacity-50'
const avatarSrc = computed(() => (user.value?.avatar_url ? assetUrl(user.value.avatar_url) : ''))
</script>

<template>
  <div class="mx-auto max-w-2xl space-y-6">
    <template v-if="ready && user">
      <header class="space-y-1">
        <h1 class="text-2xl font-bold tracking-tight">
          个人设置
        </h1>
        <p class="text-sm text-slate-500 dark:text-slate-400">
          头像和昵称会显示在评论区与右上角菜单。
        </p>
      </header>

      <!-- 头像 -->
      <section class="space-y-4 rounded-xl border border-slate-200 p-6 dark:border-slate-800" aria-label="头像">
        <h2 class="font-medium">
          头像
        </h2>
        <div class="flex items-center gap-5">
          <img
            v-if="previewUrl || avatarSrc"
            :src="previewUrl || avatarSrc"
            alt="头像预览"
            class="h-20 w-20 rounded-full object-cover"
          >
          <span
            v-else
            class="flex h-20 w-20 items-center justify-center rounded-full bg-[#3c5d85] text-2xl font-semibold text-white dark:bg-blue-600"
          >{{ (user.display_name || user.email).trim().charAt(0).toUpperCase() }}</span>
          <div class="space-y-2">
            <div class="flex gap-2">
              <button type="button" class="shrink-0 whitespace-nowrap rounded-full border border-slate-300 px-5 py-2 text-sm text-slate-700 transition-colors hover:border-blue-500 hover:text-blue-600 dark:border-slate-600 dark:text-slate-200 dark:hover:border-blue-400 dark:hover:text-blue-300" @click="selectFile">
                选择图片
              </button>
              <button type="button" :disabled="!selectedFile || uploading" :class="primaryButtonClass" @click="uploadAvatar">
                {{ uploading ? '上传中…' : '上传新头像' }}
              </button>
            </div>
            <p class="text-xs text-slate-400 dark:text-slate-500">
              支持 jpg / png / webp，自动压缩到 512px，不超过 2MB
            </p>
          </div>
        </div>
        <input ref="fileInput" type="file" accept="image/*" class="hidden" @change="onFileChange">
        <p v-if="avatarNotice" :class="avatarNotice.kind === 'ok' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'" class="text-sm" role="status">
          {{ avatarNotice.text }}
        </p>
      </section>

      <!-- 昵称 -->
      <section class="space-y-4 rounded-xl border border-slate-200 p-6 dark:border-slate-800" aria-label="昵称">
        <h2 class="font-medium">
          昵称
        </h2>
        <form class="flex gap-3" @submit.prevent="saveDisplayName">
          <input v-model="displayName" type="text" maxlength="32" :class="[inputClass, 'min-w-0 flex-1']">
          <button type="submit" :disabled="nameSaving || displayName.trim() === user.display_name" :class="primaryButtonClass">
            {{ nameSaving ? '保存中…' : '保存' }}
          </button>
        </form>
        <p v-if="nameNotice" :class="nameNotice.kind === 'ok' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'" class="text-sm" role="status">
          {{ nameNotice.text }}
        </p>
      </section>

      <!-- 密码 -->
      <section class="space-y-4 rounded-xl border border-slate-200 p-6 dark:border-slate-800" aria-label="密码">
        <h2 class="font-medium">
          修改密码
        </h2>
        <form class="space-y-3" @submit.prevent="savePassword">
          <input v-model="oldPassword" type="password" autocomplete="current-password" placeholder="当前密码" :class="inputClass">
          <input v-model="newPassword" type="password" autocomplete="new-password" placeholder="新密码（至少 8 位）" :class="inputClass">
          <input v-model="confirmPassword" type="password" autocomplete="new-password" placeholder="确认新密码" :class="inputClass">
          <p v-if="passwordNotice" :class="passwordNotice.kind === 'ok' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'" class="text-sm" role="status">
            {{ passwordNotice.text }}
          </p>
          <button type="submit" :disabled="passwordSaving" :class="primaryButtonClass">
            {{ passwordSaving ? '提交中…' : '修改密码' }}
          </button>
        </form>
      </section>
    </template>
    <div v-else class="space-y-4" aria-hidden="true">
      <div class="h-8 w-40 animate-pulse rounded-xl bg-slate-100 dark:bg-slate-800" />
      <div class="h-40 animate-pulse rounded-xl bg-slate-100 dark:bg-slate-800" />
      <div class="h-32 animate-pulse rounded-xl bg-slate-100 dark:bg-slate-800" />
    </div>
  </div>
</template>
