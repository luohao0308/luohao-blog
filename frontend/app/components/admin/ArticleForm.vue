<script setup lang="ts">
// Shared create/edit form for admin articles. The page owns the API call;
// this component owns field state and client-side validation. Mirrors the
// backend rules: slug is URL-safe lowercase and immutable after creation.
import { NButton, NCheckbox, NDynamicTags, NForm, NFormItem, NInput, NRadio, NRadioGroup, NSelect } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { ARTICLE_STATUS, type Article } from '~/composables/useArticles'
import { ADMIN_MEDIA_MAX_SOURCE_BYTES, ADMIN_MEDIA_TYPES, adminMediaError, useAdminMedia } from '~/composables/useAdminMedia'
import { useAdminCategories } from '~/composables/useCategories'

const props = defineProps<{
  initial?: Article | null
  submitting?: boolean
}>()

const emit = defineEmits<{
  submit: [payload: { title: string, slug: string, summary: string, content_md: string, tags: string[], category_slug: string | null, status: number, cover_url: string, featured: boolean }]
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

async function chooseImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || uploadingImage.value)
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
  if (!uploadedImage.value)
    return
  form.cover_url = uploadedImage.value
  mediaNotice.value = { kind: 'ok', text: '已设为文章封面' }
}
</script>

<template>
  <NForm ref="formRef" class="space-y-6" :model="form" :rules="rules" label-placement="top" :show-require-mark="false" @submit.prevent="submit">
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
  </NForm>
</template>
