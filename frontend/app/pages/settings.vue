<script setup lang="ts">
// Personal settings: avatar upload, display name, password rotation. The
// page is client-side only: the login guard restores the session first and
// bounces anonymous visitors to /login?redirect=/settings. Avatars are
// canvas-downscaled to 512px and encoded client-side (webp, falling back to
// the source format), then uploaded as base64 — the format the backend
// contract expects (≤2MiB, magic-byte sniffed jpeg/png/webp).
import type { AuthUser } from '~/composables/useAuth'

useHead({ title: '个人设置' })

const route = useRoute()
const { user, authFetch, setUser, ensureSession, logout } = useAuth()
const { requestVerification } = useAccountEmail()

const settingsTabs = [
  { id: 'profile', label: '个人资料' },
  { id: 'email', label: '邮箱验证' },
  { id: 'security', label: '账号安全' },
] as const
type SettingsTab = typeof settingsTabs[number]['id']
const activeTab = ref<SettingsTab>('profile')
const tabList = ref<HTMLElement | null>(null)

// Keep the existing admin password link and direct email links usable.
watch(() => [route.hash, route.query.new_account] as const, ([hash, newAccount]) => {
  activeTab.value = hash === '#password' ? 'security' : hash === '#email' || (!hash && newAccount === '1') ? 'email' : 'profile'
}, { immediate: true })

function selectTab(tab: SettingsTab) {
  activeTab.value = tab
  nextTick(() => {
    tabList.value?.querySelector<HTMLElement>(`#settings-tab-${tab}`)
      ?.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'instant' })
  })
}

function onTabKeydown(event: KeyboardEvent, index: number) {
  let next: number
  switch (event.key) {
    case 'ArrowRight': next = (index + 1) % settingsTabs.length; break
    case 'ArrowLeft': next = (index - 1 + settingsTabs.length) % settingsTabs.length; break
    case 'Home': next = 0; break
    case 'End': next = settingsTabs.length - 1; break
    default: return
  }
  event.preventDefault()
  const tab = settingsTabs[next]!
  selectTab(tab.id)
  nextTick(() => tabList.value?.querySelector<HTMLElement>(`#settings-tab-${tab.id}`)?.focus({ preventScroll: true }))
}

const ready = ref(false)
const verificationPending = ref(false)
const verificationNotice = ref<{ kind: 'ok' | 'err', text: string } | null>(null)
onMounted(async () => {
  await ensureSession()
  if (!user.value) {
    navigateTo('/login?redirect=%2Fsettings', { replace: true })
    return
  }
  ready.value = true
})

async function resendVerification() {
  if (verificationPending.value || user.value?.email_verified) return
  verificationPending.value = true
  verificationNotice.value = null
  try {
    await requestVerification()
    verificationNotice.value = { kind: 'ok', text: '请求已接收；若邮件投递可用，请检查邮箱中的验证链接。' }
  }
  catch (err) {
    const reason = accountEmailReason(err)
    const status = accountEmailStatus(err)
    const text = status === 429 || ['AUTH_EMAIL_REQUEST_COOLDOWN', 'AUTH_EMAIL_TOO_MANY_ATTEMPTS'].includes(reason)
      ? '请求过于频繁，请稍后重试。'
      : reason === 'AUTH_MAIL_NOT_CONFIGURED'
        ? '邮件服务尚未配置，请稍后再试。'
        : reason === 'AUTH_EMAIL_DELIVERY_FAILED'
          ? '邮件投递失败，可稍后重试。'
          : '验证请求暂时无法处理，请稍后重试。'
    verificationNotice.value = { kind: 'err', text }
  }
  finally { verificationPending.value = false }
}

// --- avatar --------------------------------------------------------------

const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const previewUrl = ref('')
const uploading = ref(false)
const avatarNotice = ref<{ kind: 'ok' | 'err', text: string } | null>(null)

const MAX_AVATAR_BYTES = 2 * 1024 * 1024

onBeforeUnmount(() => {
  if (previewUrl.value)
    URL.revokeObjectURL(previewUrl.value)
})

function onFileChange(event: Event) {
  if (uploading.value)
    return
  avatarNotice.value = null
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file)
    return
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    avatarNotice.value = { kind: 'err', text: '请选择 jpg / png / webp 图片' }
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
      // Only retry when webp encoding fails; never race two encoders.
      canvas.toBlob((blob) => {
        if (blob) {
          handleBlob(blob)
          return
        }
        canvas.toBlob(handleBlob, file.type === 'image/png' ? 'image/png' : 'image/jpeg', 0.85)
      }, 'image/webp', 0.85)
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
    if (!updated?.avatar_url)
      throw new Error('invalid avatar response')
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
  if (passwordByteLength(newPassword.value) > PASSWORD_MAX_BYTES) {
    passwordNotice.value = { kind: 'err', text: '新密码过长，请控制在 1024 字节以内' }
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
    await logout().catch(() => {})
    await navigateTo('/login?password_changed=1', { replace: true })
  }
  catch (err) {
    const status = (err as { response?: { status?: number } } | null)?.response?.status
    passwordNotice.value = { kind: 'err', text: status === 401 ? '当前密码不正确' : status === 400 ? '新密码至少 8 位' : '修改失败，请稍后再试' }
  }
  finally {
    passwordSaving.value = false
  }
}

const inputClass = 'ui-field w-full px-3 py-2 text-sm'
const primaryButtonClass = 'ui-button shrink-0 whitespace-nowrap px-5 py-2 text-sm font-medium'
const avatarSrc = computed(() => (user.value?.avatar_url ? assetUrl(user.value.avatar_url) : ''))
</script>

<template>
  <div class="settings-page">
    <template v-if="ready && user">
      <header class="settings-heading">
        <h1 class="text-3xl font-semibold tracking-tight">个人设置</h1>
        <p class="settings-muted mt-2 text-sm">管理你的个人资料与账号安全。</p>
      </header>

      <section class="settings-summary settings-card" aria-label="账号概览">
        <img v-if="avatarSrc" :src="avatarSrc" alt="当前账号头像" class="settings-summary-avatar rounded-full object-cover">
        <span v-else class="settings-summary-avatar settings-avatar" aria-hidden="true">{{ (user.display_name || user.email).trim().charAt(0).toUpperCase() }}</span>
        <div class="settings-identity min-w-0">
          <h2 class="break-words text-sm font-semibold">{{ user.display_name }}</h2>
          <p class="settings-muted break-all text-xs">{{ user.email }}</p>
        </div>
        <span class="settings-badge settings-summary-badge" :class="user.email_verified ? 'settings-verified' : 'settings-unverified'">{{ user.email_verified ? '邮箱已验证' : '邮箱待验证' }}</span>
      </section>

      <div ref="tabList" class="settings-tabs" role="tablist" aria-label="设置分组">
        <button
          v-for="(tab, index) in settingsTabs" :id="`settings-tab-${tab.id}`" :key="tab.id"
          type="button" role="tab" :aria-selected="activeTab === tab.id"
          :aria-controls="`settings-panel-${tab.id}`" :tabindex="activeTab === tab.id ? 0 : -1"
          @click="selectTab(tab.id)" @keydown="onTabKeydown($event, index)"
        >{{ tab.label }}</button>
      </div>

      <!-- Keep panels mounted so switching groups preserves edits and selected files. -->
      <div v-show="activeTab === 'profile'" id="settings-panel-profile" role="tabpanel" aria-labelledby="settings-tab-profile">
        <div class="settings-profile-grid">
          <section class="settings-card settings-edit-card" aria-label="头像">
            <div class="settings-section-heading"><h2 class="font-semibold">头像</h2><p class="settings-muted mt-1 text-sm">为评论与互动设置一张头像。</p></div>
            <div class="settings-avatar-row">
              <img v-if="previewUrl || avatarSrc" :src="previewUrl || avatarSrc" alt="头像预览" class="h-16 w-16 shrink-0 rounded-full object-cover">
              <span v-else class="settings-avatar h-16 w-16 shrink-0 text-2xl" aria-hidden="true">{{ (user.display_name || user.email).trim().charAt(0).toUpperCase() }}</span>
              <div class="min-w-0 flex-1">
                <div class="settings-avatar-buttons">
                  <button type="button" :disabled="uploading" class="settings-secondary" @click="selectFile">选择图片</button>
                  <button type="button" :disabled="!selectedFile || uploading" :class="primaryButtonClass" @click="uploadAvatar">{{ uploading ? '上传中…' : '上传新头像' }}</button>
                </div>
                <p class="settings-hint mt-3">支持 JPG、PNG 或 WebP · 自动压缩至 512px · 最大 2MB</p>
              </div>
            </div>
            <input ref="fileInput" type="file" accept="image/jpeg,image/png,image/webp" :disabled="uploading" class="hidden" @change="onFileChange">
            <p v-if="avatarNotice" :class="avatarNotice.kind === 'ok' ? 'text-emerald-700 dark:text-emerald-300' : 'text-red-700 dark:text-red-300'" class="mt-4 text-sm" role="status">{{ avatarNotice.text }}</p>
          </section>

          <section class="settings-card settings-edit-card" aria-label="昵称">
            <div class="settings-section-heading"><h2 class="font-semibold">昵称</h2><p class="settings-muted mt-1 text-sm">让其他读者认识你。</p></div>
            <form @submit.prevent="saveDisplayName">
              <label for="settings-display-name" class="mb-2 block text-sm font-medium">昵称</label>
              <input id="settings-display-name" v-model="displayName" type="text" maxlength="32" autocomplete="nickname" :class="inputClass">
              <button type="submit" :disabled="nameSaving || displayName.trim() === user.display_name" :class="[primaryButtonClass, 'settings-name-save mt-5']">{{ nameSaving ? '保存中…' : '保存昵称' }}</button>
            </form>
            <p class="settings-hint mt-3">昵称会显示在评论区与右上角菜单。</p>
            <p v-if="nameNotice" :class="nameNotice.kind === 'ok' ? 'text-emerald-700 dark:text-emerald-300' : 'text-red-700 dark:text-red-300'" class="mt-4 text-sm" role="status">{{ nameNotice.text }}</p>
          </section>
        </div>
      </div>

      <div v-show="activeTab === 'email'" id="settings-panel-email" role="tabpanel" aria-labelledby="settings-tab-email">
        <section id="email" class="settings-card settings-edit-card" aria-label="邮箱验证">
          <div class="settings-section-heading settings-divider"><h2 class="font-semibold">邮箱验证</h2><p class="settings-muted mt-1 text-sm">查看当前邮箱的验证状态。</p></div>
          <div class="settings-email-row">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-3"><p class="break-all text-sm">{{ user.email }}</p><span class="settings-badge" :class="user.email_verified ? 'settings-verified' : 'settings-unverified'">{{ user.email_verified ? '已验证' : '待验证' }}</span></div>
              <p v-if="!user.email_verified" class="settings-muted mt-3 text-sm"><template v-if="route.query.new_account === '1'">账号已创建。</template>验证邮箱有助于找回密码，当前不会限制登录、评论或其他功能。</p>
            </div>
            <button v-if="!user.email_verified" type="button" :disabled="verificationPending" class="settings-secondary" @click="resendVerification">{{ verificationPending ? '提交中…' : '发送验证请求' }}</button>
          </div>
          <p v-if="verificationNotice" role="status" class="mt-4 text-sm" :class="verificationNotice.kind === 'ok' ? 'text-emerald-700 dark:text-emerald-300' : 'text-red-700 dark:text-red-300'">{{ verificationNotice.text }}</p>
        </section>
      </div>

      <div v-show="activeTab === 'security'" id="settings-panel-security" role="tabpanel" aria-labelledby="settings-tab-security">
        <section id="password" class="settings-card settings-edit-card" aria-label="密码">
          <div class="settings-section-heading settings-divider"><h2 class="font-semibold">修改密码</h2><p class="settings-muted mt-1 text-sm">更新用于登录的密码。</p></div>
          <form class="settings-password-form space-y-4" @submit.prevent="savePassword">
            <div><label for="settings-old-password" class="mb-2 block text-sm font-medium">当前密码</label><input id="settings-old-password" v-model="oldPassword" type="password" autocomplete="current-password" placeholder="当前密码" :class="inputClass"></div>
            <div><label for="settings-new-password" class="mb-2 block text-sm font-medium">新密码（至少 8 位）</label><input id="settings-new-password" v-model="newPassword" type="password" autocomplete="new-password" placeholder="新密码（至少 8 位）" :class="inputClass"></div>
            <div><label for="settings-confirm-password" class="mb-2 block text-sm font-medium">确认新密码</label><input id="settings-confirm-password" v-model="confirmPassword" type="password" autocomplete="new-password" placeholder="确认新密码" :class="inputClass"></div>
            <p class="settings-hint">修改成功后需要重新登录。</p>
            <p v-if="passwordNotice" :class="passwordNotice.kind === 'ok' ? 'text-emerald-700 dark:text-emerald-300' : 'text-red-700 dark:text-red-300'" class="text-sm" role="status">{{ passwordNotice.text }}</p>
            <button type="submit" :disabled="passwordSaving" :class="primaryButtonClass">{{ passwordSaving ? '提交中…' : '修改密码' }}</button>
          </form>
        </section>
      </div>
    </template>
    <div v-else class="space-y-4" aria-hidden="true">
      <div class="h-8 w-40 animate-pulse rounded-xl bg-slate-100 dark:bg-slate-800" />
      <div class="h-24 animate-pulse rounded-xl bg-slate-100 dark:bg-slate-800" />
      <div class="h-64 animate-pulse rounded-xl bg-slate-100 dark:bg-slate-800" />
    </div>
  </div>
</template>

<style scoped>
.settings-page { width: 100%; min-width: 0; }
.settings-heading { margin-bottom: 28px; }
.settings-muted { color: var(--text-muted); }
.settings-hint { font-size: 12px; color: var(--text-muted); }
.settings-card { min-width: 0; border: 1px solid var(--border-default); border-radius: var(--radius-panel); background: var(--surface-panel); }
.settings-summary { display: flex; align-items: center; gap: 14px; padding: 18px 22px; margin-bottom: 26px; }
.settings-summary-avatar { width: 48px; height: 48px; flex-shrink: 0; }
.settings-avatar { display: grid; place-items: center; border-radius: 50%; background: var(--surface-hover); color: var(--accent); font-weight: 600; }
.settings-summary .settings-avatar { font-size: 21px; }
.settings-summary-badge { margin-left: auto; flex-shrink: 0; }
.settings-badge { display: inline-block; padding: 4px 11px; border-radius: 999px; font-size: 12px; white-space: nowrap; }
.settings-unverified { background: var(--status-warning-surface); color: var(--status-warning); }
.settings-verified { background: var(--mint-surface); color: var(--mint-text); }
.settings-tabs { display: flex; gap: 28px; overflow-x: auto; overscroll-behavior-x: contain; scrollbar-width: thin; margin-bottom: 26px; border-bottom: 1px solid var(--border-default); padding: 4px 4px 0; }
.settings-tabs button { position: relative; flex-shrink: 0; min-height: 44px; padding: 10px 0 13px; font-size: 14px; white-space: nowrap; color: var(--text-muted); }
.settings-tabs button[aria-selected='true'] { color: var(--accent); font-weight: 600; }
.settings-tabs button[aria-selected='true']::after { content: ''; position: absolute; left: 0; right: 0; bottom: 0; height: 2px; background: var(--accent); }
.settings-profile-grid { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 20px; }
.settings-edit-card { padding: 24px; }
.settings-section-heading { margin-bottom: 22px; }
.settings-divider { padding-bottom: 18px; border-bottom: 1px solid var(--border-default); }
.settings-avatar-row { display: flex; align-items: flex-start; gap: 20px; }
.settings-avatar-buttons { display: flex; flex-wrap: wrap; gap: 8px; }
.settings-avatar-buttons button { padding-left: 13px; padding-right: 13px; }
.settings-secondary { min-height: 44px; padding: 9px 17px; border: 1px solid var(--border-strong); border-radius: var(--radius-control); color: var(--text-primary); background: var(--surface-input); white-space: nowrap; }
.settings-secondary:hover { border-color: var(--accent); color: var(--accent); }
.settings-secondary:disabled { opacity: 0.55; cursor: not-allowed; }
.settings-email-row { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
.settings-password-form { max-width: 460px; }
@media (max-width: 640px) {
  .settings-heading h1 { font-size: 26px; }
  .settings-summary { display: grid; grid-template-columns: 48px minmax(0, 1fr); gap: 8px 12px; padding: 16px; margin-bottom: 22px; }
  .settings-summary-avatar { grid-row: 1 / 3; align-self: start; }
  .settings-summary-badge { grid-column: 2; justify-self: start; margin: 0; }
  .settings-tabs { gap: 26px; margin-bottom: 20px; }
  .settings-profile-grid { grid-template-columns: minmax(0, 1fr); gap: 16px; }
  .settings-edit-card { padding: 20px; }
  .settings-avatar-row { gap: 16px; }
  .settings-avatar-buttons { flex-direction: column; }
  .settings-name-save, .settings-password-form button { width: 100%; }
  .settings-email-row { align-items: flex-start; flex-direction: column; gap: 18px; }
  .settings-password-form { max-width: none; }
}
</style>
