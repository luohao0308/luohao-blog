---
slug: fullstack-smoke-pitfalls
title: 从登录到发布：一次 Go 与 Nuxt 后台的验收路径
summary: 从一次点击到真正发布，追踪 Go 与 Nuxt 之间的字段、路由和 Cookie 边界。
tags: Go, Kratos, Nuxt, 全栈开发, 测试
status: published
published_at: 2026-10-01T02:06:46+08:00
---
一次后台操作的完成标准，是刷新后仍能继续编辑，并在公开页读到正确结果。

对应到本站，管理后台需要走通登录、创建草稿、修改正文、发布、公开读取这条路径。接口分别返回成功，只说明几个局部动作成立；浏览器能不能恢复身份、发布后的正文是不是刚保存的版本，还要放在一起看。

[![从浏览器到后端：一次操作，三个边界。](/images/editorial/web-flow.svg)](/images/editorial/web-flow.svg)

*页面、BFF 与后端各有自己的输入输出，问题常出在交接处。*

## 只发布状态，为什么会牵连正文

假设编辑器已经保存标题和正文，用户随后点击发布。发布请求只想修改状态，并没有重新提交所有字段。如果服务端直接把请求转换为完整业务对象，未携带的标题和正文就可能变成零值。此时完整对象校验可能拒绝请求，直接保存则可能覆盖旧数据。

本站的 `UpdateArticle` 采用了明确的顺序：先检查 slug 与 mask，拒绝未知路径，再读取现有文章，把选中的字段合并进去，最后转换为业务对象并更新。下面摘出核心步骤，省略错误处理：

```go
current, err := s.GetArticle(ctx, &v1.GetArticleRequest{
    Slug: req.GetArticle().GetSlug(),
})
// 实际代码在这里检查 err。
fieldmask.Update(req.GetUpdateMask(), current, req.GetArticle())
article, err := s.uc.UpdateArticle(ctx, convertArticle(current))
```

这使“没传正文”与“明确清空正文”有了区分。正文不在 mask 中，就保留原值；正文在 mask 中，即使新值为空，也应该交给业务规则判断能否接受。让前端每次发送完整文章虽然容易绕过局部校验问题，却把旧数据回写风险也交给了客户端。

[![FieldMask 合并示意](/images/editorial/fieldmask-code.svg)](/images/editorial/fieldmask-code.svg)

*先取得现有对象，再合并指定字段；合并后的对象仍须通过业务校验。*

先读再写也有代价：它本身没有解决并发编辑。两个窗口读到同一版本，随后先后提交，仍需单独讨论版本校验或冲突处理，不能从 FieldMask 推导出并发安全。

## 转换丢一个字段，页面就会出现另一种结果

DTO 到业务对象的转换看起来只是赋值，却决定了业务层实际收到什么。仓库里有 `TestConvertArticleCarriesStatus`，覆盖草稿、发布和未指定状态的转换。这个检查很具体：调用方传入发布状态，转换后不能悄悄回到零值。

验收更新时还应检查持久化后的读取结果。比如把正文从“初稿”改成带代码块的新内容，保存响应正确后，再从详情接口读取 Markdown 和渲染 HTML，最后查看公开页面。这样可以分清是提交失败、转换丢字段，还是展示仍在使用旧结果。只看按钮上的“保存成功”，定位不到这些差异。


[![本站文章更新服务的代码节选](/images/editorial/fieldmask-code.svg)](/images/editorial/fieldmask-code.svg)

*本站文章更新服务的代码节选。*


## 登录后的第二个请求更值得看

本站浏览器访问 `/api/v1/auth/*`，后端接收 `/v1/auth/*`。Nuxt BFF 去掉 `/api` 前缀转发请求，但 Cookie 的 Path 是浏览器侧的匹配规则，不能跟着服务器内部路径走。

```ts
return proxyRequest(event, target, {
  cookiePathRewrite: { '/v1/auth': '/api/v1/auth' },
})
```

如果响应仍写入 `/v1/auth`，浏览器随后访问 `/api/v1/auth/refresh` 时就不会因该路径规则携带刷新 Cookie。登录后的内存状态可能暂时掩盖问题，刷新页面才暴露身份恢复失败。排查时应看响应的 Set-Cookie、浏览器实际保存的 Path，以及刷新请求是否带 Cookie，而不是反复重试密码。

排查时也要保留操作顺序。先在同一标签页登录、保存草稿，再刷新并发布，与重新登录后直接发布，经过的会话恢复分支并不相同。记录失败请求的路径、状态码和不含敏感信息的错误原因，通常就能把范围缩到代理或业务层。Cookie、令牌与完整请求头不需要进入测试截图或公开文章。

另一个实用检查是保存后重新打开编辑器，确认表单回填的是服务端值。页面本地状态没有清空，可能让失败的保存看起来仍然成功；重新读取可以打破这层假象。

## 把用户动作变成一组断言

文件路由也要放进同一条路径：列表页在 `pages/admin/posts/index.vue`，新建和编辑是子路由。点击进入能用，还要验证直接打开编辑地址与刷新页面。

| 动作 | 要观察的结果 |
| --- | --- |
| 登录后刷新 | 身份恢复，后台仍可访问 |
| 只改正文 | 标题、状态、标签保持原值 |
| 只改状态 | 已保存正文不丢失，公开页可读取 |
| 提交未知 mask | 明确拒绝，不产生部分修改 |
| 登出后再次进入 | 不能继续使用原会话 |

接口测试适合迅速定位字段与错误，浏览器冒烟负责把路由、Cookie 和页面操作串起来。保留这两层证据，比给每个接口单独贴一个成功截图更接近后台真正可用的标准。

## 继续读什么

- [Nuxt 官方：文件路由](https://nuxt.com/docs/4.x/directory-structure/app/pages)
- [FieldMask 官方：字段更新语义](https://protobuf.dev/reference/protobuf/google.protobuf/#field-mask)
