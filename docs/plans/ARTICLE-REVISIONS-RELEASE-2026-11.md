# 文章版本与修订发布准备

- 请求：用户 2026-11-02 要求提交发布。
- 交付范围：本工作树已整合的问本文、搜索同步状态/重试、SEO 与草稿增强，以及文章 CAS/修订历史/差异恢复；不含原目录未整合的视觉/内容改写。
- 本地交付分支：feat/article-revisions；禁止推送 codex 临时分支。截图、fixture SQL备份、原目录patch/status和运行日志只留 output，不进入Git。
- 本地质量：重新运行Go build/test/vet/lint、前端lint/typecheck/build；开发阶段的真实MySQL迁移/并发/故障/备份恢复和Web专项见开发计划。提交必须包含新文件及生成物，不能只提交tracked diff。

## 交付步骤与边界

1. 本地提交：当前用户请求已授权；准备固定源码提交和Git tree，保留原目录改动。
2. Git交付：push、创建PR、合并分别经项目delivery_guard核对准确ref/SHA、一次性授权与平台证据。若无相关Issue，需要针对本次交付创建或关联；PR required CI通过才可合并。
3. 镜像：main合并会自动触发现有release workflow构建并推送镜像；不可把PR通过当成镜像已经成功。记录后端/前端不可变镜像及源码树身份。
4. 生产：先备份数据库和uploads并核验完整性，再在批准窗口仅替换backend/frontend；后端启动会自动应用17、18迁移。保留实际Compose完整覆盖文件链、原镜像和配置；不重建Caddy/数据库/Redis/ES，不改凭据或历史正文。

## 当前只读生产核验

- 2026-11-02重新核实远端main与生产Git均为fe77f3b277fe076bb3ecac5fde4302efc4762073；数据库migration=16、dirty=false。
- backend/frontend均healthy，但镜像来自不同次发布且各有Compose固定覆盖链；不可仅依全局IMAGE_TAG覆盖它们。
- 剩余空间约13GiB；发布前仍需重新核实镜像/备份实际容量，不能盲目清理既有镜像或备份。
- 一次性实际镜像/覆盖链证据保存在本机受控交付材料，不把生产环境文件、配置中的秘密或数据库内容提交。

## 停止与恢复条件

- required CI、镜像构建、备份完整性、迁移dirty或关键冒烟任一失败就停止，不能用force version或down migration掩盖。
- schema升级至18后，旧v16后端可能因缺少该迁移源拒绝启动，不能简单切回旧backend镜像。前端可在独立授权下回退并停写；后端恢复须用兼容schema18的修复版本，或针对准确备份另行授权数据恢复。
- 此准备文件和概括发布请求不替代交付守卫要求的准确版本一次性授权；不能自行修改持久gitPolicy或复用过去已消费的grant。
