# 博客同镜像发布命令迁移

状态为已批准实施。迁移范围是保留第 14 代生产验收已经使用的审批与容器替换边界，并将博客运维入口移入本仓库。生产验收来源为 [Dev Workflow 运行 #43830947832](https://github.com/luohao0308/dev-workflow/actions/runs/43830947832)。

1. Dev Workflow 独立提交 [Docker 兼容性修复 #55](https://github.com/luohao0308/dev-workflow/pull/55) 和 [审批客户端 #56](https://github.com/luohao0308/dev-workflow/pull/56)。验证使用主线基础上的框架单测与 PR 检查，回退点为各自的提交。
2. 博客在 deploy/ 中承接命令、目标探测和说明。命令读取两个仓库各自已发布的固定提交，部署 worker 继续使用 Dev Workflow 的原始归档。通过仓库内离线测试验证归档边界、预检和阶段操作，合并后用只读预检检查现有生产基线。回退点为博客仓库的迁移提交。

本次迁移不执行容器变更，也不重跑已通过的生产替换。
