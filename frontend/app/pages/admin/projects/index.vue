<script setup lang="ts">
import { NButton, NDataTable, NTag, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import type { Project } from '~/composables/usePortfolio'

definePageMeta({ layout: 'admin', middleware: 'admin-auth' })
useHead({ title: '项目管理' })
const message = useMessage()
const { listProjects, updateProject, unpublishProject } = useAdminPortfolio()
const projects = ref<Project[]>([])
const loading = ref(false)
const loadFailed = ref(false)
const busySlug = ref('')

async function load() {
  loading.value = true
  loadFailed.value = false
  try {
    projects.value = await listProjects()
  }
  catch {
    loadFailed.value = true
    message.error('项目列表加载失败')
  }
  finally {
    loading.value = false
  }
}

async function togglePublished(project: Project) {
  if (busySlug.value) return
  busySlug.value = project.slug
  try {
    if (project.published) await unpublishProject(project.slug)
    else await updateProject({ ...project, published: true })
    message.success(project.published ? '已下架' : '已上架')
    await load()
  }
  catch {
    message.error('项目状态更新失败')
  }
  finally {
    busySlug.value = ''
  }
}

onMounted(load)

const columns: DataTableColumns<Project> = [
  { title: '排序', key: 'sort', width: 70 },
  { title: '项目', key: 'name', render: row => h('div', {}, [h('strong', {}, row.name), h('p', { class: 'text-xs text-slate-500' }, row.slug)]) },
  { title: '年份', key: 'year', width: 80 },
  { title: '状态', key: 'published', width: 90, render: row => h(NTag, { type: row.published ? 'success' : 'warning', bordered: false, size: 'small' }, { default: () => row.published ? '已上架' : '未上架' }) },
  { title: '精选', key: 'featured', width: 70, render: row => row.featured ? '是' : '否' },
  {
    title: '操作', key: 'actions', width: 170,
    render: row => h('div', { class: 'flex gap-1' }, [
      h(NButton, { size: 'tiny', quaternary: true, type: 'primary', onClick: () => navigateTo(`/admin/projects/${encodeURIComponent(row.slug)}/edit`) }, { default: () => '编辑' }),
      h(NButton, { size: 'tiny', quaternary: true, type: row.published ? 'warning' : 'success', disabled: Boolean(busySlug.value), onClick: () => togglePublished(row) }, { default: () => row.published ? '下架' : '上架' }),
    ]),
  },
]
</script>

<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-xl font-semibold">项目管理</h1>
      <NButton type="primary" @click="navigateTo('/admin/projects/new')">新增项目</NButton>
    </div>
    <div v-if="loadFailed" role="alert" class="flex items-center justify-between rounded-xl border border-red-200 p-4 text-sm text-red-600 dark:border-red-900 dark:text-red-300">
      <span>项目列表暂时无法加载。</span><NButton size="small" type="error" secondary @click="load">重新加载</NButton>
    </div>
    <NDataTable :columns="columns" :data="projects" :loading="loading" :bordered="false" :row-key="(row: Project) => row.slug" />
    <p v-if="!loading && !loadFailed && !projects.length" class="py-12 text-center text-sm text-slate-500">还没有项目，可先新增一项。</p>
  </div>
</template>
