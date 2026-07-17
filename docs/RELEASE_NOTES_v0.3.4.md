# 0.3.4 版本更新

## 修复问题

1. 修复 OpenAI OAuth Tool Calling 因 `input.namespace` 被拒绝而断流
    - 修复上游返回 `unknown_parameter: input[n].namespace` 后被泛化为 `Upstream request failed`，最终导致客户端显示 `stream disconnected before completion` 的问题。
    - 覆盖 Responses HTTP、流式 HTTP、WSv2、OAuth `ctx_pool`、WebSocket passthrough 与 WS→HTTP bridge。
    - WebSocket 兼容重试保持在同一条物理连接上，避免 `store=false`、`previous_response_id` 与 `function_call_output` 的上下文失效。

2. 修复 namespace 兼容逻辑在不同传输模式间行为不一致
    - 仅在上游明确拒绝历史 `input` 项的 namespace 时执行一次兼容重试。
    - 只移除历史 `input` 项的 `namespace`，保留 `tools` 中的 namespace 定义和原生 namespaced tool identity。
    - 同一 WebSocket 连接会记住上游能力，后续轮次直接兼容，不再每轮先失败一次。

3. 补齐兼容错误格式与输入形态
    - 同时识别 `input[18].namespace` 与 `input.18.namespace` 错误路径。
    - 支持数组和单对象两种 Responses `input` 形态。
    - 兼容重试最多执行一次，避免重复拒绝时形成无限循环。

4. 修复 OAuth passthrough 缺少 `instructions` 时被本地错误拦截为 403
    - HTTP passthrough 与 WebSocket ingress 统一补齐与旧链路一致的默认 instructions。
    - OpenAI-compatible 客户端无需为 ChatGPT 内部端点额外构造该字段。
    - 明确的账号安全策略和上游真实 403 仍按原有规则处理。

## 优化调整

1. 统一 OpenAI Responses namespace 兼容层
    - HTTP、池化 WebSocket、passthrough WebSocket 与 HTTP bridge 共用一致的错误判定规则。
    - WebSocket 重试前会清理失败请求产生的临时事件、响应 ID、usage 和计量状态，避免污染成功结果。

## 升级说明

1. 本版本无需数据库或配置迁移
    - 现有 OpenAI OAuth 和 API Key 账号配置可以直接使用。
    - namespace 兼容仅在上游明确拒绝时启用，不会默认关闭原生 namespace 支持。

## 验证与发布备注

1. 已完成完整回归验证
    - namespace 定向测试、OAuth/passthrough 两轮同连接测试和 Go race detector 均通过。
    - `internal/service/...` 与后端 `go test -tags unit ./... -count=1` 全量测试通过。
    - Go 格式化与 `git diff --check` 通过。
