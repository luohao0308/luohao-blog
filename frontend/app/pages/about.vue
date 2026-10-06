<script setup lang="ts">
useHead({ title: '关于我' })

const skills = [
  { group: '后端', items: ['Go', 'Kratos', 'ent', 'gRPC / protobuf', 'MySQL', 'Redis', 'Elasticsearch'] },
  { group: '前端', items: ['Vue 3', 'Nuxt 4', 'TypeScript', 'Tailwind CSS', 'Vite'] },
  { group: '工程化', items: ['Docker', 'GitHub Actions', 'golang-migrate', 'Prometheus / Grafana', 'AI 协作开发流程'] },
]

const timeline = [
  { year: '2026', text: '搭建 luohao-blog：全栈个人站 + AI 交付治理实践场，所有迭代过程写成文章。' },
  { year: '2026', text: '开源 dev-workflow：把大厂工程治理（契约/门禁/可观测）做成可安装的 AI 协作流程。' },
  { year: '2025', text: '深耕全栈：Vue / Go 双栈项目、求职工作台等工具型产品。' },
]

const contacts = [
  { label: 'GitHub', href: 'https://github.com/luohao0308', text: 'github.com/luohao0308' },
  { label: '邮箱', href: 'mailto:hello@example.com', text: 'hello@example.com' },
]
</script>

<template>
  <div class="space-y-12">
    <PageHeader
      title="关于我"
      description="全栈工程师 · Go / Vue 双栈 · AI Agent 工程实践者。相信工程治理和自动化流程是个人开发者的杠杆。"
    />

    <section class="space-y-4">
      <h2 class="text-xl font-bold tracking-tight">技能栈</h2>
      <div class="space-y-3">
        <div v-for="g in skills" :key="g.group" class="flex flex-wrap items-baseline gap-2">
          <span class="w-16 shrink-0 text-sm font-medium text-slate-500 dark:text-slate-400">{{ g.group }}</span>
          <span
            v-for="item in g.items"
            :key="item"
            class="rounded-full border border-slate-200 px-2.5 py-0.5 text-sm dark:border-slate-700"
          >{{ item }}</span>
        </div>
      </div>
    </section>

    <section class="space-y-4">
      <h2 class="text-xl font-bold tracking-tight">时间线</h2>
      <ol class="space-y-4 border-l-2 border-slate-200 pl-5 dark:border-slate-700">
        <li v-for="(t, i) in timeline" :key="i" class="relative">
          <span class="absolute -left-[27px] top-1.5 h-3 w-3 rounded-full border-2 border-white bg-blue-600 dark:border-slate-950" />
          <p class="text-sm font-medium text-blue-600 dark:text-blue-400">{{ t.year }}</p>
          <p class="text-slate-700 dark:text-slate-300">{{ t.text }}</p>
        </li>
      </ol>
    </section>

    <section class="space-y-4">
      <h2 class="text-xl font-bold tracking-tight">联系我</h2>
      <ul class="space-y-2">
        <li v-for="c in contacts" :key="c.label" class="text-slate-700 dark:text-slate-300">
          <span class="mr-2 inline-block w-16 text-sm text-slate-500 dark:text-slate-400">{{ c.label }}</span>
          <a :href="c.href" target="_blank" rel="noopener" class="hover:text-blue-600 hover:underline dark:hover:text-blue-400">
            {{ c.text }}
          </a>
        </li>
      </ul>
      <p class="text-sm text-slate-500">
        有机会合作或想交流技术，欢迎邮件——看到都会回。
      </p>
    </section>
  </div>
</template>
