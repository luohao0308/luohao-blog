---
slug: dev-workflow-project
title: dev-workflow：把项目规则留在仓库里
summary: dev-workflow 的出发点：把项目知识、协作规则与交付检查留在仓库里。
tags: Python, dev-workflow, AI协作, 开源项目
status: draft
---

换一个 AI 会话，项目的代码还在，背景却常常需要重新解释。接口由哪里生成、哪些命令能跑、哪些文档只是计划，这些信息如果只留在对话里，下一轮工作就容易从猜测开始。我维护的 [dev-workflow](https://github.com/luohao0308/dev-workflow)，是在仓库里给这些信息安排固定入口，并补上能执行的检查。

## 先把项目事实分开放

Core 从 `AGENTS.md` 引导读取文档，但不要求每次加载整个目录。`TASKS.md` 记录当前任务，`PROJECT-SUMMARY.md` 保存稳定结构和命令，`WORKING-CONTEXT.md` 留给本机短期交接。已有架构文档则继续作为权威来源，通过映射接入，而不是复制一份同名模板。

例如，为文章列表增加筛选，下一位助手需要知道接口定义、服务入口、测试命令和当前阻塞。它不需要读完过去每次安装的日志。把“接口由哪个文件生成”留在稳定摘要，把“筛选测试已过，页面验收未做”留在任务上下文，两者更新频率不同，也不应共享一个越来越长的备忘录。

[![从上下文到交付，给协作留下可检查的入口。](/images/editorial/delivery-flow.svg)](/images/editorial/delivery-flow.svg)

## Core 和流程包怎样落到仓库

分发仓库把 `core/` 与 `packs/<name>/` 当作覆盖层：其中的相对路径就是安装后的路径。架构、设计、交付、契约、运维和功能清单按需选择。一个仅需协作规则的小项目，不必为了形式完整先填写所有模板。

```text
目标项目/
├── AGENTS.md
├── docs/
│   ├── README.md
│   ├── TASKS.md
│   ├── PROJECT-SUMMARY.md
│   └── WORKFLOW-ADOPTION.md
└── .dev-workflow/
    └── manifest.json
```

manifest 记录本机安装版本、文件来源、安装动作和原始哈希。它解决的是“哪些文件由工具创建、之后有没有改过”，不是团队共享的所有项目配置。已经采用的 `workflow.config.json` 负责项目能力、命令目录和文档映射，两者不能混作一份配置。


[![项目知识、任务与临时上下文分别存放](/images/editorial/workflow-files.svg)](/images/editorial/workflow-files.svg)

*项目知识、任务与临时上下文分别存放。*


## 安装结束后还要完成项目画像

安装器使用 Bash 或 PowerShell，Python 工具采用标准库，不需要给业务项目添加依赖。实际接入先做 dry-run，确认已有文件会被保留；已有 `AGENTS.md` 追加带稳定标记的核心区块，普通文件不直接覆盖。

更费判断的是接下来的只读画像：仓库有几个模块，服务从哪里启动，测试是否依赖数据库，哪些配置涉及生产。查不到的事实要写 Unknown，不能从模板里的示例推断项目已经拥有这些能力。

`WORKFLOW-ADOPTION.md` 和 manifest 中的 onboarding 状态需要保持一致。审计能检查结构、归属和状态，却不会替接入者把状态改为 ready。严格审计通过，说明接入契约满足；业务命令能否成功，还需要实际运行项目对应的检查。

## 用小工具缩短定位路径

我希望普通工作不依赖常驻流程服务。安装后的入口可以是几个明确的命令：

```bash
python3 scripts/workflow_check.py --repo . --json
python3 scripts/feature_catalog.py --query "文章筛选"
```

第一条只读检查流程配置和本机 Git 基线，不运行项目测试，也不检查远端生产状态。第二条要求已安装并维护功能清单，作用是定位相关能力、代码与证据，避免 AI 为一个缺陷加载整份功能目录。

查询结果只是索引入口。定位到测试文件后，仍需读取用例断言并运行相关检查；一条证据路径如果失效，也应该作为清单缺口修正，而不能让助手自行补出结果。

功能清单也要区分“有入口”和“已成熟”。代码路径、测试证据与已知缺口应分别登记；由脚本生成的矩阵是阅读视图，不能手动改几格就当作功能验收。清单若长期不更新，照样会误导检索，因此变更影响到已登记功能时，需要同步检查它。

## 升级为什么不能重新复制模板

规则会演进，项目也会定制同一份文档。直接重装新版，无法判断一段差异来自模板更新还是项目自己的约束。升级引擎因此比较旧分发基线、项目当前内容和新版内容，先生成计划，再应用。

```bash
# 在新版分发目录中预览，路径为示意
python3 scripts/upgrade.py plan   --target /project --baseline /old-dev-workflow   --output /tmp/workflow-upgrade.json
```

未知基线、文本冲突或已修改的交付权限脚本会阻止整份计划应用。回退也要避免覆盖升级后新增的用户修改。这样做增加了一次计划审阅，却保住了项目规则的归属；相比自动把模板刷到最新，我更愿意让冲突显式出现。

本机文件的排除使用 Git 解析出的 `info/exclude`，不改项目 `.gitignore`。但已跟踪文件不会因为新增 exclude 就停止跟踪，工具只能告警。这也是安装时必须检查现状，而不能只看文件名的原因。

## 留下可以继续工作的边界

这套流程最终要服务于具体变更。局部调整沿用现有模式并运行最小检查，跨契约、权限或恢复边界的任务才补更深的决策和验证。文档数量不应成为完成指标。

远端交付另由 guard 检查执行者、准确版本和证据；发布与部署还需要各自的可信执行条件。安装流程本身不会获得这些权限。对我而言，一个有效的接入结果，是下一轮助手能找到事实、知道还缺什么，并用项目真实命令验证下一步，而不是把模板读得更熟。

## 继续读什么

- [dev-workflow：源码与使用说明](https://github.com/luohao0308/dev-workflow)
- [GitHub 官方：代码审查](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/reviewing-changes-in-pull-requests/about-pull-request-reviews)
