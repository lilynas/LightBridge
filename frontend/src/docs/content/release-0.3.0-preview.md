# 0.3.0-preview 版本更新

## 新增功能

1. 新增 Grok Build 与官方 xAI API 双 OAuth 模式
    - Grok OAuth 明确区分 `build_proxy` 与 `official_api`，默认使用 Grok Build。
    - Build 授权链路携带 `referrer=grok-build`，并保存 Token capability、referrer 与重新授权状态。
    - OAuth Session 冻结 mode、PKCE、redirect URI 与代理配置，支持 Redis 多实例一次性消费。

2. 新增 Grok Build 多协议与多轮工具调用支持
    - OpenAI Responses、Anthropic Messages、Chat Completions 与 Responses WebSocket bridge 可路由到 Grok。
    - 多轮工具调用会回放有效的 encrypted reasoning 与 function/custom tool call，再衔接 tool output。
    - Reasoning replay 支持本地有界缓存与 Redis 多实例共享，并按 API Key、分组、模型和会话隔离。

3. 新增 Grok Token context 与真实可用性探测
    - 对可解析 JWT 检查 `referrer=grok-build`；opaque Token 交由真实上游最终验证。
    - OAuth 创建、重新授权与刷新后执行最小 Build 探测。
    - 401/403/资格错误要求重新授权，429 进入冷却，5xx 与网络错误按瞬态故障处理。

4. 新增 Grok 账户迁移与管理能力
    - CPA xAI/Grok 导入会识别 Build、Official API、unknown 与 incompatible capability。
    - OAuth 空账户名称按 email、JWT claims、subject/account ID 和平台默认名逐级回填。
    - 管理界面展示 OAuth 模式、Token capability 与重新授权提示，并提供 OpenCode 配置示例。

## 修复问题

1. 修复 Grok Build 配额和调度误判
    - 来源不明或历史导入的零配额快照只展示，不再自动暂停 Build 账号。
    - 仅 `active_probe` / `gateway_response` 的可信配额窗口参与调度。
    - Grok OAuth 测试账号必须具备 Token context，运行时冷却后会正确切换到备用账号。

2. 修复 Grok WebSocket bridge 多轮回放偶发丢失
    - SSE normalizer 现在会在终端事件写给下游前执行 observer。
    - 避免上层读到 `response.completed` 后立即关闭 pipe，导致 reasoning/tool call 未写入回放缓存。

3. 修复渠道模型定价限制被空实现绕过
    - 恢复 Gateway 与 OpenAI/Grok 调度器的 requested、channel_mapped、upstream 三种限制检查。
    - 恢复 Antigravity 默认映射、OpenAI 账号映射和 compact-only 映射后的上游模型判定。
    - 粘性会话命中受限账号时会清理绑定并选择可用账号。

4. 修复模型映射与显式白名单边界
    - `model_mapping` 默认仅负责模型改名，未命中时继续透传原模型。
    - 只有账号显式启用 `restrict_to_model_list` 时才执行白名单过滤，空列表不再意外放行。
    - Gemini、Anthropic 与 Antigravity 调度测试已同步新语义。

5. 修复分组路由绕过混合调度与强制平台限制
    - 分组只限定候选账号池，不再无条件绕过请求级平台校验。
    - 未开启 `mixed_scheduling` 的 Antigravity 账号不会被 Anthropic/Gemini 分组路由误选。
    - Legacy 路由、粘性会话、负载感知与普通选择路径使用一致的平台判定。

6. 修复协议响应桥的空响应状态码与流式事件处理
    - 无响应体的 `204` 和错误状态会显式提交到底层 ResponseWriter，不再回落为 `200`。
    - Gemini 流式桥按完整 SSE 事件边界增量转换，保留即时 flush 行为。

7. 修复 Release 与回归测试接口不一致
    - 补齐 Grok OAuth 依赖注入、Token refresh 构造参数、API contract peak rate 字段和版本测试。
    - 保留上一候选版本已修复的 Vue 外部模板 typecheck bridge、Router JSON 读取、调度器 CandidateCount 与计费语义。
    - Official API 配额探测使用 LightBridge User-Agent；Build Proxy 保持 Pager/Shell CLI 身份头。

## 优化调整

1. 强化发布供应链
    - Release workflow 使用不可变 Action SHA、固定 Go/Node/pnpm 版本与最小权限。
    - 发布前强制运行 release validator、代码清单校验、secret scan、后端 unit/integration 和前端构建。
    - Preview 构建按平台生成独立二进制与 checksums，并在成功后同步 `VERSION`。

2. 强化 Grok Build Transport 与安全边界
    - 使用独立 HTTP/2 Transport profile，避免 HTTP/2 frame 被错误按 HTTP/1.1 解析。
    - 不透传 CPA 中不可信的 Authorization、Host、Connection 等请求头。
    - 探测和持久化错误只保留稳定分类，不保存完整 Token、JWT 身份字段或原始敏感响应。

## 兼容性 / 破坏性变更

1. 现有 Grok Build OAuth 账号建议重新授权
    - 影响范围：旧 Token 可能缺少 `referrer=grok-build`，或没有经过真实 Build 探测。
    - 处理方式：升级后在账户页面执行“重新授权”，探测成功后再恢复调度。

2. Grok Build 与 Official API 配额不再混用
    - 影响范围：来源不明的旧 quota 快照不会再自动暂停 Build 账号。
    - 处理方式：通过真实请求或主动探测刷新可信配额状态。

3. 本版本不包含数据库结构迁移
    - 影响范围：无需手动执行新的 SQL migration。
    - 处理方式：正常升级应用并完成 Grok 账号重新授权检查。

## 升级说明

1. 运行环境建议与 Release CI 保持一致
    - Go `1.26.5`、Node.js `22.13.1`、pnpm `9.15.9`。
    - 升级前备份现有配置与数据库，不要把 OAuth Token、Cookie 或用户数据写入日志。

2. 升级后验证 Grok 核心链路
    - 分别验证 Responses、Messages、Chat Completions 和 WebSocket bridge。
    - 至少完成一次包含 reasoning、tool call 和 tool output 的两轮工具调用。
    - 检查 401/403/429、配额冷却、刷新与备用账号切换行为。

## 验证与发布备注

1. 本版本为 Preview
    - 用于验证 Grok Build Token context、多协议 Router、渠道限制恢复和 Release 工作流。
    - 未经真实 Grok 测试账号与生产前冒烟验证，不建议直接替换关键生产实例。
