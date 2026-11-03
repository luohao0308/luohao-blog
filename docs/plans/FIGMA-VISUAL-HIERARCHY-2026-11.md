# Figma 视觉层级与系列阅读落地

日期：2026-11-03。状态：completed（本地实施与验收）。用户明确批准本轮 Figma 方案并要求“按照这个改”。基线 origin/main `9de9f789`，本地分支 `codex/blog-visual-hierarchy`；原有三份任务/计划文档未提交变更保留，不纳入本轮产品差异。

设计源：[可编辑稿](https://www.figma.com/design/zkDqdpkupLz3DqOrnaJnlA)，文章7:2、系列8:2/8:106、作品10:2、手机11:54/11:106。完整设计上下文仅留本机 output/visual-hierarchy/figma。

## 本轮范围与顺序

本轮是一个可独立验收的前端视觉切片：补齐两篇新文章封面、差异化五个项目封面、专题系列封面/图文目录与复用卡片展开、浅蓝灰页面/白色卡片/蓝色主操作/深色页脚。依次核验素材与回归入口、完成样式及系列接入、再执行完整前端与浏览器验收。

保留真实API内容、搜索/筛选/收藏、后台自定义cover_url优先、SSR、SEO、首页模型城与既有阅读/编辑行为。原型中的示意文章文案、年份、静态计数不替代真实数据。保留暗色、320px安全阅读、键盘/修饰键和reduced-motion。无新依赖、API/数据库改动、小程序发布、生产写入或远端Git操作。

## 验收

- 两篇新文章在CMS cover_url为空时拥有可解码的16:9封面；自定义封面仍优先，图片失败可读。
- 系列只计公开文章，入口有独立专题视觉，目录复用真实ArticleCard；系列卡片与文章/作品共用展开链路，浏览器验证导航、焦点、修饰键与减少动态。
- 五个项目封面分别可识别，原真实项目截图/正文及来源保留。
- 文章/作品/系列在1440/390/320、浅/暗色无横向溢出；主按钮与卡片有可辨识层级，首页及阅读端回归。
- `node --test tests/*.test.cjs`、`pnpm lint`、`pnpm typecheck`、`pnpm build`；隔离fixture浏览器专项及必要旧用例。素材校验非空/实际格式与尺寸，无临时Figma URL进入运行代码。

回退：撤销本切片前端文件与新素材引用即可，不涉及数据或生产。既有未提交文档与其他工作树不变。

## 结果

实施与本地验收完成；未push、创建PR或操作生产。后端、小程序、部署与CI配置无变更。原有三份未提交文档内容保留；仅对TASKS追加本任务状态。

### 实现与修正

- Figma原始资源直接下载到public：两篇案例PNG、三个主打作品PNG、两作品SVG及其PNG分享图、两系列SVG（共11文件/9视觉素材）。SVG保持根尺寸/viewBox，无脚本或外部引用，PNG实际格式/非空/16:9校验通过，无临时Figma URL进入代码。
- 复用原ArticleCard/ProjectCard与主题token，新增SeriesCard并接入共享展开；系列目录与问题推荐只使用真实公开文章。保留完整搜索、收藏、标签、CMS自定义封面、原真实项目截图和首页城市。
- 审查发现的系列li内卡片100%高度越界，已将等高规则限制到直接网格子卡片，并以真实几何断言锁定；自定义封面上的操作文字沿用白字；四张原生图形改为同源SVG，防止详情放大模糊。未重绘已批准素材。

### 新鲜验证

| 检查 | 结果 | 本机证据 |
|---|---|---|
| `node --test tests/*.test.cjs`（frontend） | 93/93、零skip | output/visual-hierarchy/unit-tests.log |
| `pnpm lint` / `pnpm typecheck` / `pnpm build` | 全部退出0；最终构建包含推荐入口与SVG | output/visual-hierarchy/{lint,typecheck,build}.log |
| `node scripts/e2e/visual_hierarchy_e2e.cjs` | completed=true；40/40，errors=[]，无failure记录；5类交互/异常专项通过 | output/visual-hierarchy/e2e/results.json、screenshots/ |
| `node scripts/e2e/article_series_e2e.cjs` | 桌面/手机目录、公开过滤、sitemap、前后篇、失败重试、未知404通过 | output/series/ |
| 定向系列/封面/SEO单测 | 8/8 | 对应tests/*.test.cjs |
| 独立代码审查 | APPROVE，无未解决阻断 | 审查发现及修正如上 |
| 资源完整性、`git diff --check` | 通过 | output/visual-hierarchy/asset-check.json |

浏览器使用任务自有随机回环fixture与生产构建；Playwright来自既有工具环境，Chrome为本机安装。覆盖1440/390/320与明暗，手机为浏览器模拟；未声称真机或生产验收。自有mock、Nitro与浏览器已清理。详细截图/命令输出仅本机保存，不提交。
