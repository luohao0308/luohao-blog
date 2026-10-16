# 搜索与单轮问答结果契约

契约源仍为 `backend/api/blog/v1/{article,search,chat}.proto`；本文件解释兼容策略，不替代生成物。

## 问答

`POST /v1/chat` 仍接收 `query`、可选 `top_k`，返回原有纯文本 `answer`、检索候选 `citations` 和增量 `outcome`。HTTP codec 使用 snake_case 和数字枚举。

| outcome | 含义 | 客户端行为 |
|---|---|---|
| 0 / 缺省 / 未知值 | 旧版本或未知结果 | 展示已有文本/参考文章，不推断生成成功 |
| 1 ANSWERED | 生成回答 | 展示回答，可复制 |
| 2 NO_MATCH | 检索可用但无公开文章命中 | 提示换种问法，不调用LLM |
| 3 RETRIEVAL_UNAVAILABLE | 索引或读取故障 | 提示稍后手动重试，不伪报无匹配 |
| 4 GENERATION_UNAVAILABLE | 检索成功、生成失败或空回答 | 保留参考文章，可手动重试 |

生成未配置仍保留HTTP503，非法查询与限流保留400/429。每次问题独立；不传历史上下文、不流式输出。`citations` 是检索到的参考文章，不保证每一条均被回答实际引用，不宣称逐句证据。HTTP200不能单独证明生成成功。

## 搜索

`GET /v1/search/articles` 仍返回 `ArticleSet`，增加 `search_outcome`：0缺省/非搜索列表，1可用，2不可用。旧客户端仍可读articles/next_page_token。索引NotFound陈旧命中可跳过；未知hydration故障标不可用，避免半截结果误作完整成功。

Web保留服务端相关性顺序，后续仅应用显式标签/分类约束。不可用/请求失败时才能用已加载公开文章进行字面降级，并明确提示“本地匹配”；不把本地字面条件叠加到语义搜索命中上。新请求/清空查询使旧响应失效。

## 验证入口

- `backend/internal/biz/chat_test.go`：生成、空结果、检索与读取故障、空回答、旧包装兼容。
- `backend/internal/server/codec_test.go`：数字枚举与snake_case wire格式。
- `backend/internal/service/chat_outcome_test.go`：DO→DTO映射。
- `miniprogram/scripts/verify.cjs`：数字/字符串结果、小程序原位重试、HTTP200降级。
- `scripts/e2e/ui_search_chat_e2e.cjs`：Web搜索/竞态/降级、IME、长度、重试、草稿、复制、旧响应兼容。

字段为增量，无Schema迁移。先部署兼容后端再更新客户端可获得完整状态；客户端面对旧服务保持保守显示。本文不代表已执行部署。
