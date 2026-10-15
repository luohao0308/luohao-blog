<script setup lang="ts">
import { NButton, NCheckbox, NDynamicTags, NForm, NFormItem, NInput, NInputNumber } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { ADMIN_MEDIA_MAX_SOURCE_BYTES, ADMIN_MEDIA_TYPES, adminMediaError, useAdminMedia } from '~/composables/useAdminMedia'
import type { Project } from '~/composables/usePortfolio'

const props = defineProps<{ initial?: Project | null, submitting?: boolean }>()
const emit = defineEmits<{ submit: [project: Project] }>()
const { uploadImage } = useAdminMedia()
const formRef = ref<FormInst | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const mediaNotice = ref<{ kind: 'ok' | 'error', text: string } | null>(null)

const form = reactive<Project>({
  slug: props.initial?.slug ?? '',
  name: props.initial?.name ?? '',
  summary: props.initial?.summary ?? '',
  description: props.initial?.description ?? '',
  stack: [...(props.initial?.stack ?? [])],
  role: props.initial?.role ?? '',
  repo_url: props.initial?.repo_url ?? '',
  live_url: props.initial?.live_url ?? '',
  emoji: props.initial?.emoji ?? '🧩',
  year: props.initial?.year ?? String(new Date().getFullYear()),
  featured: props.initial?.featured ?? false,
  cover_url: props.initial?.cover_url ?? '',
  screenshots: [...(props.initial?.screenshots ?? [])],
  problem: props.initial?.problem ?? '',
  approach: props.initial?.approach ?? '',
  outcome: props.initial?.outcome ?? '',
  published: props.initial?.published ?? false,
  sort: props.initial?.sort ?? 100,
})

const validHTTP = (_rule: unknown, value: string) => !value || Boolean(safePortfolioHref(value))
const rules: FormRules = {
  slug: [
    { required: true, message: '请输入 slug', trigger: 'blur' },
    { pattern: /^[a-z0-9]+(?:-[a-z0-9]+)*$/, message: '仅允许小写字母、数字和单连字符', trigger: 'blur' },
  ],
  name: { required: true, message: '请输入项目名称', trigger: 'blur' },
  summary: { required: true, message: '请输入项目摘要', trigger: 'blur' },
  repo_url: { validator: validHTTP, message: '源码链接必须是 http/https 地址', trigger: 'blur' },
  live_url: { validator: validHTTP, message: '在线链接必须是 http/https 地址', trigger: 'blur' },
}

async function chooseImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || uploading.value) return
  if (!ADMIN_MEDIA_TYPES.includes(file.type)) {
    mediaNotice.value = { kind: 'error', text: '仅支持 jpg、png 或 webp 图片' }
    return
  }
  if (file.size > ADMIN_MEDIA_MAX_SOURCE_BYTES) {
    mediaNotice.value = { kind: 'error', text: '原始图片大小不能超过 2MB' }
    return
  }
  uploading.value = true
  mediaNotice.value = null
  try {
    const url = await uploadImage(file)
    if (!form.screenshots?.includes(url)) form.screenshots?.push(url)
    mediaNotice.value = { kind: 'ok', text: '截图已上传' }
  }
  catch (err) {
    const invalidResponse = err instanceof Error && err.message === 'invalid media response'
    mediaNotice.value = { kind: 'error', text: invalidResponse ? '上传结果无效，请重试' : adminMediaError(err) }
  }
  finally {
    uploading.value = false
  }
}

function submit(event?: Event) {
  event?.preventDefault()
  formRef.value?.validate((errors) => {
    if (errors) return
    emit('submit', {
      ...form,
      slug: form.slug.trim(),
      name: form.name.trim(),
      summary: form.summary.trim(),
      stack: form.stack.map(item => item.trim()).filter(Boolean),
      screenshots: [...(form.screenshots ?? [])],
      repo_url: form.repo_url?.trim(),
      live_url: form.live_url?.trim(),
    })
  })
}
</script>

<template>
  <NForm ref="formRef" class="space-y-6" :model="form" :rules="rules" label-placement="top" :show-require-mark="false" @submit.prevent="submit">
      <div class="grid gap-x-6 sm:grid-cols-2">
        <NFormItem label="项目名称" path="name" :show-feedback="false"><NInput v-model:value="form.name" /></NFormItem>
        <NFormItem label="Slug（创建后不可修改）" path="slug" :show-feedback="false"><NInput v-model:value="form.slug" :disabled="!!initial" placeholder="project-slug" /></NFormItem>
      </div>
      <NFormItem label="摘要" path="summary"><NInput v-model:value="form.summary" type="textarea" :rows="2" /></NFormItem>
      <NFormItem label="项目说明" path="description"><NInput v-model:value="form.description" type="textarea" :rows="5" /></NFormItem>
      <div class="grid gap-x-6 sm:grid-cols-2">
        <NFormItem label="角色" path="role"><NInput v-model:value="form.role" /></NFormItem>
        <NFormItem label="技术栈" path="stack"><NDynamicTags v-model:value="form.stack" /></NFormItem>
        <NFormItem label="源码链接" path="repo_url"><NInput v-model:value="form.repo_url" placeholder="https://" /></NFormItem>
        <NFormItem label="在线链接" path="live_url"><NInput v-model:value="form.live_url" placeholder="https://" /></NFormItem>
        <NFormItem label="年份" path="year"><NInput v-model:value="form.year" /></NFormItem>
        <NFormItem label="图标" path="emoji"><NInput v-model:value="form.emoji" maxlength="8" /></NFormItem>
        <NFormItem label="排序（越小越靠前）" path="sort"><NInputNumber v-model:value="form.sort" :min="0" class="w-full" /></NFormItem>
      </div>
      <div class="grid gap-5 sm:grid-cols-3">
        <NFormItem label="问题" path="problem"><NInput v-model:value="form.problem" type="textarea" :rows="4" /></NFormItem>
        <NFormItem label="方案" path="approach"><NInput v-model:value="form.approach" type="textarea" :rows="4" /></NFormItem>
        <NFormItem label="结果" path="outcome"><NInput v-model:value="form.outcome" type="textarea" :rows="4" /></NFormItem>
      </div>
      <NFormItem label="截图与封面" path="screenshots">
        <div class="w-full space-y-3">
          <input ref="fileInput" data-testid="project-image-input" type="file" class="sr-only" accept="image/jpeg,image/png,image/webp" @change="chooseImage">
          <div class="flex flex-wrap items-center gap-2">
            <NButton attr-type="button" size="small" :loading="uploading" :disabled="submitting" @click="fileInput?.click()">上传截图</NButton>
            <span v-if="mediaNotice" data-testid="project-media-notice" class="text-xs" :class="mediaNotice.kind === 'ok' ? 'text-emerald-600' : 'text-red-600'">{{ mediaNotice.text }}</span>
          </div>
          <div v-if="form.screenshots?.length" class="grid gap-3 sm:grid-cols-2">
            <div v-for="image in form.screenshots" :key="image" class="space-y-2 rounded-xl border border-slate-200 p-2 dark:border-slate-700">
              <img :src="assetUrl(image)" alt="项目截图预览" class="aspect-[16/9] w-full rounded-lg object-cover">
              <div class="flex flex-wrap gap-2">
                <NButton attr-type="button" size="tiny" secondary @click="form.cover_url = image">设为封面</NButton>
                <NButton attr-type="button" size="tiny" quaternary type="error" @click="form.screenshots = form.screenshots?.filter(item => item !== image)">移除</NButton>
              </div>
            </div>
          </div>
          <p v-else class="text-xs text-slate-500">暂无截图；公开页会明确显示资料正在整理，不生成示例图片。</p>
          <img v-if="form.cover_url" :src="assetUrl(form.cover_url)" alt="项目封面预览" class="max-h-56 rounded-xl border border-slate-200 object-cover dark:border-slate-700">
        </div>
      </NFormItem>
      <div class="flex flex-wrap gap-6">
        <NCheckbox v-model:checked="form.featured">精选项目</NCheckbox>
        <NCheckbox v-model:checked="form.published">公开上架</NCheckbox>
      </div>
    <slot name="actions" :submit="submit" />
  </NForm>
</template>
