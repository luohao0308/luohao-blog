---
workflow: dev-workflow
status: ready
updated: 2026-09-30
---

# dev-workflow 接入记录

> 本文件记录当前项目是否已经完成通用开发流程接入。它不是任务状态源，也不是项目稳定事实源；任务状态看 `TASKS.md`，稳定事实看 `PROJECT-SUMMARY.md`。

## 状态说明

- `pending`：模板已安装，但项目画像、命令、所有权或流程包映射仍未完成。
- `ready`：项目画像已根据代码和运行证据完成，后续任务可以按本流程直接执行。
- `blocked`：接入被缺少权限、运行环境或无法验证的关键事实阻塞。

机器可读的安装版本、流程包、Git 交付权限、接入状态和最近一次已记录审计时间保存在 `.dev-workflow/manifest.json`。该文件不应包含凭据、绝对本机路径或动态生产秘密，只约束当前机器，不能替代远端平台门禁。

## 首次接入步骤

1. 只读扫描仓库拓扑、子仓库、模块所有权、入口和本地运行方式。
2. 从仓库脚本、配置和 CI 中确认安装、启动、测试、lint、类型检查、构建、迁移和发布入口。
3. 找出已有架构、设计、契约、测试、运维和工作日志文档，建立职责映射；不要因为模板路径不同就复制第二套权威文档。
4. 识别受保护路径、凭据边界、需要人工授权的操作和无法验证的 Unknown。
   核验 `gitPolicy` 中 push、PR 创建/更新、远端 PR merge 的 mode 与 actor；默认均为 `manual + user`。核验 PR、CI、独立 Review 默认强制，删除和发布类权限不在普通初始化中开放。
5. 将已验证稳定事实填入 `PROJECT-SUMMARY.md`，将项目专属规则填入根 `AGENTS.md` 的项目扩展区或现有等价入口。
6. 按实际启用的流程包补充导航、命令矩阵、契约索引、Runbook 和验证入口。
7. 如果安装了 `feature-catalog`，运行 `python3 scripts/feature_catalog.py --init`，用项目事实替换脚手架，生成矩阵，并把 `--check` 接入适用的 CI。
8. 保持 `pending` 状态运行接入审计；预期退出码为 `2`，但不应有结构错误，告警应逐项处理或解释。
9. 完成检查清单后，同步将本文件与 manifest 的 `onboarding.status` 改为 `ready`，在 manifest 写入 UTC `lastAuditAt`，再运行审计并确认退出码为 `0`。

## 接入检查清单

- [x] 仓库拓扑和每个路径的所有权已确认。（空仓库：README + 流程文件；规划目录见 PROJECT-SUMMARY §2）
- [x] 真实开发、测试、构建和 CI 命令已记录。（M0 完成：命令矩阵见 PROJECT-SUMMARY §5 与 docs/development/README.md，均经 2026-09-29 实际验证）
- [x] 项目摘要、路径速查和项目专属规则已填充。
- [x] 已有文档与新流程包已建立职责映射，没有并行权威源。（接入前仅有 README，无并行权威源）
- [x] 契约、迁移、发布、健康检查和回滚入口按项目适用性登记。（空仓库阶段在 PROJECT-SUMMARY §6 登记为 Unknown/planned）
- [x] 敏感信息和不可逆操作边界已明确。
- [x] manifest 中 push、PR 创建/更新、远端 PR merge 的审批模式和执行角色符合项目约定；策略变更已经人工确认并记录 `policyChangedAt` / `policyChangedBy`。（默认 manual + user，未做策略变更）
- [x] PR、required CI、独立 Review 的远端强制门已登记；实现者不能成为唯一审批者，AI review 不计作独立批准。（manifest 已登记 true；GitHub 远端 branch protection 尚未配置，列为待办，属仓库设置类需人工操作）
- [x] tag/Release、package/image publish、deploy、migration/backfill、rollback、traffic switch、仓库设置、凭据和删除权限保持未授予。（privilegedOperationsDefault=deny，deleteAllowed=false）
- [x] Git 仓库中的本地 `info/exclude` 已包含完整且顺序正确的 dev-workflow managed block，Git 最终确认安装器创建的文件被忽略，且相关文件未被 Git 跟踪。（2026-09-29 经 `git check-ignore` 验证）
- [x] Unknown、阻塞和需要人工确认的事项已记录。（Unknown 见 PROJECT-SUMMARY；branch protection 需用户在 GitHub 配置）
- [x] 如安装 `feature-catalog`：未启用该流程包，不适用。
- [x] 已从 dev-workflow 分发仓库运行 `audit.sh`；最终 `ready` 状态审计退出码为 `0`，剩余告警已有解释。（strict 模式额外报告空仓库未勾项，详见审计记录与 TASKS.md 技术债登记）

## 既有文档映射

| 通用职责 | 当前项目权威路径 | 是否已核验 | 备注 |
|---|---|---|---|
| 任务状态 | `docs/TASKS.md` | yes | 流程安装创建，暂无任务 |
| 短期上下文 | `docs/WORKING-CONTEXT.md` | yes | 本机文件，不提交 |
| 稳定项目摘要 | `docs/PROJECT-SUMMARY.md` | yes | 2026-09-29 按空仓库事实填充，选型标注 planned |
| 架构/模块 | `docs/architecture/` | yes | 模板待 M0 后按真实结构填充 |
| 设计/计划 | `DESIGN.md`、`docs/design/`、`docs/plans/` | yes | 模板；里程碑计划落入 `docs/plans/` |
| 契约/生成物 | `docs/contracts/` | yes | 模板；proto 契约源 M1 起登记 |
| 功能清单/成熟度（如启用） | 未启用 | no | 未安装 feature-catalog 包 |
| 测试/验证 | `docs/testing/` | yes | 模板；CI 与命令矩阵 M0 后回填 |
| 运维/Runbook | `docs/operations/` | yes | 模板；部署目标未定（Unknown） |
| 历史证据 | GitHub Actions / PR 记录 | no | 尚无 CI，Unknown |

## 审计记录

| 时间 | 命令/入口 | 结果 | 证据/剩余风险 |
|---|---|---|---|
| 2026-09-29 | `audit.sh --strict`（pending 阶段，dev-workflow 0.4.2） | pending | 退出码 2（符合预期）；无结构错误，唯一告警为 frontmatter 占位日期，已修复 |
| 2026-09-29 | `audit.sh --strict`（ready 阶段） | ready（strict 未通过） | 退出码 1：唯一未勾项「开发/测试/构建/CI 命令已记录」在空仓库阶段客观无法完成；已登记 TASKS.md 待办 T-001 与技术债，M0 后回填消除 |
| 2026-09-29 | `audit.sh`（ready 阶段，正式门） | ready | **退出码 0**；剩余告警同上，已有解释与跟踪任务 |
| 2026-09-29 | `audit.sh --strict`（M0 完成后） | ready | **退出码 0，strict 全绿**；空仓库未勾项已随 M0 命令矩阵回填消除 |
| 2026-09-29 | `install.sh`（v0.4.2→v0.5.0 升级）+ `audit.sh --strict` | ready | 升级仅版本号推进（内容 diff 仅 VERSION）；dry-run 全部 [skip]；strict 退出码 0 |
| 2026-09-29 | `install.sh`（加装 api-governance/containers/delivery-cicd/deployment）+ `audit.sh --strict` | ready | 新增 4 份治理文档；manifest 共 9 包 @ v0.5.0；strict 退出码 0 |
| 2026-09-30 | `install.sh`（v0.5.0→v0.6.0 升级，含 9 包重跑）+ `audit.sh --strict` | ready | 版本推进至 0.6.0；9 包、gitPolicy、onboarding 状态保留；delivery 包 report-worktrees.py 同步至 0.6.0 内容（新增 --no-size），其余已存在文件按不覆盖策略跳过；strict 退出码 0 |

_完成接入后保留本文件作为流程状态入口；不要把一次性排障过程复制到这里。_
