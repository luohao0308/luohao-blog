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

## PR 检查修复

- Issue [#122](https://github.com/luohao0308/luohao-blog/issues/122)、PR [#123](https://github.com/luohao0308/luohao-blog/pull/123) 已按授权创建，首次 head 为 `6b01b1158d223d8c7dd2af69fa230ef5b077c0ed`。
- CI [41331172877](https://github.com/luohao0308/luohao-blog/actions/runs/41331172877) 的前端/小程序通过，后端真实身份集成测试失败。本地隔离复现确认 `TestWechatHTTPDisabledMySQLPreservesWebsiteAuthAndRead` 查询 `articles.version` 时，fixture 仍为 schema16。
- 保留 backfill CLI 仅允许 clean schema16 的门禁及原有回填/备份恢复检查；这些步骤完成后，仅把应用测试主库迁移至当前嵌入版本18，再运行原完整 TestWechat 列表。备份恢复库仍验证16；升级检查旧文章列、用户、身份绑定不变，版本默认1、修订表为空及迁移幂等。
- 新鲜本地验证：`python3 -m unittest discover -s scripts -p test_wechat_identity_test.py`（3项通过）；`python3 scripts/test_wechat_identity.py`（真实 MySQL/Redis/HTTP 全通过、零跳过，v15→16回填/备份恢复及应用升级18均通过）；`git diff --check` 通过。脚本修复不改应用运行时代码。
- 首次 push/PR 一次性授权已消费；修复提交改变 head SHA，需要重新绑定准确版本后更新 PR。尚未合并、构建发布镜像或操作生产。
