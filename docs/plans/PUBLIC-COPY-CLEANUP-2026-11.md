# 公开页面文案精简

2026-11-03。用户要求删除截图中的文章副文案和英文眉题，并检查全站同类表达。状态：completed（本地）；属于已批准视觉改动的局部文案修订。

## 清理计划

1. 删除文章/作品英文眉题、口号式副标题和重复导览说明。系列入口保留真实主题、篇数、链接；推荐文章保留，标题直接说明内容。
2. 首页/关于页将“探索、构建、过程与取舍”等泛化口号改为作者职业、技术方向、项目功能等已确认事实。空态使用“暂无文章/项目”，错误/权限/操作提示保持准确。
3. 删除本轮已无引用的CSS；不增加抽象、依赖或文案配置层。封面、配色、文章/作品真实数据、导航/搜索/收藏/展开、首页城市和关于背景沿用上一轮。

不改CMS正文、数据、后台/小程序或生产；图片中的标题属于已批准素材，不因这次两条页面文字截图擅自重画。开发预览/历史实验文案只在发现仍是公开入口时处理。

## 行为锁与回退分类

修改前运行现有系列、展开、封面、主题和首页动效测试，锁定导航/失效回退与动效行为。此次只改文案和展示层，不新增逐字匹配实现的单元测试；以浏览器中的文案消失、真实链接/计数/交互仍可用作为替代证据。

空状态、图片失败、权限/网络错误属于有依据的状态或fail-safe边界，保留条件和恢复入口；仅修改过度修饰的空状态措辞。没有需要移除的masking fallback路径。

主代理编写方案及修改，独立reviewer审核范围与结果，保持writer/reviewer分离。验收：前端单测、lint/typecheck/build、桌面/手机明暗浏览器和首页/关于回归；确认两张截图的文字不再出现，未留下空包装或悬空样式。

## 结果

已完成，无远端或生产操作，分支仍为 codex/blog-visual-hierarchy。原有未提交改动保留。

| 文件/范围 | 简化 |
|---|---|
| pages/posts/index.vue、pages/projects/index.vue | 删除英文眉题、口号副标题；文章页只保留真实系列名/篇数链接 |
| pages/series/index.vue、pages/series/[slug].vue、data/articleSeries.ts | 删除重复导览，标题改为“目录/推荐文章”，说明改为实际主题 |
| pages/index.vue、pages/about.vue | 用姓名、职业方向、项目功能和联系方式替代泛化口号；原动效DOM和链接保留 |
| pages/tags/、pages/categories/ | 删除标题下重复的浏览说明 |
| ArticleCollection.vue、WechatFollow.vue、projects/[slug].vue、utils/seo.ts | 简化空态/公众号/SEO文案，移除无引用description插槽 |
| assets/css/main.css及页面scoped CSS | 删除相应无引用样式，保留已批准色彩/卡片布局 |

回退分类：空态、网络错误、权限和图片回退仍为有依据的兼容/失败边界，判断条件和恢复操作不变；无masking fallback变更。历史预览、动效参考素材、封面内文字与CMS内容不在本轮改写范围。

验证：修改前39项定向行为锁通过（output/copy-cleanup-baseline.log）；最终93单测、lint、typecheck、生产build均通过，日志在output/copy-cleanup/。浏览器completed=true，54/54组页面/视口/主题检查与交互/异常专项通过，errors=[]；覆盖截图文字消失、首页/关于实际信息、分类/标签、封面、展开/焦点/修饰键/减少动态、错误与空态。报告/截图位于output/visual-hierarchy/e2e/，上一轮40组报告已另存output/copy-cleanup/previous-visual-report.json。任务自有fixture/Nitro/浏览器已清理。

独立reviewer对计划和结果均APPROVE，未参与写入；修改文件ESLint及git diff --check通过。没有未解决阻断；手机为浏览器模拟，本轮没有生产或真机验收。
