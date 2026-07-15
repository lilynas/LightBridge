# LightBridge 0.3.1 正式版

## Grok Builder Tool Calling 兼容性修复

- 修复 LiteLLM 路由 Grok 渠道到 Grok Builder 时，Responses 流式事件不符合严格客户端预期的问题。
- 为 function call 输出生成连续、稳定的 `output_index`、item ID 与 call ID，并在完整事件链和终态响应中保持一致。
- 补齐 `response.function_call_arguments.done`、工具项 `response.output_item.done` 与 reasoning 输出项生命周期事件。
- 保留协议要求的零值索引和空字段，避免 JSON `omitempty` 破坏 Grok Builder 所需的事件结构。
- Grok Build CLI 用户代理现在会自动使用严格 Responses 兼容模式。

## 稳定性与回归覆盖

- 增加纯工具调用、reasoning + 工具调用、并行非连续工具索引和 Grok Build 用户代理测试。
- 修复终态响应输出、创建时间与 usage 字段归一化，避免流式结束事件缺失上下文。
- 本版本没有数据库 schema migration，现有配置和账号数据无需迁移。

## 发布验证范围

- `apicompat` 生产代码编译通过，相关现有与新增测试全部通过。
- Router 客户端配置测试通过。
- Go 格式化与 `git diff --check` 通过。
- 代码清单与 Release 配置校验通过。
