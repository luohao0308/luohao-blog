<script setup lang="ts">
// Admin-only chrome: slim top bar with management entry points and the
// signed-in account. Follows the site dark mode through the html.dark class.
// naive-ui is themed to the site palette (#3c5d85 brand, 8px radius) so the
// management pages read as part of the same site in both modes.
import { NButton, NConfigProvider, NDialogProvider, NForm, NFormItem, NInput, NMessageProvider, NModal, darkTheme, dateZhCN, zhCN, type FormInst, type FormRules, type GlobalThemeOverrides } from 'naive-ui'

// Light mode: the site's brand blue; dark mode: the blue-300 the site's
// dark: variants use for accents.
const lightOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#3c5d85',
    primaryColorHover: '#2d486b',
    primaryColorPressed: '#24405f',
    primaryColorSuppl: '#3c5d85',
    borderRadius: '8px',
  },
}
const darkOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#93c5fd',
    primaryColorHover: '#bfdbfe',
    primaryColorPressed: '#60a5fa',
    primaryColorSuppl: '#93c5fd',
    borderRadius: '8px',
  },
}

const { user, logout, ensureSession, authFetch } = useAuth()
const { preference, init } = useTheme()

// matchMedia is client-only, so system preference tracking starts empty and
// hydrates on mount alongside the persisted theme choice.
const systemDark = ref(false)
onMounted(() => {
  init()
  systemDark.value = window.matchMedia('(prefers-color-scheme: dark)').matches
  ensureSession()
})

const isDark = computed(() =>
  preference.value === 'dark' || (preference.value === 'system' && systemDark.value),
)

// Password rotation: the modal posts to the authenticated update-password
// endpoint; the backend verifies the old password before storing the new
// one. Errors render inline — the layout is the message provider itself, so
// naive-ui message APIs are unavailable at this level.
const pwModalVisible = ref(false)
const pwSubmitting = ref(false)
const pwError = ref('')
const pwForm = reactive({ old_password: '', new_password: '', confirm: '' })
const pwFormRef = ref<FormInst | null>(null)

const pwRules: FormRules = {
  old_password: { required: true, message: '请输入当前密码', trigger: 'blur' },
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 8, message: '新密码至少 8 位', trigger: 'blur' },
  ],
  confirm: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    {
      validator: (_rule, value: string) => value === pwForm.new_password,
      message: '两次输入的新密码不一致',
      trigger: 'blur',
    },
  ],
}

async function submitPassword() {
  pwError.value = ''
  pwFormRef.value?.validate(async (errors) => {
    if (errors)
      return
    pwSubmitting.value = true
    try {
      await authFetch('/api/v1/auth/update-password', {
        method: 'POST',
        body: { old_password: pwForm.old_password, new_password: pwForm.new_password },
      })
      pwModalVisible.value = false
      pwForm.old_password = ''
      pwForm.new_password = ''
      pwForm.confirm = ''
    }
    catch (err: unknown) {
      const status = (err as { response?: { status?: number } } | null)?.response?.status
      pwError.value = status === 401 ? '当前密码不正确' : '修改失败，请稍后再试'
    }
    finally {
      pwSubmitting.value = false
    }
  })
}

// No useMessage here: this layout is the provider itself, and naive-ui
// message APIs only resolve inside its descendants.
async function onLogout() {
  await logout()
  navigateTo('/admin/login', { replace: true })
}
</script>

<template>
  <NConfigProvider :theme="isDark ? darkTheme : null" :theme-overrides="isDark ? darkOverrides : lightOverrides" :locale="zhCN" :date-locale="dateZhCN">
    <NMessageProvider>
      <NDialogProvider>
        <div class="flex min-h-screen flex-col bg-white text-slate-900 antialiased transition-colors dark:bg-slate-950 dark:text-slate-100">
          <header class="sticky top-0 z-40 border-b border-slate-200/70 bg-white/80 backdrop-blur dark:border-slate-800 dark:bg-slate-950/80">
            <div class="mx-auto flex max-w-6xl items-center justify-between px-6 py-3">
              <div class="flex items-center gap-6">
                <NuxtLink to="/" class="text-lg font-bold tracking-tight">
                  luohao<span class="text-blue-600 dark:text-blue-400">.blog</span>
                </NuxtLink>
                <span class="text-sm text-slate-500 dark:text-slate-400">管理后台</span>
                <NuxtLink to="/admin/posts" class="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100">
                  文章管理
                </NuxtLink>
                <NuxtLink to="/admin/categories" class="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100">
                  分类管理
                </NuxtLink>
                <NuxtLink to="/admin/comments" class="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100">
                  评论管理
                </NuxtLink>
                <NuxtLink to="/admin/subscribers" class="text-sm text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-100">
                  订阅管理
                </NuxtLink>
              </div>
              <div class="flex items-center gap-3 text-sm">
                <span v-if="user" class="text-slate-600 dark:text-slate-300">{{ user.display_name }}</span>
                <NButton size="small" quaternary @click="pwModalVisible = true">
                  修改密码
                </NButton>
                <NButton size="small" quaternary @click="onLogout">
                  退出
                </NButton>
              </div>
            </div>
          </header>

      <NModal
        v-model:show="pwModalVisible"
        preset="card"
        class="w-[24rem]"
        title="修改密码"
      >
        <NForm ref="pwFormRef" :model="pwForm" :rules="pwRules" label-placement="top" :show-require-mark="false">
          <NFormItem label="当前密码" path="old_password" :show-feedback="false">
            <NInput v-model:value="pwForm.old_password" type="password" show-password-on="click" placeholder="当前密码" />
          </NFormItem>
          <NFormItem label="新密码（至少 8 位）" path="new_password" :show-feedback="false">
            <NInput v-model:value="pwForm.new_password" type="password" show-password-on="click" placeholder="新密码" />
          </NFormItem>
          <NFormItem label="确认新密码" path="confirm" :show-feedback="false">
            <NInput v-model:value="pwForm.confirm" type="password" show-password-on="click" placeholder="再次输入新密码" />
          </NFormItem>
        </NForm>
        <p v-if="pwError" class="mb-3 text-sm text-red-600 dark:text-red-400" role="alert">
          {{ pwError }}
        </p>
        <template #footer>
          <div class="flex justify-end gap-3">
            <NButton @click="pwModalVisible = false">
              取消
            </NButton>
            <NButton type="primary" :loading="pwSubmitting" @click="submitPassword">
              确认修改
            </NButton>
          </div>
        </template>
      </NModal>

      <main class="mx-auto w-full max-w-6xl flex-1 px-6 py-8">
        <slot />
      </main>
        </div>
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>
