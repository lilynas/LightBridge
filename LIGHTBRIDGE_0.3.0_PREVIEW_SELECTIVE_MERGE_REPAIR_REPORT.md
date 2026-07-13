# LightBridge 0.3.0 Preview 选择性合并与修复报告

日期：2026-07-13
工作区：`/Users/williamwang/Downloads/LightBridge-main`
目标版本：`0.3.0-preview`

## 1. 输入与合并策略

最新输入包：

- 文件：`LightBridge-0.2.80-grok-build-token-context-production-candidate-full.zip`
- SHA-256：`af0083d584b7ba0f6275cbf5c6675eb5bc53d5944287cd8926df25ec3e25eab0`
- 包内原始 `VERSION`：`0.2.71`

本次未用最新 ZIP 整包覆盖本地修复版，而是以“上一候选包 → 最新候选包”的纯增量差异进行选择性合并。对最新包中的功能增量予以吸收，对会恢复旧缺陷的代码予以拒绝，并保留本地已经验证过的修复。

## 2. 合并的主要功能

1. Grok Build / Official API 双 OAuth 模式
    - `build_proxy` 与 `official_api` 显式区分。
    - Build OAuth 使用 `referrer=grok-build`。
    - OAuth Session 冻结 mode、PKCE、redirect URI 和代理，并支持 Redis 多实例一次性消费。

2. Token context 与账户生命周期
    - 新增 Token capability、referrer、context checked time 与 reauthorization 状态。
    - OAuth 创建、重新授权和刷新后执行真实 Build 探测。
    - 401/403/资格错误、429 冷却、5xx/网络瞬态错误采用不同调度策略。
    - CPA xAI/Grok 导入按 capability 判定，不允许普通字段伪造 Build context。

3. 多协议 Router 与工具调用
    - Grok 支持 Responses、Anthropic Messages、Chat Completions 和 Responses WebSocket bridge。
    - 支持 encrypted reasoning、function/custom tool call 与 tool output 的多轮回放。
    - 回放缓存支持本地有界存储与 Redis 多实例共享。

4. 配额、Transport 与 CLI 身份
    - 仅可信 `active_probe` / `gateway_response` 配额快照参与 Build 自动暂停。
    - Grok 使用独立 HTTP/2 Transport profile。
    - Build Proxy 使用 Pager/Shell CLI 身份头，Official API 使用 LightBridge User-Agent。

5. 前端与文档
    - 账户页展示 OAuth 模式、Token capability 和重新授权状态。
    - 增加 OpenCode OpenAI-Compatible 示例。
    - 增加 Token context 升级文档、生产测试计划和专项评审文档。

## 3. 拒绝的旧回归

选择性合并时保留了以下本地修复，没有被候选包旧基线覆盖：

- 保留生产文件 `setting_handler_email.go`，不恢复错误的 `_test.go` 文件形态。
- 保留 `patchGrokResponsesBody(..., usingAPI)` 三参数调用。
- 保留 Router 测试的 `gjson.GetBytes`。
- 保留 Vue 外部模板 typecheck bridge。
- 保留调度器 CandidateCount 测试修正。
- 保留生产模式待补价、Simple 模式零成本的计费语义。
- 保留布局测试读取外部 HTML 模板。
- 保留 AccountsView 测试 wrapper 卸载。
- 保留内容审核 worker、macOS 路径及上一轮发布修复。

## 4. 本次额外发现并修复的问题

1. Official API 配额探测身份错误
    - Official API 改用 `lightbridge-grok-quota-probe/1.1`。
    - Build Proxy 继续保持 Pager/Shell CLI 身份头。

2. Grok 配额测试桩 panic
    - 补齐仓储桩 `SetError` 行为，避免 nil interface panic。

3. Grok 调度测试不符合 Token context 新约束
    - 测试账号补充 opaque Build Token。
    - 配额测试补充可信 observation source 与 observed headers。

4. WebSocket bridge reasoning 回放竞态
    - SSE observer 原实现位于事件写出之后。
    - 上层收到终端事件后立即关闭 pipe 时，observer 可能未执行，导致第二轮缺少 reasoning/tool call。
    - 已将 observer 移到事件写出之前，确保终端响应先进入回放缓存。

5. 渠道模型定价限制被空桩绕过
    - `GatewayService` 与 `OpenAIGatewayService` 的限制函数原先全部直接返回 `false`。
    - 已恢复 requested、channel_mapped、upstream 三种限制语义。
    - 已恢复 Antigravity 默认映射、OpenAI 账号映射、Grok 映射与 compact-only 上游模型解析。
    - 受限的粘性账号会清理绑定并回退到可用账号。

6. Unit / API contract 接口漂移
    - 补齐 `NewTokenRefreshService` 的 Grok OAuth / scheduler 参数位。
    - API contract 补齐 peak rate 字段。
    - 版本解析测试改用当前 `parseSemanticVersion` 实现。

7. 版本与发布文档
    - `VERSION` 更新为 `0.3.0-preview`。
    - 新增根目录与应用内 `0.3.0-preview` 版本更新说明。

8. 模型映射与白名单语义漂移
    - `model_mapping` 按新设计默认只负责模型改名，未命中时允许原模型透传。
    - 仅在账号显式启用 `restrict_to_model_list` 时执行模型列表过滤。
    - 修正空模型列表意外放行，并同步 Gateway、Gemini 与 Antigravity 测试夹具。

9. 分组路由绕过平台与混合调度限制
    - 分组候选列表原先会无条件跳过请求级平台校验。
    - 未开启 `mixed_scheduling` 的 Antigravity 账号可能被 Anthropic/Gemini 路由误选，强制平台也可能失效。
    - 已统一 Legacy 路由、粘性会话、负载感知与普通选择路径的平台判定。

10. Protocol Response Bridge 空响应状态码错误
    - Gin `Status` 只更新内部状态，无响应体时不会自动提交到底层 writer。
    - `204` 或空错误响应因此可能最终表现为 `200`。
    - 已在空响应分支调用 `WriteHeaderNow`，并修正 Gemini SSE 增量测试的完整事件边界。

11. 发布门禁稳定性
    - ChannelMonitor `StopAndWait` 测试改为显式阻塞/释放 worker，避免用固定耗时推断等待行为。
    - TLS 指纹 integration 测试将第三方 `tls.peet.ws` 的瞬时 handshake EOF 识别为外部服务不可用，避免误判代码回归。
    - Inventory 与 secret scan 排除本地 Go/Codex/Mimocode 临时缓存目录。

## 5. 已完成验证

- `go test -p 1 -tags=unit ./...`：PASS。
- `go test -p 1 -tags=integration ./...`：PASS。
- `internal/service` 3022 个顶层 unit 测试完成分批定位，并通过完整包级复跑。
- ChannelMonitor Stop 测试连续 20 次通过；Protocol Response Bridge 定向测试连续 20 次通过。
- Grok 调度 Token context、可信配额与运行时切换定向测试：PASS。
- Grok WebSocket bridge 多轮 reasoning/tool call 回放：PASS。
- 前端 ESLint：PASS。
- 前端 vue-tsc：PASS。
- 前端 Vitest：116 个测试文件、749/749 测试通过。
- 前端 production build：PASS。
- `gofmt` 与 `git diff --check`：PASS。

## 6. 发布产物与状态

- 核心源码提交：`bc59a7e`（`release: prepare v0.3.0-preview`）。
- 精简源码包：`LightBridge-0.3.0-preview-grok-build-token-context-selective-merged-full-2026-07-13.zip`。
- ZIP 大小：约 `9.8 MiB`，共 `3102` 个目录/文件条目。
- ZIP SHA-256：`b8dba3c1c654f84ea4143fbb86a917228623c02a406f9c77e87da4c24f1e5a77`。
- ZIP 完整性：PASS；已排除 `.git`、node_modules、dist/build、数据目录、缓存、Mimocode/Playwright 本地痕迹、旧 ZIP、临时报告与 inventory。
- Release configuration validator：PASS。
- Codebase inventory：PASS，索引 `2856` 个文本文件。
- Secret scan：PASS，扫描 `2864` 个文件。
- Preview Tag：`v0.3.0-preview`。
- Preview Release：`https://github.com/WilliamWang1721/LightBridge/releases/tag/v0.3.0-preview`。
- GitHub Actions 的最终 Run URL 与发布结论在交付回复中列出。
