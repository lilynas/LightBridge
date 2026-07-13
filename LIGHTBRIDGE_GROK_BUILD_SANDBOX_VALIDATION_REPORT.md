# LightBridge Grok Build Token Context — 沙盒验证报告

日期：2026-07-12
结论：Production Candidate；尚不能标记为 Production Ready。

## 1. 本次范围

本轮针对 Grok Build 上游策略变化完成：

- `build_proxy` / `official_api` 显式 OAuth 模式；
- Build 授权 URL `referrer=grok-build`；
- Access Token capability 有界检查；
- Redis 多实例 OAuth Session 与一次性消费；
- OAuth state 强制验证和 redirect/proxy 冻结；
- OAuth 创建、通用账户创建、重新授权和刷新后的真实 Build 探测；
- 401/403/资格错误停用、429 临时冷却、5xx/网络故障瞬态处理；
- Refresh 锁竞争与过期 Token 防缓存；
- CPA xAI/Grok 导入 capability 隔离；
- Build 配额来源可信度与调度规则；
- Grok Build HTTP/2 Transport Profile；
- Grok Pager/Shell 客户端身份头；
- 前端 capability/reauthorization 状态展示；
- OpenCode OpenAI-Compatible 示例。

## 2. 实际通过的检查

### Go

- `gofmt`：1,801 个 Go 文件，无未格式化文件；
- Go AST：1,801 个文件，0 个语法问题；
- 变更 Go import 静态检查：46 个文件，0 个可疑未使用 import；
- 新增测试文件均通过 AST 和 gofmt；
- 通用创建、重新授权、Refresh、CPA、配额、HTTP/2 和 Redis Session 均添加定向测试代码。

### 前端

- TypeScript/JavaScript/Vue script 语法解析：688 个单元，0 个问题；
- OpenCode 示例 JSON 可解析；
- Grok Token capability、mode、referrer 和 reauthorization 字段的 API/type/i18n 变更通过语法检查。

### 配置与发布

- 8 份 YAML 通过解析及重复键检查；
- 4 份关键 JSON 通过解析；
- Release workflow 32 个 shell block 通过 `bash -n`；
- `tools/validate_release_configuration.py`：通过；
- Release 使用固定 Node/pnpm/Corepack、不可变 Action SHA 和最小权限策略。

### 仓库与安全

以下数量是在写入本报告前完成门禁时记录的；最终打包前会再次生成 Inventory 并重新运行 Secret Scan。

- `tools/codebase_inventory.py --check`：通过；
- Inventory：2,846 个文本文件；维护代码 708,169 行，生成内容 222,620 行；
- `tools/secret_scan.py`：通过，扫描 2,847 个文件；
- 新文档和示例不包含真实 Token、私钥或账号凭据；
- OAuth/探测错误不会保存完整上游响应体或 JWT 身份字段。

## 3. 无法在沙盒完成的检查

### 原生 Go 测试

项目 `backend/go.mod` 要求 Go 1.26.5，沙盒只有 Go 1.23.2。自动工具链下载失败：

```text
go: download go1.26.5 ... lookup proxy.golang.org ... connection refused
go: go.mod requires go >= 1.26.5 (running go 1.23.2; GOTOOLCHAIN=local)
```

日志：`native-go-toolchain-check.log`。

### 临时副本定向测试

在不修改正式工程的临时副本中，将 `go` 指令暂时改为 1.23.0，并设置 `GOPROXY=off`，尝试以下包：

- `internal/pkg/xai`；
- `internal/service` Grok 定向测试；
- `internal/repository` Redis/HTTP 定向测试；
- `internal/handler/admin` OAuth/CPA/创建定向测试。

结果均因依赖模块未缓存、`module lookup disabled by GOPROXY=off` 停止。临时目录已删除，正式项目 `go.mod` 未改动。日志：`grok-build-targeted-go-test.log`。

### 未执行

- `go test ./...`、unit/integration/race/vet；
- `pnpm install --frozen-lockfile`、lint/typecheck/Vitest/build；
- PostgreSQL + Redis 双实例集成；
- 真实 xAI/Grok Build OAuth；
- 真实 `referrer=grok-build` Token 签发验证；
- Codex、Claude Code、OpenCode 两轮工具调用；
- 代理环境下 HTTP/2/ALPN；
- GitHub Actions Release E2E。

## 4. 代码级风险控制

### JWT claim

`referrer` 仅通过未验签 payload 解码用于兼容性诊断：

- 可解析 Build JWT 缺少 claim：fail-fast；
- opaque Token：交给真实上游；
- claim 正确也不直接视为授权成功；
- 真实 Build 探测是最终可用性依据。

### 探测状态

- 2xx：可用；
- 401/403/明确 entitlement：停止调度；
- 429：按 `Retry-After` 写入临时不可调度，默认 2 分钟、最大 24 小时；
- 5xx/网络错误：不永久停用；
- 只有 LightBridge 自身管理的 availability 错误允许重新授权后自动恢复。

### 最常用创建路径

前端创建弹窗通过通用 `/admin/accounts` 创建 Grok OAuth 账户。该通用 Handler 已加入真实 Build 探测，避免只修专用 `create-from-oauth` 但遗漏主入口。

## 5. 发布阻断条件

以下任一情况都必须阻断发布：

- Build OAuth 新 Token 未获得正确 referrer，或真实探测失败；
- 重新授权成功后账号仍不可调度，或鉴权失败账号被错误恢复；
- 429 后账号仍被下一次调度立即选中；
- Refresh 锁竞争返回过期 Token；
- CPA 缺少 Build context 的账号仍可调度；
- OpenCode 未使用 `@ai-sdk/openai-compatible`；
- Claude Code 工具调用第二轮缺失 usage/tool result；
- HTTP/2 出现 raw frame / malformed HTTP response；
- unit、integration、race、前端 typecheck/build 或 Release 任一失败。

## 6. 下一步

在具备 Go 1.26.5、pnpm 9.15.9、PostgreSQL、Redis 和真实 Grok Build 账号的环境中，严格执行：

`LIGHTBRIDGE_GROK_BUILD_PRODUCTION_TEST_PLAN.md`

全部 P0 通过并回传日志后，才可将包从 Production Candidate 升级为 Production Ready。
