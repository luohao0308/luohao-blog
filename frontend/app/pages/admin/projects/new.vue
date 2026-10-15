<script setup lang="ts">
import { NButton, useMessage } from 'naive-ui'
import type { Project } from '~/composables/usePortfolio'
definePageMeta({ layout: 'admin', middleware: 'admin-auth' })
useHead({ title: '新增项目' })
const message = useMessage()
const submitting = ref(false)
const { createProject } = useAdminPortfolio()
async function submit(project: Project) {
  submitting.value = true
  try {
    const created = await createProject(project)
    message.success('项目已创建')
    await navigateTo(`/admin/projects/${encodeURIComponent(created.slug)}/edit`)
  }
  catch { message.error('创建失败，请检查 slug 与链接') }
  finally { submitting.value = false }
}
</script>
<template>
  <div class="space-y-6">
    <div class="flex items-center justify-between"><h1 class="text-xl font-semibold">新增项目</h1><NuxtLink to="/admin/projects" class="text-sm text-slate-500 hover:underline">返回列表</NuxtLink></div>
    <AdminProjectForm :submitting="submitting" @submit="submit"><template #actions="{ submit: submitForm }"><NButton type="primary" attr-type="button" :loading="submitting" @click="submitForm">创建</NButton></template></AdminProjectForm>
  </div>
</template>
