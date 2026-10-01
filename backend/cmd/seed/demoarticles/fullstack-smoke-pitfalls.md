---
slug: fullstack-smoke-pitfalls
title: 全栈冒烟一天踩的五个坑：Kratos + Nuxt 管理后台
summary: 
tags: go, kratos, nuxt
status: published
published_at: 2026-10-01T02:06:46+08:00
---
给博客加管理后台（Kratos v3 + Nuxt 4），验收标准很简单：浏览器里走完「登录 → 写文章 → 发布 → 前台可见」。curl 层面的契约测试都绿了，真到全栈冒烟，一天之内连踩五个坑。每个坑都挺典型，记录下来。

## 坑一：FieldMask 不豁免校验

列表页的"发布"按钮，第一版只发 `{slug, status}` + `update_mask=status`，结果 400：`missing required field: article.title`。

原因是 proto 里 `title`、`content_md` 标了 `(google.api.field_behavior) = REQUIRED`，Kratos 的 VALIDATOR 中间件按**请求体**校验，根本不看你的 update_mask 想改哪个字段。FieldMask 是"改哪些"，REQUIRED 是"请求里必须有"——两套机制各管各的。

所以客户端要发完整字段 + mask 控制生效范围。这本身不算坑，真正的坑在后面。

## 坑二：转换层丢了一个字段，update 接口坏了三个里程碑

发完整字段后还是 400：`invalid article argument`。追进去发现 service 层的 `convertArticle` 只拷贝 slug/title/summary/content_md/tags，**没拷 status**；而 biz 层第一行就是"status 为 UNSPECIFIED 拒绝"。两行代码合谋，让 update 接口从 M1/S2 出厂起就是坏的——此前所有冒烟只测了 create/read/delete，谁都没碰过 update。

修复一行（透传 status），补一个纯函数单测。教训：**验收路径要覆盖每个 RPC，"能创建"不等于"能更新"**。

## 坑三：无 body 的 POST 也要 Content-Type

refresh 接口请求体为空，curl 默认不带 `Content-Type`，Kratos 直接 400 `CODEC: unregister Content-Type`。前端 `$fetch` 对无 body 的 POST 同样可能不带头。解法很无聊但有效：所有 POST 显式 `Content-Type: application/json`。

## 坑四：Nuxt 文件路由的嵌套陷阱

`/admin/posts/new` 打开后渲染的却是文章列表页。查了半天是 Nuxt 的文件路由规则：存在 `pages/admin/posts.vue` 和 `pages/admin/posts/new.vue` 时，前者自动成为后者的**父布局**，子页面要靠父组件里的 `<NuxtPage>` 渲染——而列表页当然没有。dev 日志里的 `NUXT_E4011` 警告早就把答案贴脸上了。

平级页面应该用 `posts/index.vue`，而不是 `posts.vue`。

## 坑五：BFF 后面，cookie 的 Path 永远匹配不上

后端把 refresh cookie 设成 `Path=/v1/auth`（安全收紧，好设计）。但浏览器访问的是 BFF 代理的 `/api/v1/auth/*`——带 `/api` 前缀，Path=/v1/auth 的 cookie 一个请求都匹配不上，刷新闭环静默失效。

在 BFF 里用 h3 的 `cookiePathRewrite: {'/v1/auth': '/api/v1/auth'}` 重写，httpOnly 语义原封不动，后端契约也不用改。

## 一点复盘

五个坑里三个（一、二、五）的本质是同一件事：**契约在边界处换了一次手，两边的理解就漂移了**——validator 与 mask、service 转换层、BFF 与后端的 cookie 语义。冒烟之所以能抓住它们，是因为请求真的从浏览器出发、真的穿过了每一层。契约测试再全，也替代不了这条完整链路。
