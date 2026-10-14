<script setup lang="ts">
// Shared create/edit form for admin articles. The page owns the API call;
// this component owns field state and client-side validation. Mirrors the
// backend rules: slug is URL-safe lowercase and immutable after creation.
import { NButton, NCheckbox, NDynamicTags, NForm, NFormItem, NInput, NRadio, NRadioGroup, NSelect } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { ARTICLE_STATUS, type Article } from '~/composables/useArticles'
import { useAdminCategories } from '~/composables/useCategories'

const props = defineProps<{
  initial?: Article | null
  submitting?: boolean
}>()

const emit = defineEmits<{
  submit: [payload: { title: string, slug: string, summary: string, content_md: string, tags: string[], category_slug: string | null, status: number, cover_url: string, featured: boolean }]
}>()

const { authFetch } = useAuth()
const imageInput = ref<HTMLInputElement | null>(null)
const editorRef = ref<{ insertMarkdown: (markdown: string) => boolean } | null>(null)
const uploadingImage = ref(false)
const uploadedImage = ref('')
const mediaNotice = ref<{ kind: 'ok' | 'error', text: string } | null>(null)
const MAX_SOURCE_BYTES = 2 * 1024 * 1024
const MAX_ENCODED_BYTES = 1024 * 1024

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
  formRef.value?.validate((errors) => {
    if (!errors)
      emit('submit', {
        title: form.title.trim(),
        slug: form.slug,
        summary: form.summary,
        content_md: form.content_md,
        tags: form.tags,
        category_slug: form.category_slug,
        status: form.status,
        cover_url: form.cover_url,
        featured: form.featured,
      })
  })
}

function encodeImage(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const objectURL = URL.createObjectURL(file)
    const image = new Image()
    image.onload = () => {
      URL.revokeObjectURL(objectURL)
      const scale = Math.min(1, 1600 / Math.max(image.naturalWidth, image.naturalHeight))
      const canvas = document.createElement('canvas')
      canvas.width = Math.max(1, Math.round(image.naturalWidth * scale))
      canvas.height = Math.max(1, Math.round(image.naturalHeight * scale))
      const context = canvas.getContext('2d')
      if (!context) {
        reject(new Error('canvas unavailable'))
        return
      }
      // JPEG keeps WeChat-compatible article images compact. A white canvas
      // makes transparent PNG/WebP pixels deterministic instead of black.
      context.fillStyle = '#ffffff'
      context.fillRect(0, 0, canvas.width, canvas.height)
      context.drawImage(image, 0, 0, canvas.width, canvas.height)
      const qualities = [0.88, 0.78, 0.68, 0.58]
      const tryEncode = (index: number) => {
        canvas.toBlob((blob) => {
          if (!blob) {
            reject(new Error('encode failed'))
            return
          }
          if (blob.size > MAX_ENCODED_BYTES && index + 1 < qualities.length) {
            tryEncode(index + 1)
            return
          }
          if (blob.size > MAX_ENCODED_BYTES) {
            reject(new Error('encoded image too large'))
            return
          }
          const reader = new FileReader()
          reader.onload = () => {
            const data = String(reader.result).split(',')[1] ?? ''
            if (!data) reject(new Error('empty image'))
            else resolve(data)
          }
          reader.onerror = () => reject(new Error('read failed'))
          reader.readAsDataURL(blob)
        }, 'image/jpeg', qualities[index])
      }
      tryEncode(0)
    }
    image.onerror = () => {
      URL.revokeObjectURL(objectURL)
      reject(new Error('not an image'))
    }
    image.src = objectURL
  })
}

function mediaError(err: unknown): string {
  const status = (err as { response?: { status?: number }, status?: number } | null)?.response?.status
    ?? (err as { status?: number } | null)?.status
  if (status === 400)
    return '图片不符合要求：仅支持 jpg/png/webp，且不超过 2MB'
  if (status === 401 || status === 403)
    return '当前账号没有上传权限'
  return '图片上传失败，请稍后重试'
}

async function chooseImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || uploadingImage.value)
    return
  mediaNotice.value = null
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    mediaNotice.value = { kind: 'error', text: '仅支持 jpg、png 或 webp 图片' }
    return
  }
  if (file.size > MAX_SOURCE_BYTES) {
    mediaNotice.value = { kind: 'error', text: '原始图片大小不能超过 2MB' }
    return
  }
  uploadingImage.value = true
  try {
    const data = await encodeImage(file)
    const response = await authFetch<{ url?: string }>('/api/v1/admin/media/images', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: { data },
    })
    if (!response?.url || !/^\/v1\/assets\/media\/[0-9a-f]{32}\.(?:jpg|png|webp)$/.test(response.url)) {
      mediaNotice.value = { kind: 'error', text: '上传结果无效，请重试' }
      return
    }
    uploadedImage.value = response.url
    const markdown = `![文章图片](${assetUrl(response.url)})`
    if (!editorRef.value?.insertMarkdown(markdown))
      form.content_md = `${form.content_md.trimEnd()}\n\n${markdown}\n`
    mediaNotice.value = { kind: 'ok', text: '图片已插入正文，可继续设为封面' }
  }
  catch (err) {
    mediaNotice.value = { kind: 'error', text: mediaError(err) }
  }
  finally {
    uploadingImage.value = false
  }
}

function setCover() {
  if (!uploadedImage.value)
    return
  form.cover_url = uploadedImage.value
  mediaNotice.value = { kind: 'ok', text: '已设为文章封面' }
}
</script>

<template>
  <form class="space-y-6" @submit="submit">
    <NForm ref="formRef" :model="form" :rules="rules" label-placement="top" :show-require-mark="false">
      <div class="grid gap-x-6 sm:grid-cols-2">
        <NFormItem label="标题" path="title" :show-feedback="false">
          <NInput v-model:value="form.title" placeholder="文章标题" />
        </NFormItem>
        <NFormItem label="Slug（创建后不可修改）" path="slug" :show-feedback="false">
          <NInput v-model:value="form.slug" :disabled="!!props.initial" placeholder="url-safe-slug" />
        </NFormItem>
      </div>

      <NFormItem label="摘要（留空则从正文截取）" path="summary" :show-feedback="false">
        <NInput v-model:value="form.summary" type="textarea" :rows="2" placeholder="显示在列表页的短摘要" />
      </NFormItem>

      <div class="grid gap-x-6 sm:grid-cols-2">
        <NFormItem label="标签" path="tags" :show-feedback="false">
          <NDynamicTags v-model:value="form.tags" />
        </NFormItem>
        <NFormItem label="分类" path="category_slug" :show-feedback="false">
          <NSelect
            v-model:value="form.category_slug"
            :options="categoryOptions"
            clearable
            placeholder="未分类"
          />
        </NFormItem>
      </div>

      <NFormItem label="正文" path="content_md">
        <div class="w-full space-y-2">
          <div class="flex flex-wrap items-center gap-2">
            <input
              ref="imageInput"
              data-testid="article-image-input"
              class="sr-only"
              type="file"
              accept="image/jpeg,image/png,image/webp"
              @change="chooseImage"
            >
            <NButton attr-type="button" size="small" :loading="uploadingImage" :disabled="submitting" @click="imageInput?.click()">
              上传图片并插入正文
            </NButton>
            <NButton v-if="uploadedImage" attr-type="button" size="small" secondary :disabled="uploadingImage || submitting" @click="setCover">
              设为封面
            </NButton>
            <span
              v-if="mediaNotice"
              data-testid="media-notice"
              class="text-xs"
              :class="mediaNotice.kind === 'ok' ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'"
            >{{ mediaNotice.text }}</span>
          </div>
          <AdminMarkdownEditor ref="editorRef" v-model="form.content_md" />
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
              <NButton v-if="form.cover_url" attr-type="button" size="tiny" quaternary type="error" @click="form.cover_url = ''">
                清除封面
              </NButton>
            </div>
          </div>
        </NFormItem>
        <NFormItem label="首页展示" path="featured" :show-feedback="false">
          <NCheckbox v-model:checked="form.featured">
            首页精选
          </NCheckbox>
        </NFormItem>
      </div>
    </NForm>

    <div class="flex flex-wrap items-center gap-6">
      <!-- CreateArticle always starts as DRAFT server-side, so the status
           choice only makes sense when editing an existing article. -->
      <NRadioGroup v-if="props.initial" v-model:value="form.status">
        <NRadio :value="1">
          草稿
        </NRadio>
        <NRadio :value="2">
          发布
        </NRadio>
      </NRadioGroup>
      <slot name="actions" :submit="submit" :form="form" />
    </div>
  </form>
</template>
