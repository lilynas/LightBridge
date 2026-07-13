# LightBridge Grok Build Token Context 升级说明

日期：2026-07-12
状态：Production Candidate

## 1. 背景

近期 Grok Build 的实际行为表明，能否调用 `https://cli-chat-proxy.grok.com/v1` 不只取决于 Base URL 和 CLI 请求头，还取决于 OAuth 签发上下文及 Access Token 本身。可用于 Build 的 JWT Access Token可能包含：

```json
{"referrer":"grok-build"}
```

该字段不能通过修改 JWT payload 或 CPA JSON 伪造，因为修改 payload 会破坏签名。LightBridge 因此把 Grok 上游拆分为两种显式模式：

- `build_proxy`：Grok Build OAuth + CLI Proxy；
- `official_api`：官方 xAI API。

两种模式的 OAuth 上下文、请求头、Body Patch、配额语义和可用性判定不再混用。

## 2. 本次升级

### 2.1 OAuth 签发上下文

- 默认 Grok OAuth 模式为 `build_proxy`；
- Build 授权 URL 使用 `referrer=grok-build`；
- Official API 使用独立模式，不要求 Build referrer；
- OAuth 模式、PKCE verifier、redirect URI 和代理在创建 Session 后冻结；
- Session 使用 Redis TTL 存储，通过 Lua 原子 `GET + DEL` 一次性消费，支持多实例回调；
- 无论粘贴完整回调 URL 还是裸授权码，都必须校验 state。

### 2.2 Token capability

LightBridge 对 JWT payload 进行有界、未验签的兼容性检查：

- Token 最大检查长度 64 KiB；
- Payload 最大 16 KiB；
- Build JWT 中 `referrer=grok-build`：标记为 `grok_build`；
- Build JWT 可解析但缺少或错误 referrer：标记为 `incompatible`；
- opaque / 非 JWT Token：标记为 `unknown`，交由 xAI 上游最终判断；
- Official API Token：不要求 Build referrer。

注意：JWT payload 解码仅用于诊断和 fail-fast，不代表 LightBridge 验证了 xAI 的签名。最终授权结论仍由真实 xAI 上游响应决定。

### 2.3 真实可用性探测

OAuth 创建、重新授权或刷新后，LightBridge 使用最小受控请求探测真实 Build 服务：

```json
{
  "model": "grok-4.5",
  "input": ".",
  "max_output_tokens": 8,
  "stream": false
}
```

判定规则：

- `2xx`：确认当前 Token 能到达服务，必要时恢复由 LightBridge 管理的停用状态；
- `429`：进入冷却，不永久停用，也不把旧错误直接恢复；
- `401` / `403` / 明确 entitlement 或 spending-limit 资格错误：停止调度并要求重新授权；
- `5xx`、网络错误和超时：视为瞬态上游故障，不永久判死账号。

探测错误只保存稳定分类，不保存原始响应体、完整 Token 或 JWT 身份字段。

### 2.4 Token 刷新

- 刷新锁竞争时有界等待其他实例刷新结果；
- 不再返回或重新缓存已过期 Token；
- Refresh 后重新检查 Build capability；
- Build Token 丢失 `referrer=grok-build` 时立即清除 Token cache 并退出调度；
- Refresh 成功后执行真实 Build 探测；
- 只有 LightBridge 自己写入的 reauthorization/availability 错误才会自动恢复，管理员手动停用不会被误开启。

### 2.5 CPA 导入

对 `type: xai/grok` 的 CPA 文件：

- Build Token 带正确 referrer：按 Build 账户导入；
- 可解析但缺少 Build referrer：导入但标记为需要重新授权，默认不参与调度；
- opaque Token：标记为 unknown，必须通过真实探测；
- Official API 模式不要求 Build referrer；
- CPA 普通字段中手工填写 `referrer` 不能绕过 Access Token 检查；
- 不允许 CPA 文件注入 `Host`、`Connection`、`Authorization` 等任意请求头。

### 2.6 配额与调度

Grok Build 的官方 API quota 数字可能为 0，但 Build 请求仍然可用。因此：

- 来源不明、导入数据或旧快照中的 `remaining=0` 只展示，不自动停用；
- 只有来自 `active_probe` 或 `gateway_response` 且实际观察到限流头的窗口才参与自动暂停；
- 真实 `429` 与可信 `Retry-After` 仍正常进入运行时冷却；
- 401、403、429、5xx 的运行时封禁支持 Grok，不再复用仅支持 OpenAI 的守卫函数。

### 2.7 Grok Build Transport

Grok Build 使用独立 `grok_h2` Transport Profile：

- `ForceAttemptHTTP2=true`；
- 不主动设置 hop-by-hop `Connection` 请求头；
- 账户测试和探测使用原生 Go HTTP Transport；
- 不进入只适合 HTTP/1 指纹模拟的调用路径；
- 避免把 HTTP/2 frame 当作 HTTP/1.1 响应解析的 `malformed HTTP response "\\x00..."` 问题。

### 2.8 CLI 身份头

Build Proxy 请求由 LightBridge 受控生成：

```http
User-Agent: grok-pager/0.2.93 grok-shell/0.2.93 (linux; x86_64)
X-XAI-Token-Auth: xai-grok-cli
x-grok-client-identifier: grok-pager
x-grok-client-version: 0.2.93
```

这些 Header 不从不可信 CPA 文件透传。

## 3. 重新授权迁移流程

已有 Grok Build 账号升级后建议执行一次重新授权：

1. 在账户页面选择“重新授权”；
2. LightBridge 创建 `build_proxy` OAuth Session；
3. 完成 xAI 授权并粘贴回调 URL或授权码；
4. state、redirect URI、PKCE 和 Session mode 校验通过；
5. 新 Token 落库并清除各节点 Token cache；
6. LightBridge 检查 Token context；
7. 发送最小 Build 探测；
8. 探测成功后才恢复调度。

若 Token 可解析但缺少 `referrer=grok-build`，修改 Header 无效，必须重新走 Build OAuth。

## 4. 客户端兼容

- Codex：使用 OpenAI Responses 接口；
- Claude Code：使用 Anthropic Messages，由 Router 转成 Responses；
- OpenCode：xAI Provider 必须使用 `@ai-sdk/openai-compatible`，示例见 `examples/opencode-grok-build.json`；
- Grok Build 多轮工具调用依赖 reasoning + function/custom tool call replay，支持 Redis 多实例续链和本地有界回退。

## 5. 安全边界

- 不集成批量注册、Cloudflare 绕过或第三方共享 Token；
- 不修改或重签 xAI JWT；
- 不把未验签 JWT claim 当作可信身份；
- 不在日志、错误或 API 响应中返回完整 Access Token、Refresh Token、`sub`、`jti`、`principal_id`、`team_id`；
- OAuth Token URL 在依赖注入阶段强制通过 HTTPS、xAI host 和私网地址校验，错误配置会阻止服务启动。

## 6. 发布条件

本版本在云端沙盒完成了格式、AST、前端语法、YAML、Release、库存和 Secret Scan 等静态门禁，但由于沙盒没有 Go 1.26.5 及完整依赖，不能宣称全量编译和真实 Grok E2E 已通过。只有执行 `LIGHTBRIDGE_GROK_BUILD_PRODUCTION_TEST_PLAN.md` 的全部 P0 项目后，才可升级为 Production Ready。
