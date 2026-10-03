package server

import (
	"encoding/json"

	"github.com/go-kratos/kratos/v3/encoding"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Kratos 默认的 "json" codec 用 stdlib encoding/json 序列化 proto 消息，
// 生成代码里的 omitempty 会把所有空字段整体省略：一篇无标签文章到达前端时
// 连 `tags` 键都不存在，前端一处未判空的下标就 SSR 打挂整页（2026-10 review
// P0 的根因）。这里以 protojson 语义重新注册 "json" codec，让 HTTP 线上格式
// 与 openapi 契约对齐：空 repeated 字段输出 []、int64/uint64 输出字符串、
// Timestamp 输出 RFC3339。UseProtoNames/UseEnumNumbers 钉死前端已依赖的
// snake_case 字段名与数字枚举；非 proto 载荷（kratos 的 errors.Error 等）
// 保持 stdlib 行为，错误体形状不变。请求方向 protojson 同时接受两种字段名，
// 对现有客户端透明。

var hybridJSONCodec = &jsonCodec{
	marshal: protojson.MarshalOptions{
		UseProtoNames:   true,
		UseEnumNumbers:  true,
		EmitUnpopulated: true,
	},
	unmarshal: protojson.UnmarshalOptions{DiscardUnknown: true},
}

func init() {
	encoding.RegisterCodec(hybridJSONCodec)
}

// jsonCodec 是 kratos encoding.Codec：proto 消息走 protojson，其余走 stdlib。
type jsonCodec struct {
	marshal   protojson.MarshalOptions
	unmarshal protojson.UnmarshalOptions
}

func (c *jsonCodec) Marshal(v any) ([]byte, error) {
	if m, ok := v.(proto.Message); ok {
		return c.marshal.Marshal(m)
	}
	return json.Marshal(v)
}

func (c *jsonCodec) Unmarshal(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	if m, ok := v.(proto.Message); ok {
		return c.unmarshal.Unmarshal(data, m)
	}
	return json.Unmarshal(data, v)
}

func (c *jsonCodec) Name() string {
	return "json"
}
