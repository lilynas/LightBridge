# LightBridge 项目开发指南

> 本文档记录项目环境配置、常见坑点和注意事项，供 Claude Code 和团队成员参考。

## 一、项目基本信息

| 项目 | 说明 |
|------|------|
| **上游仓库** | WilliamWang1721/LightBridge |
| **Fork 仓库** | bayma888/LightBridge-bmai |
| **技术栈** | Go 后端 (Ent ORM + Gin) + Vue3 前端 (pnpm) |
| **数据库** | PostgreSQL 16 + Redis |
| **包管理** | 后端: go modules, 前端: **pnpm**（不是 npm） |

## 二、本地环境配置

### PostgreSQL 16 (Windows 服务)

| 配置项 | 值 |
|--------|-----|
| 端口 | 5432 |
| psql 路径 | `C:\Program Files\PostgreSQL\16\bin\psql.exe` |
| pg_hba.conf | `C:\Program Files\PostgreSQL\16\data\pg_hba.conf` |
| 数据库凭据 | user=`LightBridge`, password=`LightBridge`, dbname=`LightBridge` |
| 超级用户 | user=`postgres`, password=`postgres` |

### Redis

| 配置项 | 值 |
|--------|-----|
| 端口 | 6379 |
| 密码 | 无 |

### 开发工具

```bash
# golangci-lint v2.7
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7

# pnpm (前端包管理)
npm install -g pnpm
```

## 三、CI/CD 流水线

### GitHub Actions Workflows

| Workflow | 触发条件 | 检查内容 |
|----------|----------|----------|
| **backend-ci.yml** | push, pull_request | 单元测试 + 集成测试 + golangci-lint v2.7 |
| **security-scan.yml** | push, pull_request, 每周一 | govulncheck + gosec + pnpm audit |
| **release.yml** | tag `v*` | 构建发布（PR 不触发） |

### CI 要求

- Go 版本必须是 **1.25.7**
- 前端使用 `pnpm install --frozen-lockfile`，必须提交 `pnpm-lock.yaml`

### 本地测试命令

```bash
# 后端单元测试
cd backend && go test -tags=unit ./...

# 后端集成测试
cd backend && go test -tags=integration ./...

# 代码质量检查
cd backend && golangci-lint run ./...

# 前端依赖安装（必须用 pnpm）
cd frontend && pnpm install
```

## 四、模块化开发硬规则

LightBridge 的长期方向是微内核 + 可下载模块。开发时必须按下面规则切分，避免把模块化做成“核心内置代码 + 开关隐藏”的伪模块化。

模块化专题文档入口：

| 文档 | 用途 |
|------|------|
| `docs/modules/README.md` | 模块化总览、Core 边界、生命周期、第一阶段开发顺序 |
| `docs/modules/package-spec.md` | 模块包目录、`module.yaml`、checksum/signature、状态机、人工验包清单 |
| `docs/modules/backend-plugin-protocol.md` | sidecar runtime、ProviderAdapter RPC、GatewayRequest/Event、CoreBridge |
| `docs/modules/frontend-extension-protocol.md` | UI manifest、动态路由、菜单、账号表单 contribution、remote 失败兜底 |
| `docs/modules/provider-sdk.md` | provider 模块开发教程、mock provider、本地打包安装和 streaming 验证 |
| `docs/modules/migrations-and-data.md` | 模块 migration 规则、私有表命名、卸载和 purge 数据策略 |
| `docs/modules/security-permissions.md` | permissions 声明、审批、secret 读取、审计和 MVP sandbox 边界 |
| `docs/modules/testing.md` | core no-provider、模块安装、sidecar crash、remote UI failure 测试矩阵 |
| `docs/modules/runbooks.md` | 安装失败、签名失败、sidecar 启动失败、迁移失败、provider unhealthy 排查 |

推荐执行顺序固定为：

```text
module schema
  -> installer
  -> supervisor
  -> provider protocol
  -> provider registry
  -> UI manifest
  -> sample/mock provider
```

任何实现变更如果改变接口名、状态名、API 路径、表名、错误码或模块包字段，必须同步更新 `docs/modules/` 对应专题文档。

### 1. Core 只保留稳定内核

- Core 可以定义协议、扩展点、生命周期、权限声明、审计和调度流程。
- Core 不允许新增 provider 专属分支，例如 `if provider == "openai"`、`if provider == "anthropic"`。
- Core 网关只面向统一的 `ProviderAdapter`、routing policy、gateway hook 等抽象。
- 新 provider 必须通过模块安装、manifest 声明、sidecar adapter 注册和动态 UI 挂载接入。

### 2. Provider 代码必须在模块中

新增 provider 时，最小交付物是：

- `module.yaml`
- `checksums.txt`
- provider sidecar backend
- provider settings frontend remote
- manifest 引用的 migrations
- provider adapter 协议测试

不要在核心前端预置 provider 配置页；provider settings 必须来自 `/api/v1/modules/ui-manifest` 和动态 remote module。

### 3. 模块包必须自描述且可校验

模块包必须使用 `lightbridge-module-<module-id>-<version>.tar.zst` 格式，并且必须包含 `module.yaml`、`checksums.txt` 与 `signature.sig`。`module.yaml` 中声明的 `backend.command`、`frontend.entry`、`migrations` 文件必须真实存在于包内，并且全部出现在 `checksums.txt` 中。

模块市场第一阶段使用静态 JSON registry。Core 从 `modules.marketplace_registry_path` 或 `modules.marketplace_registry_url` 读取 registry；如果两者都配置，本地 path 优先。registry entry 必须声明 `id`、`version`、`type`、`core`、`downloadUrl`、`capabilities`，并且 capability 必须在 MVP allowlist 内。

`downloadUrl` 支持本地路径、`file://`、`http://`、`https://`。registry `sha256` 校验的是整个归档包字节；包内 `checksums.txt` 校验的是展开后的模块文件。marketplace 安装顺序固定为：读取 registry → 校验 entry → 下载/复制归档 → 可选 registry SHA256 → 进入包安装器 → 校验 `module.yaml`/`checksums.txt`/`signature.sig`/manifest 引用文件/迁移边界。

当前 checksum 行格式固定为：

```text
sha256 <hex> <relative-path>
```

路径必须是模块包内相对路径，不能是绝对路径，也不能逃逸到 `../`。

`signature.sig` 必须签名原始 `checksums.txt` 字节。Core 通过 `modules.signature_public_key_path` 配置受信 Ed25519 公钥；未配置或签名不匹配时，模块安装必须失败。

### 4. 数据库边界

- 模块自有表必须使用模块私有前缀，例如 `provider_openai_*`。
- 不要修改已经应用或已经发布的 migration。需要修正时新增 migration，并在提交说明里写清楚原因。
- 模块迁移不能直接修改核心表，除非有明确的核心扩展点和 migration review。
- 安装器会静态检查 migration SQL 中的 `CREATE/ALTER/DROP TABLE`、`CREATE INDEX`、`COMMENT ON TABLE/COLUMN`、`INSERT/UPDATE/DELETE/TRUNCATE`、`REFERENCES` 目标表；任何不匹配 `permissions.database` 前缀的表都必须拒绝安装。
- `core.compatible` 是强约束。服务启动时注入当前 `BuildInfo.Version`，安装器必须拒绝不兼容 core 版本的模块包。
- provider 实例统一写入 `ai_provider_instances`，不要重新引入旧名 `provider_instances`。
- `Disable` 只停 runtime 和 UI/API 注册，保留数据。
- `Uninstall` 停 runtime，安全删除 `<modules.data_dir>/modules/<module-id>/<version>` 下的模块文件，保留数据库数据和 migration 记录。
- `Purge` 必须是显式用户动作；它先删除 manifest `permissions.database` 前缀匹配的模块私有表，再删除模块文件并标记为 `purged`。Purge 不能删除核心表。

### 5. 权限与安全

- `permissions` 是 declaration / approval / audit 的基础，不代表已经完成进程级沙箱。
- MVP 阶段必须记录模块声明了哪些 network、secrets、database、ui、gateway 权限。
- 任何读取 secret、发起外部网络、挂载登录流程、注册 gateway hook 的能力都必须先在 manifest 中声明。
- 模块之间不能直接互相调用，也不能直接读取其他模块数据；跨模块协作必须通过 Core Service API。

### 6. 后端接入规则

- sidecar 是第一阶段唯一允许的 backend kind。
- Core 通过 runtime 启动 sidecar，并把 sidecar 包装成核心内部的 adapter。
- gateway/core 只能依赖 `ProviderRegistry.Resolve()` 得到统一 adapter，不能知道具体 provider 实现。
- 模块 provider 账号必须显式使用 `type=module`，新账号应使用 `platform=module`，并同时提交 `provider_id`、`extra.provider_id`、`extra.module_id`。
- `platform=module` 只是账号分类标记，不能当作 provider ID。缺少 `provider_id` 的模块账号必须报错。
- 模块账号解析不到已注册 provider 时必须返回 provider-module 错误，不能回落到旧 Claude/Anthropic/OpenAI 分支。
- 当前 Ent schema 已有 `accounts.provider_id`，但生成代码若未更新，仓储层必须继续以 `extra.provider_id` 作为兼容 source of truth。只有 `go generate ./ent` 成功生成 `ProviderID` / `SetProviderID` 后，才能改为强列读写。
- 如果 Wire 生成因为网络、代理或依赖下载失败无法运行，可以手动同步 `wire_gen.go`，但必须在提交说明或开发记录里写清楚原因。

### 7. 前端接入规则

- 主前端只保留 Shell、基础登录、基础设置、模块市场、模块管理和动态路由挂载器。
- 模块菜单、后台页面、账号配置表单必须来自 UI manifest。
- 不允许为了某个 provider 在核心 sidebar、router、views 中写死页面。
- provider frontend remote 必须暴露 manifest 中声明的 `exposedModule`。

### 8. 第一阶段范围

第一阶段只把 provider 模块 MVP 打通：

- 模块安装、签名校验、checksum 校验、migration runner、权限声明/审批。
- marketplace registry 读取、local/file/http/https 包下载、registry archive SHA256 校验。
- sidecar provider runtime、启动恢复、runtime 状态与 stdout/stderr 日志。
- `ProviderRegistry`、模块 provider gateway bridge、模块账号测试 bridge。
- UI manifest、动态后台路由、动态 provider 账号表单挂载。
- 模块账号契约：`platform=module`、`type=module`、`provider_id`、`extra.provider_id`、`extra.module_id`。

下面内容不是第一阶段完成条件：

- 发布密钥轮换与远程 registry trust policy
- Auth extension / 2FA / Passkey
- 把所有旧 provider 代码从 Core 中删除
- 进程级网络与文件系统沙箱

### 9. Provider 模块验证命令

优先跑小范围验证，避免在当前大包上触发长时间无输出的包加载：

```bash
cd backend
go test ./internal/modules -count=1 -timeout=90s
go test ./internal/repository -run 'TestModuleStore' -count=1 -timeout=90s
go test ./internal/service -run 'TestProviderModuleBridge' -count=1 -timeout=90s
go generate ./ent
```

如果 `go test ./internal/repository`、`go test ./internal/service` 或 `go generate ./ent` 超过 30 秒没有任何输出，先终止并记录为环境/包加载阻塞，不要继续并行启动新的 Go 命令。当前 provider bridge 可以先依赖 `extra.provider_id` 兼容层继续开发。

## 五、常见坑点 & 解决方案

### 坑 1：pnpm-lock.yaml 必须同步提交

**问题**：`package.json` 新增依赖后，CI 的 `pnpm install --frozen-lockfile` 失败。

**原因**：上游 CI 使用 pnpm，lock 文件不同步会报错。

**解决**：
```bash
cd frontend
pnpm install  # 更新 pnpm-lock.yaml
git add pnpm-lock.yaml
git commit -m "chore: update pnpm-lock.yaml"
```

---

### 坑 2：npm 和 pnpm 的 node_modules 冲突

**问题**：之前用 npm 装过 `node_modules`，pnpm install 报 `EPERM` 错误。

**解决**：
```bash
cd frontend
rm -rf node_modules  # 或 PowerShell: Remove-Item -Recurse -Force node_modules
pnpm install
```

---

### 坑 3：PowerShell 中 bcrypt hash 的 `$` 被转义

**问题**：bcrypt hash 格式如 `$2a$10$xxx...`，PowerShell 把 `$2a` 当变量解析，导致数据丢失。

**解决**：将 SQL 写入文件，用 `psql -f` 执行：
```bash
# 错误示范（PowerShell 会吃掉 $）
psql -c "INSERT INTO users ... VALUES ('$2a$10$...')"

# 正确做法
echo "INSERT INTO users ... VALUES ('\$2a\$10\$...')" > temp.sql
psql -U LightBridge -h 127.0.0.1 -d LightBridge -f temp.sql
```

---

### 坑 4：psql 不支持中文路径

**问题**：`psql -f "D:\中文路径\file.sql"` 报错找不到文件。

**解决**：复制到纯英文路径再执行：
```bash
cp "D:\中文路径\file.sql" "C:\temp.sql"
psql -f "C:\temp.sql"
```

---

### 坑 5：PostgreSQL 密码重置流程

**场景**：忘记 PostgreSQL 密码。

**步骤**：
1. 修改 `C:\Program Files\PostgreSQL\16\data\pg_hba.conf`
   ```
   # 将 scram-sha-256 改为 trust
   host    all    all    127.0.0.1/32    trust
   ```
2. 重启 PostgreSQL 服务
   ```powershell
   Restart-Service postgresql-x64-16
   ```
3. 无密码登录并重置
   ```bash
   psql -U postgres -h 127.0.0.1
   ALTER USER LightBridge WITH PASSWORD 'LightBridge';
   ALTER USER postgres WITH PASSWORD 'postgres';
   ```
4. 改回 `scram-sha-256` 并重启

---

### 坑 6：Go interface 新增方法后 test stub 必须补全

**问题**：给 interface 新增方法后，编译报错 `does not implement interface (missing method XXX)`。

**原因**：所有测试文件中实现该 interface 的 stub/mock 都必须补上新方法。

**解决**：
```bash
# 搜索所有实现该 interface 的 struct
cd backend
grep -r "type.*Stub.*struct" internal/
grep -r "type.*Mock.*struct" internal/

# 逐一补全新方法
```

---

### 坑 7：Windows 上 psql 连 localhost 的 IPv6 问题

**问题**：psql 连 `localhost` 先尝试 IPv6 (::1)，可能报错后再回退 IPv4。

**建议**：直接用 `127.0.0.1` 代替 `localhost`。

---

### 坑 8：Windows 没有 make 命令

**问题**：CI 里用 `make test-unit`，本地 Windows 没有 make。

**解决**：直接用 Makefile 里的原始命令：
```bash
# 代替 make test-unit
go test -tags=unit ./...

# 代替 make test-integration
go test -tags=integration ./...
```

---

### 坑 9：Ent Schema 修改后必须重新生成

**问题**：修改 `ent/schema/*.go` 后，代码不生效。

**解决**：
```bash
cd backend
go generate ./ent  # 重新生成 ent 代码
git add ent/       # 生成的文件也要提交
```

---

### 坑 10：前端测试看似正常，但后端调用失败（模型映射被批量误改）

**典型现象**：
- 前端按钮点测看起来正常；
- 实际通过 API/客户端调用时返回 `Service temporarily unavailable` 或提示无可用账号；
- 常见于 OpenAI 账号（例如 Codex 模型）在批量修改后突然不可用。

**根因**：
- OpenAI 账号编辑页默认不显式展示映射规则，容易让人误以为“没映射也没关系”；
- 但在**批量修改同时选中不同平台账号**（OpenAI + Antigravity/Gemini）时，模型白名单/映射可能被跨平台策略覆盖；
- 结果是 OpenAI 账号的关键模型映射丢失或被改坏，后端选不到可用账号。

**修复方案（按优先级）**：
1. **快速修复（推荐）**：在批量修改中补回正确的透传映射（例如 `gpt-5.3-codex -> gpt-5.3-codex-spark`）。
2. **彻底重建**：删除并重新添加全部相关账号（最稳但成本高）。

**关键经验**：
- 如果某模型已被软件内置默认映射覆盖，通常不需要额外再加透传；
- 但当上游模型更新快于本仓库默认映射时，**手动批量添加透传映射**是最简单、最低风险的临时兜底方案；
- 批量操作前尽量按平台分组，不要混选不同平台账号。

---

### 坑 11：PR 提交前检查清单

提交 PR 前务必本地验证：

- [ ] `go test -tags=unit ./...` 通过
- [ ] `go test -tags=integration ./...` 通过
- [ ] `golangci-lint run ./...` 无新增问题
- [ ] `pnpm-lock.yaml` 已同步（如果改了 package.json）
- [ ] 所有 test stub 补全新接口方法（如果改了 interface）
- [ ] Ent 生成的代码已提交（如果改了 schema）

## 六、常用命令速查

### 数据库操作

```bash
# 连接数据库
psql -U LightBridge -h 127.0.0.1 -d LightBridge

# 查看所有用户
psql -U postgres -h 127.0.0.1 -c "\du"

# 查看所有数据库
psql -U postgres -h 127.0.0.1 -c "\l"

# 执行 SQL 文件
psql -U LightBridge -h 127.0.0.1 -d LightBridge -f migration.sql
```

### Git 操作

```bash
# 同步上游
git fetch upstream
git checkout main
git merge upstream/main
git push origin main

# 创建功能分支
git checkout -b feature/xxx

# Rebase 到最新 main
git fetch upstream
git rebase upstream/main
```

### 前端操作

```bash
# 安装依赖（必须用 pnpm）
cd frontend
pnpm install

# 开发服务器
pnpm dev

# 构建
pnpm build
```

### 后端操作

```bash
# 运行服务器
cd backend
go run ./cmd/server/

# 生成 Ent 代码
go generate ./ent

# 运行测试
go test -tags=unit ./...
go test -tags=integration ./...

# Lint 检查
golangci-lint run ./...
```

## 七、项目结构速览

```
LightBridge-bmai/
├── backend/
│   ├── cmd/server/          # 主程序入口
│   ├── ent/                 # Ent ORM 生成代码
│   │   └── schema/          # 数据库 Schema 定义
│   ├── internal/
│   │   ├── handler/         # HTTP 处理器
│   │   ├── service/         # 业务逻辑
│   │   ├── repository/      # 数据访问层
│   │   └── server/          # 服务器配置
│   ├── migrations/          # 数据库迁移脚本
│   └── config.yaml          # 配置文件
├── frontend/
│   ├── src/
│   │   ├── api/             # API 调用
│   │   ├── components/      # Vue 组件
│   │   ├── views/           # 页面视图
│   │   ├── types/           # TypeScript 类型
│   │   └── i18n/            # 国际化
│   ├── package.json         # 依赖配置
│   └── pnpm-lock.yaml       # pnpm 锁文件（必须提交）
└── .claude/
    └── CLAUDE.md            # 本文档
```

## 八、参考资源

- [上游仓库](https://github.com/WilliamWang1721/LightBridge)
- [Ent 文档](https://entgo.io/docs/getting-started)
- [Vue3 文档](https://vuejs.org/)
- [pnpm 文档](https://pnpm.io/)
