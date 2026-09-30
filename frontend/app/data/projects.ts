// Portfolio placeholder data. P4 replaces this with the backend projects
// module; the page code only depends on these types.

export interface Project {
  slug: string
  name: string
  summary: string
  description: string
  stack: string[]
  role: string
  repoUrl?: string
  liveUrl?: string
  emoji: string
  year: string
  featured: boolean
}

export const projects: Project[] = [
  {
    slug: 'luohao-blog',
    name: 'luohao-blog',
    summary: '本站：Go + Kratos 后端、Nuxt 4 SSR 前端的全栈个人站',
    description:
      '单体仓库全栈项目：Kratos 承载 REST/gRPC 契约与文章渲染管道（goldmark + chroma），Nuxt 4 负责 SSR 与 BFF 代理；开发全流程由自研 dev-workflow 治理（fail-closed 交付门禁、版本化迁移、契约索引）。本站的所有功能迭代都以文章形式记录在博客里。',
    stack: ['Go', 'Kratos', 'ent', 'Nuxt 4', 'Tailwind CSS', 'MySQL', 'Redis', 'Elasticsearch'],
    role: '独立设计与开发',
    repoUrl: 'https://github.com/luohao0308/luohao-blog',
    emoji: '🛠️',
    year: '2026',
    featured: true,
  },
  {
    slug: 'dev-workflow',
    name: 'dev-workflow',
    summary: '可分发的 AI 协作开发流程：把大厂工程治理装进任意仓库',
    description:
      '一套安装到任意代码仓库的 AI 协作开发流程：项目画像、契约治理、交付门禁（push/PR/merge 全部 fail-closed 校验）、任务上下文与工作日志。本站从建仓到每个 PR 的合并都由它把守。',
    stack: ['Bash', 'Python', 'PowerShell', 'AI Workflow'],
    role: '作者与维护者',
    repoUrl: 'https://github.com/luohao0308/dev-workflow',
    emoji: '⚙️',
    year: '2026',
    featured: true,
  },
  {
    slug: 'job-application-workbench',
    name: 'job-application-workbench',
    summary: '求职投递工作台：岗位聚合、匹配度排序与投递状态管理',
    description:
      'Vue + Node 全栈工具：聚合多渠道岗位、按个人画像做匹配度排序、跟踪投递状态流转。M1 之前的主要练手项目，vue-workflow 治理模式的发源地。',
    stack: ['Vue 3', 'Node.js', 'Vite'],
    role: '独立开发',
    emoji: '📋',
    year: '2025',
    featured: false,
  },
]

export function getProject(slug: string): Project | undefined {
  return projects.find((p) => p.slug === slug)
}
