<script setup lang="ts">
import { NButton, useMessage } from 'naive-ui'
import type { Project } from '~/composables/usePortfolio'
definePageMeta({ layout: 'admin', middleware: 'admin-auth' })
const route = useRoute()
const slug = computed(() => String(route.params.slug))
const message = useMessage()
const project = ref<Project | null>(null)
const loadFailed = ref(false)
const submitting = ref(false)
const { getProject, updateProject } = useAdminPortfolio()
useHead({ title: computed(() => project.value ? `编辑：${project.value.name}` : '编辑项目') })
async function load() {
  loadFailed.value = false
  try { project.value = await getProject(slug.value) }
  catch { loadFailed.value = true }
}
onMounted(load)
async function submit(value: Project) {
  submitting.value = true
  try {
    project.value = await updateProject({ ...value, slug: slug.value })
    message.success('项目已保存')
  }
  catch { message.error('保存失败，请检查链接和字段') }
  finally { submitting.value = false }
}
</script>
<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between"><h1 class="text-xl font-semibold">编辑项目</h1><NuxtLink to="/admin/projects" class="text-sm text-slate-500 hover:underline">返回列表</NuxtLink></div>
    <div v-if="loadFailed" role="alert" class="flex items-center justify-between rounded-xl border border-red-200 p-4 text-sm text-red-600"><span>项目加载失败。</span><NButton size="small" type="error" secondary @click="load">重新加载</NButton></div>
    <AdminProjectForm v-if="project" :key="project.updated_at ? String(project.updated_at) : project.slug" :initial="project" :submitting="submitting" @submit="submit"><template #actions="{ submit: submitForm }"><NButton type="primary" attr-type="button" :loading="submitting" @click="submitForm">保存</NButton></template></AdminProjectForm>
  </div>
</template>
