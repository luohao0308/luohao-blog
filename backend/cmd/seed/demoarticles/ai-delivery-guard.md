---
slug: ai-delivery-guard
title: 给 AI 编码助手上一道锁：415 行的交付守卫
summary: 
tags: go, dev-workflow, ai-agent
status: published
published_at: 2026-10-01T02:06:46+08:00
---
写这个博客的过程中，我给自己加了一条有点"自虐"的规则：AI 助手可以写代码、可以提交，但 push、创建 PR、合并 PR 每一次都要过一道 415 行的 Python 守卫，而且每次都要一份我本人当次签发的一次性授权。

## 为什么不直接让 AI 推

用 agent 做开发最大的效率红利，是它可以从"改完代码"一路跑到"PR 合并"。但这个红利恰好也是风险：合并进 main 的东西，责任人是仓库的主人，不是模型。流程要求、执行权限、质量门禁是三层不同的东西——CI 绿不代表这一步被授权执行了，被授权执行也不代表门禁免检。

所以我的 `gitPolicy` 是 `manual + user`：AI 想动远端，先拿授权。

## 一次性授权的九个要素

授权不是一句"可以"。守卫要求每次授权文件绑定九个字段：

```json
{
  "grantId": "m2s4-merge-20261001",
  "approvedBy": "user",
  "repository": "/Users/luohao/Desktop/vibecoding/luohao-blog",
  "remote": "origin",
  "remoteUrl": "https://github.com/luohao0308/luohao-blog.git",
  "operation": "merge",
  "sourceRef": "feat/m2-s4-admin-frontend",
  "targetRef": "main",
  "sha": "71cb60b0e57ec8dbf68c0fd5657f8fc0a816ec31",
  "expiresAt": "...",
  "maxUses": 3
}
```

操作对象变了、SHA 变了、授权过期了、次数用完了、失败了要重试——任何一种情况都要重新找我签字。授权文件只存在于本机 `.dev-workflow/`，跟着机器走，不进 Git。

## 平台证据也不能少

光有授权还不够。守卫同时要求一份**五分钟内**从托管平台拉取的证据，证明"世界和我以为的一致"：push 前目标分支确实不存在（fast-forward）、merge 前.required CI 确实全绿、head SHA 确实没被推过新提交、分支保护确实允许合并。任何一项读不到、过期了、对不上，守卫直接 deny。

这套东西落在 415 行 Python 里，有 63 处校验点，全部 fail-closed：manifest 读不了 deny，字段缺失 deny，文件权限不对（必须 0600）也 deny。

## 实战一天的感受

今天是这套体系第一次完整跑完 push → PR → merge 三步。有意思的是我自己也踩了守卫的坑：第一次提交 merge 证据时，我把字段嵌套在一个 `prEvidence` 对象里，而守卫的报错标签 `prEvidence` 其实只是错误信息前缀，字段必须平铺在顶层——deny 了一次，改成平铺就过了。

这种"烦"恰恰是设计目的。AI 每一步都要停下来找我要签字，签字绑定精确的 SHA——它没法"顺手"把一个没验证过的提交带上去。对个人项目来说，这就是一道足够好的锁。
