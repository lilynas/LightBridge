# LightBridge Grok Build Token Context — Production 测试计划

日期：2026-07-12
适用包：`LightBridge-0.2.80-grok-build-token-context-production-candidate-full.zip`

## 1. 验收标准

只有以下 P0 全部通过，才能发布：

- 后端 unit、integration、race、vet 全绿；
- 前端 lint、typecheck、Vitest、build 全绿；
- PostgreSQL 与 Redis 多实例行为通过；
- Grok Build OAuth、刷新、重新授权、CPA 导入和真实探测通过；
- Codex、Claude Code、OpenCode 工具调用至少完成两轮真实调用；
- Release snapshot 与 GitHub Actions Release 全绿；
- Secret Scan、依赖安全扫描无阻断问题。

## 2. 环境

- Go：1.26.5；
- Node.js：Release workflow 指定版本；
- pnpm：9.15.9；
- PostgreSQL：与生产主版本一致；
- Redis：与生产主版本一致，开启持久化并允许 Lua；
- 两个 LightBridge 实例，共用 PostgreSQL 与 Redis；
- 一枚合法 Grok Build OAuth 账号；
- 一枚官方 xAI API 账号（用于模式隔离测试）；
- 可选：一个旧 CPA Grok 文件，Token 缺少 Build referrer；
- 不使用来源不明的共享 Token。

## 3. 完整性

```bash
sha256sum -c LightBridge-0.2.80-grok-build-token-context-production-candidate-full.zip.sha256
unzip -t LightBridge-0.2.80-grok-build-token-context-production-candidate-full.zip
```

解压后确认没有 `.git`、私钥、Token、`.env` 生产凭据。

## 4. 静态和依赖门禁

```bash
python3 tools/codebase_inventory.py --check
python3 tools/secret_scan.py
python3 tools/validate_release_configuration.py

git diff --check
find backend -name '*.go' -type f -print0 | xargs -0 gofmt -d
```

安装并执行：

```bash
actionlint
syft dir:. -o json > syft.json
grype sbom:syft.json
```

## 5. 后端

```bash
cd backend
go version
go mod download
go mod verify
go test ./...
go test -tags=unit ./...
go test -tags=integration ./...
go test -race ./internal/service/... ./internal/handler/admin/... ./internal/repository/...
go vet ./...
```

项目 Makefile 门禁：

```bash
make test-unit
make test-integration
```

### 5.1 定向测试

```bash
go test -tags=unit ./internal/pkg/xai -run 'Test(BuildAuthorizationURL|ValidateAccessToken|GrokOAuth)'
go test -tags=unit ./internal/service -run 'TestGrok(OAuth|Token|Quota|BuildAccount|Verify|ShouldAutoPause)'
go test -tags=unit ./internal/repository -run 'TestGrokOAuthSessionStore|TestHTTPUpstreamSuite'
go test -tags=unit ./internal/handler/admin -run 'Test(ApplyOAuthCredentialsGrok|VerifyGrokAccountAvailability|CPA|ImportData.*Grok)'
```

## 6. 前端

```bash
cd frontend
corepack enable
corepack prepare pnpm@9.15.9 --activate
pnpm install --frozen-lockfile
pnpm run lint:check
pnpm run typecheck
pnpm run test:run
pnpm run build
```

确认 Grok 账户页显示：

- OAuth mode；
- Token capability；
- Token referrer（脱敏或安全短值）；
- 需要重新授权的明确提示；
- 不显示 Token、`sub`、`jti`、principal/team ID。

## 7. OAuth P0

### 7.1 Build OAuth URL

创建 Grok OAuth，检查授权 URL：

- 含 `referrer=grok-build`；
- state、nonce、PKCE challenge 均存在；
- redirect URI 与 Session 中一致；
- Session mode 为 `build_proxy`。

### 7.2 state

分别粘贴：

- 完整回调 URL；
- 裸授权码。

两者都必须提供正确 state。空 state 或错误 state 必须失败，且不能兑换 Token。

### 7.3 Token context

- 带 `referrer=grok-build` 的 JWT：进入后续真实探测；
- 可解析但无 referrer：返回 `GROK_BUILD_TOKEN_CONTEXT_MISSING`，不得调度；
- 可解析但 referrer 非 `grok-build`：不得调度；
- opaque Token：允许进入真实上游探测；
- Official API Token：不要求 Build referrer。

不得通过在 CPA 普通 JSON 字段添加 `referrer` 绕过检查。

### 7.4 Session 多实例

1. 实例 A 创建 OAuth Session；
2. 回调和兑换落到实例 B；
3. 必须成功读取同一 Redis Session；
4. 同一 session 第二次消费必须失败；
5. 服务重启后 TTL 内 Session 仍有效；
6. 超过 TTL 后失效。

## 8. 重新授权与恢复

准备一个由 LightBridge 标记为 `Grok Build availability verification failed...` 且 `schedulable=false` 的账号：

1. 前端执行重新授权；
2. 新凭据原子落库；
3. Token cache 清除；
4. 真实探测返回 200；
5. 错误清除；
6. `schedulable=true`；
7. 状态恢复 active。

反例：

- 探测 401/403：仍然不可调度；
- 探测 429：保留冷却，不立即恢复旧错误账户；
- 探测 5xx：不永久停用，也不错误宣称已恢复；
- 管理员手动停用账号：重新授权不得自动开启。

## 9. Refresh 与锁竞争

### 9.1 正常刷新

- 新 Token 保留 `referrer=grok-build`；
- 更新 expires_at；
- 清除所有节点旧 Token cache；
- 真实探测成功；
- 请求继续可用。

### 9.2 capability 丢失

模拟 Refresh 返回可解析但无 Build referrer 的 JWT：

- 不缓存新 Token；
- 清旧缓存；
- 账号退出调度；
- 明确提示重新授权。

### 9.3 多实例锁竞争

实例 A 持有刷新锁，实例 B 同时请求：

- B 有界等待；
- B 重新读取缓存或数据库中的新 Token；
- B 不返回、不缓存过期 Token；
- 超时返回可诊断错误，不无限阻塞。

## 10. 真实 Build 探测

使用 `grok-4.5` 最小请求验证状态映射：

| 上游结果 | 期望 |
|---|---|
| 200 | 可用，可信配额头可保存 |
| 401 | Token 无效，停止调度 |
| 403 | Build 无资格，停止调度 |
| entitlement/spending-limit 400 | 停止调度并要求重新授权/检查订阅 |
| 429 | 临时冷却，不永久停用 |
| 500/502/503 | 瞬态故障，不永久判死 |
| timeout/DNS | 瞬态故障，不永久判死 |

日志中不得出现完整响应体、Token、email、sub、jti、principal/team ID。

## 11. CPA 导入

分别导入：

1. Build JWT + 正确 referrer；
2. Build JWT + 缺少 referrer；
3. opaque Token；
4. Official API Token；
5. JSON 普通字段伪造 referrer；
6. xAI/Grok 单账号、数组和 ZIP。

预期：

- 正确 Build Token 保持可调度并接受真实探测；
- 缺少 referrer 的可解析 JWT 导入后必须不可调度；
- opaque Token 必须经真实探测；
- Official 模式正常；
- 普通字段伪造无效；
- Token 不出现在导入错误和日志中。

## 12. 配额与冷却

- 来源不明的 `remaining=0`：只显示，不停用；
- `active_probe` / `gateway_response` 且观察到可信限流头：允许自动暂停；
- 真实 429：按 Retry-After 冷却；
- 冷却后调度器跳过账户，过期后恢复；
- 粘性会话不得绕过运行时封禁；
- 多实例共享冷却状态。

## 13. HTTP/2 与代理

直接和通过 HTTP/SOCKS 代理测试：

```text
cli-chat-proxy.grok.com → ALPN h2 → 正常 Responses/SSE
```

确认：

- 使用 `grok_h2` profile；
- 不发送 `Connection: Keep-Alive`；
- 不出现 `malformed HTTP response "\\x00..."`；
- 代理不支持 h2 时错误清晰，不能把 raw frame 当 H1；
- 客户端取消后立即取消上游。

## 14. Router 与工具调用

每个客户端执行至少两轮：模型先发起工具调用，客户端返回 tool result，模型引用结果继续回答。

### Codex / OpenAI Responses

验证：

- function/custom tool call；
- encrypted reasoning replay；
- SSE 与 WebSocket；
- 跨 Grok 账号 failover；
- previous_response_id 不被错误交给无状态 Build Proxy。

### Claude Code / Anthropic Messages

验证：

- tools schema 转换；
- tool_use / tool_result ID 对齐；
- usage 字段始终存在，避免 `usage.input_tokens` undefined；
- 流式首包实时输出，不缓存完整响应。

### OpenCode

使用 `examples/opencode-grok-build.json`，Provider 必须指定：

```json
"npm": "@ai-sdk/openai-compatible"
```

验证 shell/file 工具调用和第二轮续链。

### Built-in search

- 不默认注入 web_search；
- 未知 built-in tool 不转成客户端 function；
- 连续相同工具调用达到上限后终止；
- 不出现无限 web_search 输出。

## 15. Release

```bash
python3 tools/validate_release_configuration.py
cd backend
goreleaser check --config ../.goreleaser.yaml
goreleaser check --config ../.goreleaser.simple.yaml
goreleaser release --snapshot --clean --config ../.goreleaser.yaml
```

随后创建测试 Tag，验证 GitHub Actions：

- Tag checkout SHA 与 Tag commit 一致；
- unit/integration、前端 typecheck/test/build 均执行；
- Actions 固定不可变 SHA；
- 默认权限只读，只有 Release Job 有 contents write；
- artifact 名称带 Tag；
- GoReleaser 不修改 go.mod/go.sum；
- Release 链接、二进制和校验和完整。

## 16. 结果回传格式

```text
环境：
Commit/ZIP SHA256：
Go/Node/pnpm/PostgreSQL/Redis 版本：

P0 总计：
通过：
失败：
阻塞：

失败命令：
完整日志路径：
复现步骤：
实际结果：
预期结果：
是否稳定复现：
相关账号模式：build_proxy / official_api
上游状态码与脱敏分类：
```
