<script setup lang="ts">
// Shared create/edit form for admin articles. The page owns the API call;
// this component owns field state and client-side validation. Mirrors the
// backend rules: slug is URL-safe lowercase and immutable after creation.
import { NButton, NDynamicTags, NForm, NFormItem, NInput, NRadio, NRadioGroup, NSelect } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { ARTICLE_STATUS, type Article } from '~/composables/useArticles'
import { ADMIN_MEDIA_MAX_SOURCE_BYTES, ADMIN_MEDIA_TYPES, adminMediaError, useAdminMedia } from '~/composables/useAdminMedia'
import { articleVersion } from '~/utils/articleVersion'
import { useAdminCategories } from '~/composables/useCategories'

const props = defineProps<{
  initial?: Article | null
  submitting?: boolean
  frozen?: boolean
}>()

const emit = defineEmits<{
  submit: [payload: { title: string, slug: string, summary: string, content_md: string, tags: string[], category_slug: string | null, status: number, cover_url: string, featured: boolean, expected_version: string }]
}>()

const { uploadImage } = useAdminMedia()
const imageInput = ref<HTMLInputElement | null>(null)
const editorRef = ref<{ insertMarkdown: (markdown: string) => boolean } | null>(null)
const uploadingImage = ref(false)
const uploadedImage = ref('')
const mediaNotice = ref<{ kind: 'ok' | 'error', text: string } | null>(null)

// Categories are fetched once per form mount; clearing the select sends
// null, which the owning page maps to an empty category brief (未分类).
const { categories, load: loadCategories } = useAdminCategories()
onMounted(loadCategories)
const categoryOptions = computed(() =>
  categories.value.map(c => ({ label: c.name, value: c.slug })),
)

const form = reactive({
  title: props.initial?.title ?? '',
  slug: props.initial?.slug ?? '',
  summary: props.initial?.summary ?? '',
  content_md: props.initial?.content_md ?? '',
  tags: props.initial?.tags ? [...props.initial.tags] : [],
  category_slug: props.initial?.category?.slug ?? null,
  status: props.initial?.status ?? ARTICLE_STATUS.DRAFT,
  cover_url: props.initial?.cover_url ?? '',
  featured: props.initial?.featured ?? false,
})

// Compare the normalized API fields, with detached arrays for in-flight saves.
function snapshot() {
  return { ...form, title: form.title.trim(), tags: [...form.tags] }
}
const { user } = useAuth()
const draftKey = user.value ? `blog:article-draft:${user.value.id}:${props.initial?.slug ?? 'new'}` : ''
let tabOwner = ''
if (import.meta.client) {
  try {
    tabOwner = sessionStorage.getItem('blog:article-draft-tab') || crypto.randomUUID()
    sessionStorage.setItem('blog:article-draft-tab', tabOwner)
  } catch { tabOwner = crypto.randomUUID() }
}
const tabDraftKey = draftKey ? `${draftKey}:${tabOwner}` : ''
const recoveryRecord = ref('')
const baseVersion = ref(articleVersion(props.initial?.version))
const recovery = ref<(ReturnType<typeof snapshot> & { base_version: string }) | null>(null)
const draftNotice = ref('')
const editorVersion = ref(0)
const applyingServer = ref(false)
const blocked = computed(() => !!props.frozen || !!recovery.value)
function persistDraft() {
  if (!draftKey || recovery.value) return
  try {
    if (dirty.value) {
      const record = JSON.stringify({ ...snapshot(), base_version: baseVersion.value, owner: tabOwner })
      localStorage.setItem(tabDraftKey, record)
      localStorage.setItem(draftKey, record)
      draftNotice.value = '未保存到服务器 · 已暂存到此浏览器'
    } else {
      for (const key of [tabDraftKey, draftKey]) {
        const record = localStorage.getItem(key)
        if (!record) continue
        const { owner, base_version: _base, ...content } = JSON.parse(record)
        if (owner === tabOwner && JSON.stringify(content) === saved.value) localStorage.removeItem(key)
      }
      draftNotice.value = '已保存到服务器'
    }
  } catch {
    draftNotice.value = dirty.value ? '本地暂存失败，请及时手动保存；离开可能丢失修改。'
      : '正文已保存，但本地旧草稿清理失败；恢复前请核对服务器内容。'
  }
}
function restoreDraft() {
  if (!recovery.value) return
  const { base_version, ...content } = recovery.value
  baseVersion.value = base_version
  Object.assign(form, content, props.initial ? { slug: props.initial.slug } : {})
  recovery.value = null
  editorVersion.value++
  persistDraft()
}
function discardDraft() {
  try {
    for (const key of [tabDraftKey, draftKey]) {
      if (localStorage.getItem(key) === recoveryRecord.value) localStorage.removeItem(key)
    }
  } catch { draftNotice.value = '本地旧草稿清理失败，请在恢复前核对服务器内容。' }
  recovery.value = null
  persistDraft()
}
onMounted(() => {
  try {
    const raw = draftKey ? (localStorage.getItem(tabDraftKey) || localStorage.getItem(draftKey)) : null
    recoveryRecord.value = raw || ''
    if (raw) {
      const candidate = JSON.parse(raw)
      if (candidate && ['title', 'slug', 'summary', 'content_md'].every(key => typeof candidate[key] === 'string')
        && Array.isArray(candidate.tags) && candidate.tags.every((tag: unknown) => typeof tag === 'string')
        && (candidate.category_slug === null || typeof candidate.category_slug === 'string') && [1, 2].includes(candidate.status)) {
        const restored = { title: candidate.title, slug: candidate.slug, summary: candidate.summary, content_md: candidate.content_md,
          tags: candidate.tags, category_slug: candidate.category_slug, status: candidate.status,
          cover_url: typeof candidate.cover_url === 'string' ? candidate.cover_url : form.cover_url,
          featured: typeof candidate.featured === 'boolean' ? candidate.featured : form.featured,
          base_version: articleVersion(candidate.base_version) }
        if (JSON.stringify({ ...restored, base_version: undefined }) !== saved.value || restored.base_version !== baseVersion.value) recovery.value = restored
      } else draftNotice.value = '本地草稿格式无效，请手动保存当前内容。'
    }
  } catch { draftNotice.value = '本地草稿不可读取，请及时手动保存。' }
})
watch(form, () => { if (!applyingServer.value) persistDraft() }, { deep: true, flush: 'sync' })
const saved = ref(JSON.stringify(snapshot()))
const dirty = computed(() => JSON.stringify(snapshot()) !== saved.value)
function markSaved(payload: ReturnType<typeof snapshot>, version?: string) {
  baseVersion.value = articleVersion(version)
  recovery.value = null
  const { expected_version: _version, ...submitted } = payload as ReturnType<typeof snapshot> & { expected_version?: string }
  saved.value = JSON.stringify(submitted)
  persistDraft()
}
const canRestore = computed(() => !dirty.value && !recovery.value && !uploadingImage.value && !!baseVersion.value)
function replaceWithServer(article: Article, expected: string): boolean {
  const version = articleVersion(article.version)
  if (!canRestore.value || baseVersion.value !== expected || !version || BigInt(version) <= BigInt(expected) || article.slug !== form.slug) return false
  // Clear only records corresponding to this clean pre-restore baseline.
  try {
    for (const key of [tabDraftKey, draftKey]) {
      const raw = localStorage.getItem(key)
      if (!raw) continue
      const { owner, base_version, ...content } = JSON.parse(raw)
      if (owner === tabOwner && base_version === expected && JSON.stringify(content) === saved.value && localStorage.getItem(key) === raw) localStorage.removeItem(key)
    }
  } catch { draftNotice.value = '恢复已保存，但本地旧草稿清理失败，请核对后再恢复本地草稿。' }
  applyingServer.value = true
  Object.assign(form, { title: article.title, slug: article.slug, summary: article.summary, content_md: article.content_md,
    tags: [...(article.tags || [])], category_slug: article.category?.slug ?? null, status: article.status,
    cover_url: article.cover_url || '', featured: article.featured ?? false })
  baseVersion.value = version
  saved.value = JSON.stringify(snapshot())
  editorVersion.value++
  applyingServer.value = false
  return true
}
defineExpose({ markSaved, dirty, baseVersion, canRestore, replaceWithServer })

function confirmLeave() {
  return !dirty.value || window.confirm('有未保存的修改，确定离开并丢弃吗？')
}
onBeforeRouteLeave(confirmLeave)
onBeforeRouteUpdate((to, from) => to.params.slug === from.params.slug || confirmLeave())
function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value) {
    event.preventDefault()
    event.returnValue = ''
  }
}
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))

const formRef = ref<FormInst | null>(null)

const rules: FormRules = {
  title: { required: true, message: '请输入标题', trigger: 'blur' },
  slug: [
    { required: true, message: '请输入 slug', trigger: 'blur' },
    { pattern: /^[a-z0-9-]+$/, message: '仅允许小写字母、数字和连字符', trigger: 'blur' },
  ],
  content_md: { required: true, message: '正文不能为空', trigger: 'blur' },
}

function submit(e: Event) {
  e.preventDefault()
  if (props.submitting || blocked.value || uploadingImage.value)
    return
  if (props.initial && !baseVersion.value) {
    draftNotice.value = '缺少草稿基准版本，不能覆盖服务器内容。请打开最新版本对照后手动整理。'
    return
  }
  formRef.value?.validate((errors) => {
    if (!errors && !props.submitting && !blocked.value && !uploadingImage.value)
      emit('submit', { ...snapshot(), expected_version: baseVersion.value })
  })
}

async function chooseImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || uploadingImage.value || blocked.value)
    return
  mediaNotice.value = null
  if (!ADMIN_MEDIA_TYPES.includes(file.type)) {
    mediaNotice.value = { kind: 'error', text: '仅支持 jpg、png 或 webp 图片' }
    return
  }
  if (file.size > ADMIN_MEDIA_MAX_SOURCE_BYTES) {
    mediaNotice.value = { kind: 'error', text: '原始图片大小不能超过 2MB' }
    return
  }
  uploadingImage.value = true
  try {
    const url = await uploadImage(file)
    uploadedImage.value = url
    const markdown = `![文章图片](${assetUrl(url)})`
    if (!editorRef.value?.insertMarkdown(markdown))
      form.content_md = `${form.content_md.trimEnd()}\n\n${markdown}\n`
    mediaNotice.value = { kind: 'ok', text: '图片已插入正文，可继续设为封面' }
  }
  catch (err) {
    const invalidResponse = err instanceof Error && err.message === 'invalid media response'
    mediaNotice.value = { kind: 'error', text: invalidResponse ? '上传结果无效，请重试' : adminMediaError(err) }
  }
  finally {
    uploadingImage.value = false
  }
}

function setCover() {
  if (!uploadedImage.value || blocked.value)
    return
  form.cover_url = uploadedImage.value
  mediaNotice.value = { kind: 'ok', text: '已设为文章封面' }
}
</script>

<template>
  <div class="space-y-4">
    <div v-if="recovery" role="alert" class="flex flex-wrap items-center gap-3 rounded-xl border border-amber-300 p-3 text-sm">
      <span>发现此账号在此浏览器的未保存草稿。</span>
      <button type="button" class="underline" @click="restoreDraft">恢复草稿</button>
      <button type="button" class="underline" @click="discardDraft">丢弃草稿</button>
    </div>
    <a v-if="props.initial && !baseVersion" :href="`/admin/posts/${encodeURIComponent(props.initial.slug)}/edit`" target="_blank" rel="noopener noreferrer" class="ui-link text-sm">打开最新版本对照</a>
    <p v-if="draftNotice" role="status" class="text-sm text-[var(--text-muted)]">{{ draftNotice }}</p>
  <NForm ref="formRef" class="space-y-6" :disabled="blocked" :model="form" :rules="rules" label-placement="top" :show-require-mark="false" @submit.prevent="submit">
      <div class="grid gap-x-6 sm:grid-cols-2">
        <NFormItem label="标题" path="title" :show-feedback="false">
          <NInput v-model:value="form.title" :disabled="blocked" placeholder="文章标题" />
        </NFormItem>
        <NFormItem label="Slug（创建后不可修改）" path="slug" :show-feedback="false">
          <NInput v-model:value="form.slug" :disabled="!!props.initial || blocked" placeholder="url-safe-slug" />
        </NFormItem>
      </div>

      <NFormItem label="摘要（留空则从正文截取）" path="summary" :show-feedback="false">
        <NInput v-model:value="form.summary" :disabled="blocked" type="textarea" :rows="2" placeholder="显示在列表页的短摘要" />
      </NFormItem>

      <div class="grid gap-x-6 sm:grid-cols-2">
        <NFormItem label="标签" path="tags" :show-feedback="false">
          <NDynamicTags v-model:value="form.tags" :disabled="blocked" />
        </NFormItem>
        <NFormItem label="分类" path="category_slug" :show-feedback="false">
          <NSelect
            v-model:value="form.category_slug"
            :disabled="blocked"
            :options="categoryOptions"
            clearable
            placeholder="未分类"
          />
        </NFormItem>
      </div>

      <NFormItem label="正文" path="content_md">
        <div :inert="blocked" class="w-full space-y-2">
          <div class="flex flex-wrap items-center gap-2">
            <input
              ref="imageInput"
              data-testid="article-image-input"
              :disabled="blocked || uploadingImage"
              class="sr-only"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              @change="chooseImage"
            >
            <NButton attr-type="button" size="small" :loading="uploadingImage" :disabled="blocked" @click="imageInput?.click()">
              上传图片并插入正文
            </NButton>
            <NButton v-if="uploadedImage" attr-type="button" size="small" secondary :disabled="uploadingImage || blocked" @click="setCover">
              设为封面
            </NButton>
            <span
              v-if="mediaNotice"
              data-testid="media-notice"
              class="text-xs"
              :class="mediaNotice.kind === 'ok' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'"
            >{{ mediaNotice.text }}</span>
          </div>
          <AdminMarkdownEditor :key="editorVersion" ref="editorRef" v-model="form.content_md" :disabled="blocked" />
        </div>
      </NFormItem>

      <div class="grid gap-5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start">
        <NFormItem label="文章封面" path="cover_url" :show-feedback="false">
          <div class="w-full space-y-3">
            <div v-if="form.cover_url" class="overflow-hidden rounded-xl border border-slate-200 bg-slate-50 dark:border-slate-700 dark:bg-slate-900">
              <img :src="assetUrl(form.cover_url)" alt="文章封面预览" class="aspect-[16/9] w-full object-cover">
            </div>
            <div class="flex items-center gap-3">
              <span class="text-xs text-slate-500 dark:text-slate-400">上传正文图片后可将其设为封面</span>
              <NButton v-if="form.cover_url" :disabled="blocked" attr-type="button" size="tiny" quaternary type="error" @click="form.cover_url = ''">
                清除封面
              </NButton>
            </div>
          </div>
        </NFormItem>
        <NFormItem label="首页展示" path="featured" :show-feedback="false">
          <label class="inline-flex min-h-11 cursor-pointer items-center gap-2">
            <input v-model="form.featured" type="checkbox" :disabled="blocked" class="h-4 w-4 accent-[var(--accent)]">
            <span>首页精选</span>
          </label>
        </NFormItem>
      </div>
    <div class="flex flex-wrap items-center gap-6">
      <!-- CreateArticle always starts as DRAFT server-side, so the status
           choice only makes sense when editing an existing article. -->
      <NRadioGroup v-if="props.initial" v-model:value="form.status" :disabled="blocked">
        <NRadio :value="1">
          草稿
        </NRadio>
        <NRadio :value="2">
          发布
        </NRadio>
      </NRadioGroup>
      <slot name="actions" :submit="submit" :form="form" />
    </div>
  </NForm>
  </div>
</template>
