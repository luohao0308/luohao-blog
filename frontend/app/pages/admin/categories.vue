<script setup lang="ts">
// Category management: a flat, admin-curated taxonomy. Create/rename go
// through CreateCategory / UpdateCategory with a name,sort field mask (slug
// is immutable); deleting detaches the referencing articles server-side.
import { NButton, NDataTable, NForm, NFormItem, NInput, NInputNumber, NModal, NPopconfirm, useMessage } from 'naive-ui'
import type { DataTableColumns, FormInst, FormRules } from 'naive-ui'

import type { Category } from '~/composables/useCategories'
import { formatDate } from '~/composables/useArticles'

definePageMeta({ layout: 'admin', middleware: 'admin-auth' })

useHead({ title: '分类管理' })

const { authFetch } = useAuth()
const message = useMessage()
const { categories, loading, load } = useAdminCategories()

onMounted(load)

const modalVisible = ref(false)
const editing = ref<Category | null>(null)
const submitting = ref(false)

const form = reactive({ slug: '', name: '', sort: 0 })
const formRef = ref<FormInst | null>(null)

const rules: FormRules = {
  slug: [
    { required: true, message: '请输入 slug', trigger: 'blur' },
    { pattern: /^[a-z0-9]+(-[a-z0-9]+)*$/, message: '仅允许小写字母、数字和连字符', trigger: 'blur' },
  ],
  name: { required: true, message: '请输入名称', trigger: 'blur' },
}

function openCreate() {
  editing.value = null
  form.slug = ''
  form.name = ''
  form.sort = 0
  modalVisible.value = true
}

function openEdit(category: Category) {
  editing.value = category
  form.slug = category.slug
  form.name = category.name
  form.sort = category.sort
  modalVisible.value = true
}

async function submit() {
  formRef.value?.validate(async (errors) => {
    if (errors)
      return
    submitting.value = true
    try {
      if (editing.value) {
        await authFetch('/api/v1/categories/update', {
          method: 'PUT',
          query: { update_mask: 'name,sort' },
          body: { slug: editing.value.slug, name: form.name, sort: form.sort },
        })
        message.success('已保存')
      }
      else {
        await authFetch('/api/v1/categories/create', {
          method: 'POST',
          body: { slug: form.slug, name: form.name, sort: form.sort },
        })
        message.success('已创建')
      }
      modalVisible.value = false
      await load()
    }
    catch {
      message.error(editing.value ? '保存失败，slug 可能不存在' : '创建失败，slug 可能已占用')
    }
    finally {
      submitting.value = false
    }
  })
}

async function removeCategory(category: Category) {
  try {
    await authFetch(`/api/v1/categories/${encodeURIComponent(category.slug)}`, { method: 'DELETE' })
    message.success('已删除，相关文章已回到未分类')
  }
  catch {
    message.error('删除失败')
  }
  load()
}

const columns: DataTableColumns<Category> = [
  {
    title: '名称',
    key: 'name',
    render: row => h('span', { class: 'font-medium' }, { default: () => row.name }),
  },
  {
    title: 'Slug',
    key: 'slug',
    render: row => h('code', { class: 'text-xs text-slate-500 dark:text-slate-400' }, { default: () => row.slug }),
  },
  { title: '排序', key: 'sort', width: 80 },
  {
    title: '文章数',
    key: 'article_count',
    width: 90,
    render: row => String(row.article_count ?? 0),
  },
  {
    title: '更新时间',
    key: 'updated_at',
    width: 120,
    render: row => formatDate(row.updated_at),
  },
  {
    title: '操作',
    key: 'actions',
    width: 160,
    render: row => h('div', { class: 'flex items-center gap-1' }, {
      default: () => [
        h(NButton, {
          size: 'tiny',
          quaternary: true,
          type: 'primary',
          onClick: () => openEdit(row),
        }, { default: () => '编辑' }),
        h(NPopconfirm, {
          onPositiveClick: () => removeCategory(row),
        }, {
          trigger: () => h(NButton, { size: 'tiny', quaternary: true, type: 'error' }, { default: () => '删除' }),
          default: () => '删除后引用它的文章会回到未分类，确定删除吗？',
        }),
      ],
    }),
  },
]
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-xl font-semibold">
        分类管理
      </h1>
      <NButton type="primary" @click="openCreate">
        新建分类
      </NButton>
    </div>
    <p class="text-sm text-slate-500 dark:text-slate-400">
      分类是后台维护的一层目录；与标签不同，每篇文章最多属于一个分类，也可以不属于任何分类。
    </p>

    <NDataTable
      :columns="columns"
      :data="categories"
      :loading="loading"
      :bordered="false"
      size="small"
      :row-key="(row: Category) => row.slug"
    />

    <NModal
      v-model:show="modalVisible"
      preset="card"
      class="w-[26rem]"
      :title="editing ? `编辑分类：${editing.name}` : '新建分类'"
    >
      <NForm ref="formRef" :model="form" :rules="rules" label-placement="top" :show-require-mark="false">
        <NFormItem label="Slug（创建后不可修改）" path="slug" :show-feedback="false">
          <NInput v-model:value="form.slug" :disabled="!!editing" placeholder="url-safe-slug" />
        </NFormItem>
        <NFormItem label="名称" path="name" :show-feedback="false">
          <NInput v-model:value="form.name" placeholder="展示名称，如：工程实践" />
        </NFormItem>
        <NFormItem label="排序（小的在前）" path="sort" :show-feedback="false">
          <NInputNumber v-model:value="form.sort" class="w-full" :min="0" />
        </NFormItem>
      </NForm>
      <template #footer>
        <div class="flex justify-end gap-3">
          <NButton @click="modalVisible = false">
            取消
          </NButton>
          <NButton type="primary" :loading="submitting" @click="submit">
            {{ editing ? '保存' : '创建' }}
          </NButton>
        </div>
      </template>
    </NModal>
  </div>
</template>
