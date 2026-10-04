<script setup lang="ts">
// Shared create/edit form for admin articles. The page owns the API call;
// this component owns field state and client-side validation. Mirrors the
// backend rules: slug is URL-safe lowercase and immutable after creation.
import { NDynamicTags, NForm, NFormItem, NInput, NRadio, NRadioGroup, NSelect } from 'naive-ui'
import type { FormInst, FormRules } from 'naive-ui'

import { ARTICLE_STATUS, type Article } from '~/composables/useArticles'
import { useAdminCategories } from '~/composables/useCategories'

const props = defineProps<{
  initial?: Article | null
  submitting?: boolean
}>()

const emit = defineEmits<{
  submit: [payload: { title: string, slug: string, summary: string, content_md: string, tags: string[], category_slug: string | null, status: number }]
}>()

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
      })
  })
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
          <AdminMarkdownEditor v-model="form.content_md" />
        </div>
      </NFormItem>
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
