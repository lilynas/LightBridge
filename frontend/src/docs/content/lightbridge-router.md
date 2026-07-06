# LightBridge Router

LightBridge Router 是 LightBridge 的全协议消息中转组件。它把入站协议、账号出站协议和中转模式拆开处理，让同一个分组内的 OpenAI、Claude、Gemini、Antigravity 与自定义 Provider 账号可以一起参与轮询，不再被分组所属平台限制。Grok 订阅账号作为独立平台按 Responses-only 路由处理，不参与通用混合调度。

## 设计目标

1. 入站协议和出站协议解耦
   - 客户端可以使用 OpenAI Responses、OpenAI Chat Completions、Claude Messages 或 Gemini `generateContent` 入站。
   - 账号按自身支持的目标协议出站，不再要求与分组平台一致。

2. 以现有链路为基础做协议互转
   - Router 负责决策协议链路，不替换既有 forwarder、converter、billing、usage、并发限制和模型映射链路。
   - 跨协议转换以 OpenAI Responses 作为 canonical pivot：非 Responses 协议互转时走 `入站协议 -> OpenAI Responses -> 目标协议`。

3. 分组只表达权限与可见性
   - 分组继续控制用户权限、倍率、模型可见性、OAuth-only 限制等业务语义。
   - 分组不再作为账号平台过滤条件；只要账号在这个分组内且可调度，就可以进入候选池。

## 支持协议

| 协议 | 标识 | 常见入口 |
| --- | --- | --- |
| OpenAI Responses | `openai_responses` | `/v1/responses` |
| OpenAI Chat Completions | `openai_chat_completions` | `/v1/chat/completions` |
| Claude Messages | `anthropic_messages` | `/v1/messages` |
| Gemini generateContent | `gemini` | `/v1beta/models/*:generateContent` |

当前 LightBridge Router 覆盖消息生成类协议。Embeddings、Images、Realtime/WebSocket 仍保留各自专用路径。Grok 仅开放 OpenAI Responses HTTP 入口（`/v1/responses`、`/responses`、`/backend-api/codex/responses`），不开放 Chat、Messages、Embeddings、Images、WebSocket 和 Count Tokens。

## 中转模式

账号通过 `accounts.extra.relay_mode` 指定中转模式。未配置时默认使用 `router`。

| relay_mode | 控制台名称 | 行为 |
| --- | --- | --- |
| `router` | Router 转换 | 允许跨协议转换。Router 根据入站协议和账号能力选择目标协议与转换链。 |
| `passthrough` | 透传（同协议） | 只在入站协议与目标账号协议一致时可被调度，绕过 Router conversion。 |
| `full_passthrough` | 完全透传（原样转发） | 继承旧透传语义，任意协议原样转发，仅替换认证和必要头。 |

旧字段会兼容映射为 `full_passthrough`：

```json
{
  "openai_passthrough": true,
  "openai_oauth_passthrough": true,
  "anthropic_passthrough": true
}
```

新配置建议统一写入：

```json
{
  "relay_mode": "router"
}
```

## 路由决策

LightBridge Router 在请求上下文中维护这些字段：

| 字段 | 含义 |
| --- | --- |
| `InboundProtocol` | 客户端实际使用的入站协议。 |
| `TargetProtocol` | 最终命中账号的出站协议。 |
| `RelayMode` | 当前请求采用的中转模式。 |
| `ConversionChain` | Router 采用的协议转换链路。 |
| `FinalRelayFormat` | 最终发送给上游的 wire format。 |

典型决策流程：

1. Handler 识别入站 endpoint，写入 `InboundProtocol`。
2. Scheduler 从分组候选池中读取所有可调度账号，不再按分组平台过滤。
3. Router 根据账号 `relay_mode` 与 `SupportedTargetProtocols` 判断是否可用。
4. 若同协议命中，直接走同协议转发链路。
5. 若跨协议命中，构造转换链并复用现有 converter / forwarder。
6. 非 `full_passthrough` 响应按入站协议返回给客户端；`full_passthrough` 保持上游响应原样。

## 协议转换矩阵

| 入站 \\ 出站 | OpenAI Responses | OpenAI Chat | Claude Messages | Gemini |
| --- | --- | --- | --- | --- |
| OpenAI Responses | 同协议 | Responses -> Chat | Responses -> Claude | Responses -> Gemini |
| OpenAI Chat | Chat -> Responses | 同协议 | Chat -> Responses -> Claude | Chat -> Responses -> Gemini |
| Claude Messages | Claude -> Responses | Claude -> Responses -> Chat | 同协议 | Claude -> Responses -> Gemini |
| Gemini | Gemini -> Responses | Gemini -> Responses -> Chat | Gemini -> Responses -> Claude | 同协议 |

Provider 独有字段无法完全等价时，转换链只保证核心消息语义、模型、stream、tool/use 等通用能力尽量保真；不可等价字段会按现有兼容策略降级处理，并通过日志或 metadata 保留排查线索。

## 分组与账号轮询

LightBridge Router 上线后，分组不再限制所属平台：

1. 同一个分组可以同时包含 OpenAI、Claude、Gemini、Antigravity、自定义 Provider 账号。
2. 请求进入分组后，Scheduler 会轮询组内所有健康且模型可用的账号。
3. `passthrough` 账号只在同协议请求中可选。
4. `router` 账号可以被跨协议请求选中。
5. `full_passthrough` 账号走原样转发路径，适合需要完全保留上游协议行为的场景。
6. Grok 分组只调度 Grok OAuth 账号，并且只接受 Responses HTTP 请求。

如果仍看到 `503 no available accounts`，优先检查：

1. 账号是否在目标分组内且状态为可调度。
2. 账号模型映射或模型白名单是否包含请求模型。
3. 账号是否被限流、过载、临时禁用或并发占满。
4. `relay_mode=passthrough` 是否遇到了跨协议请求。
5. 自定义 Provider 是否配置了正确的 `accounts.extra.protocol`。

## 自定义 Provider

自定义 Provider 账号通过 `accounts.extra.protocol` 声明上游协议：

```json
{
  "protocol": "openai_responses",
  "relay_mode": "router"
}
```

可选协议包括：

```text
openai_responses
openai_chat_completions
anthropic_messages
gemini
```

自定义 Provider 可以加入任意分组。是否参与当前请求，由 Router 根据入站协议、账号协议能力和 `relay_mode` 共同决定。

## 日志与排查

非 `full_passthrough` 请求会按入站协议返回响应，并在请求日志中补充协议字段：

```text
inbound_protocol=openai_responses
target_protocol=anthropic_messages
relay_mode=router
conversion_chain=openai_responses,anthropic_messages
final_relay_format=anthropic_messages
```

排查跨协议请求时，建议同时查看：

1. 请求入口是否识别为预期 `InboundProtocol`。
2. 最终命中账号的 `TargetProtocol` 是否符合预期。
3. `RelayMode` 是否为 `router`、`passthrough` 或 `full_passthrough`。
4. `ConversionChain` 是否经过 OpenAI Responses pivot。
5. 上游错误是否来自协议转换前、账号选择阶段，还是目标 provider 返回。

## 推荐配置

1. 默认账号使用 `router`
   - 适合大多数混合协议分组，让 Router 自动处理跨协议请求。

2. 强协议兼容账号使用 `passthrough`
   - 适合只想服务同协议客户端、不希望发生协议转换的账号。

3. 对协议细节极敏感的账号使用 `full_passthrough`
   - 适合调试、上游兼容性验证，或必须保留请求 body/path/query/stream 原样的场景。

4. 混合分组保留模型治理
   - 分组平台不再过滤账号后，更应通过模型映射、模型可见性、账号健康和倍率配置表达业务策略。
