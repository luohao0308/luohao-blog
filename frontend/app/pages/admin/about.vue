<script setup lang="ts">
import { NButton, NDynamicTags, NFormItem, NInput, useMessage } from 'naive-ui'
import type { AboutProfile } from '~/composables/usePortfolio'
import { ADMIN_MEDIA_MAX_SOURCE_BYTES, ADMIN_MEDIA_TYPES, adminMediaError, useAdminMedia } from '~/composables/useAdminMedia'

definePageMeta({ layout: 'admin', middleware: 'admin-auth' })
useHead({ title: '关于页管理' })
const message = useMessage()
const { getAbout, updateAbout } = useAdminPortfolio()
const { uploadImage } = useAdminMedia()
const profile = ref<AboutProfile | null>(null)
const loading = ref(false)
const loadFailed = ref(false)
const submitting = ref(false)
const uploading = ref(false)
const imageInput = ref<HTMLInputElement | null>(null)

async function load() {
  loading.value = true
  loadFailed.value = false
  try { profile.value = structuredClone(await getAbout()) }
  catch { loadFailed.value = true }
  finally { loading.value = false }
}
onMounted(load)

async function chooseImage(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file || !profile.value) return
  if (!ADMIN_MEDIA_TYPES.includes(file.type) || file.size > ADMIN_MEDIA_MAX_SOURCE_BYTES) {
    message.error('仅支持不超过 2MB 的 jpg/png/webp 图片')
    return
  }
  uploading.value = true
  try {
    profile.value.profile_image = await uploadImage(file)
    message.success('形象图片已上传')
  }
  catch (err) { message.error(adminMediaError(err)) }
  finally { uploading.value = false }
}

function linksValid(value: AboutProfile): boolean {
  return value.contacts.every(item => Boolean(safePortfolioHref(item.href, true)))
    && value.representative_links.every(item => Boolean(safePortfolioHref(item.href)))
}

async function save() {
  if (!profile.value || submitting.value) return
  if (!linksValid(profile.value)) {
    message.error('联系链接仅支持 http/https 或简单 mailto，代表链接仅支持 http/https')
    return
  }
  submitting.value = true
  try {
    profile.value = structuredClone(await updateAbout(profile.value))
    message.success('关于页已保存')
  }
  catch { message.error('关于页保存失败') }
  finally { submitting.value = false }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3"><h1 class="text-xl font-semibold">关于页管理</h1><NButton type="primary" :loading="submitting" :disabled="!profile" @click="save">保存</NButton></div>
    <p v-if="loading" role="status" class="py-10 text-center text-sm text-slate-500">正在加载…</p>
    <div v-else-if="loadFailed" role="alert" class="flex items-center justify-between rounded-xl border border-red-200 p-4 text-sm text-red-600"><span>关于页加载失败。</span><NButton size="small" type="error" secondary @click="load">重新加载</NButton></div>
    <div v-else-if="profile" class="space-y-8">
      <section class="grid gap-5 rounded-xl border border-slate-200 p-5 sm:grid-cols-2 dark:border-slate-800">
        <NFormItem label="名称"><NInput v-model:value="profile.name" /></NFormItem>
        <NFormItem label="一句话介绍"><NInput v-model:value="profile.headline" /></NFormItem>
        <NFormItem label="详细简介" class="sm:col-span-2"><NInput v-model:value="profile.intro" type="textarea" :rows="5" /></NFormItem>
        <NFormItem label="当前方向" class="sm:col-span-2"><NInput v-model:value="profile.current_focus" type="textarea" :rows="3" /></NFormItem>
        <NFormItem label="形象图片" class="sm:col-span-2">
          <div class="space-y-3">
            <input ref="imageInput" data-testid="about-image-input" class="sr-only" type="file" accept="image/jpeg,image/png,image/webp" @change="chooseImage">
            <NButton attr-type="button" size="small" :loading="uploading" @click="imageInput?.click()">上传图片</NButton>
            <img v-if="profile.profile_image" :src="assetUrl(profile.profile_image)" alt="关于页形象预览" class="h-40 w-40 rounded-xl object-cover">
          </div>
        </NFormItem>
      </section>

      <section class="space-y-4">
        <div class="flex items-center justify-between"><h2 class="font-medium">技能组</h2><NButton size="small" @click="profile.skills.push({ group: '', items: [] })">新增技能组</NButton></div>
        <div v-for="(group, index) in profile.skills" :key="index" class="grid gap-3 rounded-xl border border-slate-200 p-4 sm:grid-cols-[12rem_minmax(0,1fr)_auto] dark:border-slate-800">
          <NInput v-model:value="group.group" placeholder="分组名称" /><NDynamicTags v-model:value="group.items" /><NButton quaternary type="error" @click="profile.skills.splice(index, 1)">移除</NButton>
        </div>
      </section>

      <section class="space-y-4">
        <div class="flex items-center justify-between"><h2 class="font-medium">经历</h2><NButton size="small" @click="profile.timeline.push({ year: '', text: '' })">新增经历</NButton></div>
        <div v-for="(item, index) in profile.timeline" :key="index" class="grid gap-3 rounded-xl border border-slate-200 p-4 sm:grid-cols-[8rem_minmax(0,1fr)_auto] dark:border-slate-800">
          <NInput v-model:value="item.year" placeholder="年份" /><NInput v-model:value="item.text" type="textarea" :rows="2" placeholder="只填写已核实的经历" /><NButton quaternary type="error" @click="profile.timeline.splice(index, 1)">移除</NButton>
        </div>
      </section>

      <section class="space-y-4">
        <div class="flex items-center justify-between"><h2 class="font-medium">联系方式</h2><NButton size="small" @click="profile.contacts.push({ label: '', href: '', text: '' })">新增联系方式</NButton></div>
        <div v-for="(item, index) in profile.contacts" :key="index" class="grid gap-3 rounded-xl border border-slate-200 p-4 sm:grid-cols-[9rem_minmax(0,1fr)_minmax(0,1fr)_auto] dark:border-slate-800">
          <NInput v-model:value="item.label" placeholder="名称" /><NInput v-model:value="item.href" placeholder="https:// 或 mailto:" /><NInput v-model:value="item.text" placeholder="显示文字" /><NButton quaternary type="error" @click="profile.contacts.splice(index, 1)">移除</NButton>
        </div>
      </section>

      <section class="space-y-4">
        <div class="flex items-center justify-between"><h2 class="font-medium">补充代表链接</h2><NButton size="small" @click="profile.representative_links.push({ label: '', href: '' })">新增链接</NButton></div>
        <div v-for="(item, index) in profile.representative_links" :key="index" class="grid gap-3 rounded-xl border border-slate-200 p-4 sm:grid-cols-[12rem_minmax(0,1fr)_auto] dark:border-slate-800">
          <NInput v-model:value="item.label" placeholder="名称" /><NInput v-model:value="item.href" placeholder="https://" /><NButton quaternary type="error" @click="profile.representative_links.splice(index, 1)">移除</NButton>
        </div>
      </section>
    </div>
  </div>
</template>
