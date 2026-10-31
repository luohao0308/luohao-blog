export interface EditorialImage {
  src: string
  alt: string
  caption: string
  credit: string
  seoSrc?: string
  sourceUrl?: string
  diagram?: boolean
  poster?: boolean
}

const workflow: EditorialImage = {
  src: '/images/editorial/covers/workflow.webp',
  alt: '换个 AI，别从头讲。把项目规则留在仓库里。',
  caption: '', credit: '', poster: true,
}
const guard: EditorialImage = {
  src: '/images/editorial/covers/guard.webp',
  alt: 'AI 写完了，谁敢合并？。权限、版本与 CI 证据。',
  caption: '', credit: '', poster: true,
}
const harness: EditorialImage = {
  src: '/images/editorial/covers/harness.webp',
  alt: 'Agent，不止聊天。工具、状态和运行记录。',
  caption: '', credit: '', poster: true,
}
const aigc: EditorialImage = {
  src: '/images/editorial/covers/aigc.webp',
  alt: '一句话背后的后端。模型接口怎样接成产品。',
  caption: '', credit: '', poster: true,
}
const fullstack: EditorialImage = {
  src: '/images/editorial/covers/fullstack.webp',
  alt: '登录成功，不等于能用。从登录走到发布。',
  caption: '', credit: '', poster: true,
}
const ent: EditorialImage = {
  src: '/images/editorial/covers/ent.webp',
  alt: '标签丢了？先查关系。多对多关系排查。',
  caption: '', credit: '', poster: true,
}
const mysql: EditorialImage = {
  src: '/images/editorial/covers/mysql.webp',
  alt: '191 / 255，到底选谁？。字符串索引，别只抄数字。',
  caption: '', credit: '', poster: true,
}
export const articleImages: Record<string, EditorialImage> = {
  'ai-delivery-guard': guard,
  'fullstack-smoke-pitfalls': fullstack,
  'ent-m2m-edge-case-study': ent,
  'mysql-varchar-191-case-study': mysql,
  'dev-workflow-project': workflow,
  'forge-harness-project': harness,
  'aigc-backend-delivery': aigc,
}

export const projectImages: Record<string, EditorialImage> = {
  'dev-workflow': { src: '/images/projects/generated/dev-workflow-editorial-v2.webp', seoSrc: '/images/projects/generated/dev-workflow-editorial-v2.webp', alt: 'dev-workflow：仓库协作规则、任务记录与交付验证的原创封面', caption: '', credit: '', poster: true },
  'forge-harness': { src: '/images/projects/generated/forge-harness-editorial-v2.webp', seoSrc: '/images/projects/generated/forge-harness-editorial-v2.webp', alt: 'Forge Harness：任务计划、工具执行、RAG 与评测的原创封面', caption: '', credit: '', poster: true },
  'aigc': { src: '/images/projects/generated/aigc-editorial-v2.webp', seoSrc: '/images/projects/generated/aigc-editorial-v2.webp', alt: 'AIGC 聚合与调度中台：陶瓷、箱包与工艺品设计生成封面', caption: '', credit: '', poster: true },
  'luohao-blog': { src: '/images/projects/generated/luohao-blog-editorial-v2.webp', seoSrc: '/images/projects/generated/luohao-blog-editorial-v2.webp', alt: 'luohao.blog：参考本站首页的技术写作与项目作品封面', caption: '', credit: '', poster: true },
  'job-application-workbench': { src: '/images/projects/generated/job-application-workbench-editorial-v2.webp', seoSrc: '/images/projects/generated/job-application-workbench-editorial-v2.webp', alt: '机会工作台：岗位、个人画像与投递记录的原创封面', caption: '', credit: '', poster: true },
}

export const projectStories: Record<string, {
  question: string
  highlights: string[]
  diagram: string
  detailImage: EditorialImage
  references: { title: string, url: string }[]
}> = {
  'dev-workflow': {
    question: 'AI 会写代码之后，谁来守住项目的上下文与交付边界？',
    highlights: ['可安装的协作规则与流程包', '项目知识、任务与文档分工', '绑定版本与权限的交付检查'],
    diagram: 'delivery-flow',
    detailImage: { src: '/images/editorial/workflow-files.svg', alt: 'Core 中的文档与检查入口。', caption: 'Core 中的文档与检查入口。', credit: '', diagram: true },
    references: [{ title: '项目 README：从安装接入到流程包', url: 'https://github.com/luohao0308/dev-workflow' }, { title: 'GitHub 官方：受保护分支', url: 'https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches' }],
  },
  'forge-harness': {
    question: '模型能调用工具之后，一个任务怎样被追踪、暂停与恢复？',
    highlights: ['任务执行与运行轨迹', '工具接入、审批与审计', '知识检索与 Eval 探索'],
    diagram: 'agent-flow',
    detailImage: { src: '/images/editorial/harness-modules.svg', alt: '控制台、服务与桌面端的目录分工。', caption: '控制台、服务与桌面端的目录分工。', credit: '', diagram: true },
    references: [{ title: '项目 README：Agent 运行时与模块入口', url: 'https://github.com/luohao0308/forge-harness' }, { title: 'MCP 官方：架构与核心概念', url: 'https://modelcontextprotocol.io/docs/learn/architecture' }],
  },
  'aigc': {
    question: '用户只输入一句话，后端要做完哪些工作才能交付结果？',
    highlights: ['Python 后端开发', '外部模型 API 与结果适配', '服务部署与上线'],
    diagram: 'aigc-flow',
    detailImage: { src: '/images/editorial/api-adapter.svg', alt: '接口适配说明示例，非公司源码。', caption: '接口适配说明示例，非公司源码。', credit: '', diagram: true },
    references: [{ title: 'Django 官方：请求与响应', url: 'https://docs.djangoproject.com/en/5.2/ref/request-response/' }, { title: 'DRF 官方：序列化与数据表示', url: 'https://www.django-rest-framework.org/api-guide/serializers/' }],
  },
  'luohao-blog': {
    question: '一个个人网站，怎样从能访问走到能持续维护？',
    highlights: ['文章、搜索与账号评论', 'Go / Nuxt 全栈与微信阅读端', '测试、交付与运维实践'],
    diagram: 'web-flow',
    detailImage: { src: '/images/editorial/fieldmask-code.svg', alt: '本站文章更新服务中的代码节选。', caption: '本站文章更新服务中的代码节选。', credit: '', diagram: true },
    references: [{ title: '本站源码与开发说明', url: 'https://github.com/luohao0308/luohao-blog' }, { title: 'Nuxt 官方：文件路由', url: 'https://nuxt.com/docs/4.x/directory-structure/app/pages' }],
  },
  'job-application-workbench': {
    question: '散落的岗位与投递记录，能否整理成一张可持续使用的工作台？',
    highlights: ['岗位机会管理', 'Vue 与 FastAPI 全栈', '迁移、测试与部署边界'],
    diagram: 'workbench-flow',
    detailImage: { src: '/images/editorial/api-adapter.svg', alt: '前后端数据适配的说明示例。', caption: '前后端数据适配的说明示例。', credit: '', diagram: true },
    references: [{ title: 'FastAPI 官方：应用入门', url: 'https://fastapi.tiangolo.com/tutorial/' }, { title: 'Vue 官方：应用开发指南', url: 'https://vuejs.org/guide/introduction.html' }],
  },
}
