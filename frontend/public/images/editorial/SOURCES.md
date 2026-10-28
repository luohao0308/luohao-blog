# 当前封面：创作者缩略图（2026-10-28）

依据本轮用户要求，当前八张封面采用主题短句、大字标题、强对比配色与具象画面，替代上一轮实拍图库方案。使用内置 imagegen 重新创作，未搬用视频原图、人物肖像、品牌或观看量标识。封面不代表实际产品界面、生成结果或运行截图。

参考时观察了以下公开 B 站视频封面的标题层级、背景与信息对比；原始参考缩略图只保存在本机审阅目录，不随网站发布：

- [AI 编程完整工作流实战](https://www.bilibili.com/video/BV1yWMr6jErd/)
- [从需求跑到验收的 AI Coding 演示](https://www.bilibili.com/video/BV1GaBVBpEGE/)

当前资源位于 `covers/`，每张 1536×864 WebP：

| 文件 | 封面短句 | 对应主题 |
|---|---|---|
| workflow.webp | 换个 AI，别从头讲 | dev-workflow / 项目知识 |
| guard.webp | AI 写完了，谁敢合并？ | 交付守卫 |
| harness.webp | Agent，不止聊天 | Forge Harness |
| aigc.webp | 一句话背后的后端 | AIGC 接口适配 |
| fullstack.webp | 登录成功，不等于能用 | 全栈验收 / 本站 |
| ent.webp | 标签丢了？先查关系 | Ent 关系案例 |
| mysql.webp | 191 / 255，到底选谁？ | MySQL 索引案例 |
| workbench.webp | 岗位太多，投递别乱 | 岗位机会工作台 |

原始生成图、完整提示词和源路径清单保存在本机 `output/profile-content/creator-covers/`。以下为此前素材与正文插图的历史记录；历史照片已退出当前封面映射，正文 SVG 与 Ent 官方图仍沿用。

---

# 当前使用的配图（2026-10-28 第二次调整）

按本次审阅反馈替换全部旧封面，采用以下实拍。全部来自标为免费使用的 Unsplash 页面，使用 [Unsplash License](https://unsplash.com/license)，详情页保留作者与原始页面链接。图片是主题配图，未作为作者本人拍摄、产品输出或部署现场展示。

| 文件 | 作者 | 来源 |
|---|---|---|
| library.webp | Dmitry Spravko | [图书馆书架](https://unsplash.com/photos/rows-of-books-on-library-shelves-lLtNLh7EkbE) |
| stairs.webp | Rubén García | [白色建筑阶梯](https://unsplash.com/photos/photography-of-white-concrete-stairs-R-wQExeiGrc) |
| ceramics.webp | Jonathan Cosens Photography | [窗边陶器](https://unsplash.com/photos/row-of-ceramic-vases-on-a-radiator-MTCHVzNlSL0) |
| desk.webp | Lauren Mancke | [纸笔与电脑工作台](https://unsplash.com/photos/turned-off-macbook-pro-beside-white-ceramic-mug-filled-with-coffee-aOC7TSLb1o8) |
| workspace.webp | Lauren Mancke | [藤编托盘与电脑](https://unsplash.com/photos/gray-laptop-computer-on-beige-wicker-chair-xcI7f25UQSs) |

七张原有流程图改成浅色纸面、细线与低饱和配色，删除英文眉题、品牌页脚、编号装饰；保留具体技术名称。新增的五张说明图：

- workflow-files.svg：依据 dev-workflow Core 目录整理，说明文档与脚本的分工。
- harness-modules.svg：依据 Forge Harness README 的模块目录整理。
- fieldmask-code.svg：本站 `internal/service/article.go` 的 UpdateArticle 代码节选。
- mysql-schema.svg：本站 baseline 与 subscribers SQL 迁移的字段节选。
- api-adapter.svg：接口适配说明示例，不是工作项目源码。

Ent 官方图片及 Apache-2.0 许可沿用。以下保留上轮素材记录，旧的电路板、抽象纹理与线缆照片已经退出页面映射。

---

# 图文素材来源

检索与下载日期：2026-10-28。照片已转换为本地 WebP，页面不依赖第三方图片热链。

| 文件 | 作者 / 来源 | 原始页面 | 使用方式 |
|---|---|---|---|
| circuit-board.webp | Brecht Corbeel / Unsplash | [绿色电路板主题插图](https://unsplash.com/photos/a-circuit-board-illuminated-by-vibrant-green-light-and-reflections-dWIRGa_8wdk) | dev-workflow 与工程交付主题封面；非项目硬件照片 |
| abstract-blue.webp | Pawel Czerwinski / Unsplash | [红蓝抽象纹理](https://unsplash.com/photos/a-close-up-of-a-red-and-blue-abstract-painting-iuhLxpgSAuA) | Agent / 关系结构主题封面 |
| abstract-warm.webp | Pawel Czerwinski / Unsplash | [橙红抽象绘画](https://unsplash.com/photos/abstract-artwork-PPwMUmx4NSA) | AIGC 主题封面；非项目生成作品 |
| network.webp | Taylor Vick / Unsplash | [网络设备与线缆](https://unsplash.com/photos/cable-network-M5tzZtFCOfs) | 后端与系统边界主题封面；非项目部署现场 |
| ent-official-relations.png | Ent 官方文档 | [Edges](https://entgo.io/docs/schema-edges/) · [原图](https://entgo.io/images/assets/er_user_pets_groups.png) | 正文中演示 User / Pet / Group 关系；保留原图，未改绘 |

四张照片来源页均标记为免费图片，采用 [Unsplash License](https://unsplash.com/license)，并在详情页保留作者与来源链接。Ent 图采用仓库的 [Apache-2.0 许可](https://github.com/ent/ent/blob/master/LICENSE)，许可原文见同目录 `ENT-LICENSE.txt`。

其余七张 SVG 为本站根据代码、项目文档与已确认职责绘制的简化说明图，非第三方产品截图，不表示完整生产部署结构：

- delivery-flow.svg：交付角色、版本与平台证据。
- agent-flow.svg：个人 Harness 任务链路与关注点。
- aigc-flow.svg：已确认的前端输入、后端适配、外部 API 链路。
- web-flow.svg：浏览器、Nuxt BFF、Kratos API 与验收点。
- ent-relations.svg：文章、关联表与标签的案例推演。
- index-design.svg：业务、字段与索引的案例推演。
- workbench-flow.svg：Vue、FastAPI 与 PostgreSQL 的简化分工。

文字参考使用各文章末尾与项目详情中的源码/官方文档链接；没有整段转载第三方文章。
