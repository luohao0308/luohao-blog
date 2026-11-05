---
slug: blog-article-lifecycle
title: 一篇文章如何在本站发布、被搜索，再被提问
summary: 从保存 Markdown 到返回带引用的回答，顺着一篇文章看 Nuxt、Kratos、MySQL 和 Elasticsearch 如何分工。
tags: Go, Kratos, Nuxt, Elasticsearch, 个人博客
status: draft
---
我在后台写完一篇 Markdown，点下发布。读者可以从列表打开它，可以在搜索里找到它，也可以留在详情页问一句「这篇文章讲了什么」。这三个动作看的是同一份内容，却允许不同的部分出错。理解这种分工，比记住站点用了哪些框架更有用。

![博客内容架构，分别展示发布、搜索和问本文的路径](/images/editorial/portfolio-2026-11/blog-lifecycle.svg)

## 发布以 MySQL 里的文章为准

后台是 Nuxt 页面，请求通过同源代理交给 Kratos。Go 后端用 goldmark 将 Markdown 渲染成 HTML，正文和渲染结果随文章保存。发布状态由后端控制，公开页面通过文章标识读取已发布内容，Nuxt 负责把页面渲染给浏览器。

同一篇文章可以同时被两个窗口打开。编辑器提交保存时带上自己读到的版本，更新时用这个版本作为条件。版本冲突时，提交旧版本的编辑页面可以继续保留输入，让作者比较和处理差异。修订快照与正文一同提交，恢复旧稿会产生新版本。具体的写入条件和两窗口复现见[文章修订案例](/posts/article-cas-revisions-case-study)。

![线上博客的桌面文章详情，含封面和章节目录](/images/editorial/portfolio-2026-11/blog-article-desktop.png)

## 搜索索引可以单独恢复

Elasticsearch 提供候选文章标识。搜索返回前，后端按标识读回数据库记录，并筛掉当前未发布的文章。即便索引里一时残留旧文档，公开结果仍经过发布状态检查。

保存成功后，系统尝试将最新内容同步到索引。索引写入失败时，MySQL 里的正文仍然保存成功，文章链接仍可读取，搜索可能暂时遗漏新内容。后台可以单独检查同步状态、重试这一篇，再回读验证结果。这里保留了一个明确的恢复入口，详见[搜索同步案例](/posts/article-search-sync-case-study)。

## 问本文从公开正文选段落

当读者在详情页提问，后端先按文章标识读取公开正文，再从 Markdown 章节中选出与问题相关的段落。模型收到问题与选中的内容，界面同时提供参考段落和页内锚点。读者可以点回原文，核对生成回答所用的上下文。

这个入口围绕当前文章工作，区别于站内搜索。找不到相关段落时，界面会说明本文未找到匹配内容。模型生成失败但已经找到段落时，仍可展示这些段落。正文、索引和生成回答因此各有清楚的失败边界。

## 从哪条路径继续读

- 想看前端登录到文章发布如何一起验证，可以读[从登录到发布的全栈验收](/posts/fullstack-smoke-pitfalls)。
- 想了解技术选型和我承担的工作，可以看 [luohao-blog 作品页](/projects/luohao-blog)。
- 想对照实现，可以查看[本轮对应的公开源码](https://github.com/luohao0308/luohao-blog/tree/df0d04eea8ba117fdadbe60f343342d9d7deed36)。
