# Grok 对照实现融合审查

日期：2026-07-12
对照对象：`LightBridge-0.2.80-grok-phase3-full.zip` 及其交付清单、测试计划和校验摘要
融合基线：LightBridge 0.2.80 Grok / Router 当前候选工作树

## 1. 审查原则

本次没有把任一版本整包覆盖到另一版本之上，而是按文件内容哈希定位差异，再逐项判断：

1. 是否符合 LightBridge 现有 Handler / Service / Repository / Router 分层；
2. 是否支持 PostgreSQL、Redis 和多实例部署；
3. 是否保持真正的流式传输、客户端取消和背压；
4. 是否扩大 OAuth、Token、SSRF 或跨租户数据泄漏风险；
5. 是否能通过独立回归测试证明，而不是只依赖说明文档；
6. 是否会改变非 Grok 平台现有行为。

对照 Phase 3 与当前基线有 2,786 个文件内容完全一致，差异集中在 76 个路径，因此采用精确融合而不是重构或覆盖。

## 2. 已吸收的设计

| 对照实现优点 | 融合结果 | 最终实现说明 |
| --- | --- | --- |
| OAuth / setup-token 账户允许空名称 | 已吸收并扩展 | 显式名称优先；其次读取 credentials/extra 中多种 email 字段、ID Token 或 Access Token JWT、subject/account ID，最后使用确定性平台回退名称；非 OAuth/API Key 账户仍强制名称。 |
| Replay 使用更保守的资源边界 | 已吸收 | 30 分钟 TTL、最多 2,048 scope、每 scope 最多 32 item、序列化记录最多约 2 MiB；同时保留 Redis 分布式缓存与有界本地回退。 |
| CPA xAI/Grok 导入字段覆盖更完整 | 已吸收并与现有导出统一 | 支持 `type: xai/grok`、Token、subject、email、token endpoint、base URL、`using_api`、订阅层级以及多种过期时间格式；Grok OAuth 默认并发为 1。 |
| 外置 Vue template 导致 TS6133 | 已吸收 | 通过显式 binding 让严格 TypeScript 编译器识别模板中使用的声明，不关闭 `noUnusedLocals`，不降低类型严格度。 |
| Corepack 与精确 pnpm 版本 | 已吸收 | Release 使用 Node 22.13.1、Corepack、pnpm 9.15.9，安装仍使用 frozen lockfile。 |
| GitHub Actions 最小权限 | 已吸收 | 工作流默认 `contents: read`；仅正式发布、预览发布和 VERSION 同步 Job 获得所需写权限。 |
| 第三方 Action 固定不可变提交 | 已吸收并重新核验 | Release 中所有外部 Action 固定到对应官方主版本标签当前解析出的 40 位 Commit SHA。 |
| 测试计划强调完整候选验收 | 已吸收并扩展 | 测试计划新增 Code Review 八项缺陷、真流式首包延迟、多实例 OAuth、官方/Build 请求分层和 Release 权限固定的专项用例。 |

## 3. 未直接采用的设计

| 对照实现设计 | 未采用原因 | 最终替代方案 |
| --- | --- | --- |
| 单进程内存 reasoning replay | 多实例、进程重启和账号故障转移时状态丢失；无法满足网关横向扩展 | Redis 分布式 replay + 有界本地 fallback；缓存键包含租户、分组、模型和会话/响应维度；无效密文可清毒。 |
| 生产路径使用 `httptest.ResponseRecorder` 做协议转换 | 完整缓存 SSE，客户端无法及时收到首事件；内存随响应增长；取消和背压语义变弱 | 生产用增量 `http.ResponseWriter` 转换桥，按 SSE 事件边界解析、转换、写出并 Flush；只有非流式响应才有界缓冲。 |
| 所有 xAI 模式共享同一请求补丁 | Build workaround 可能无意删除官方 API 合法字段 | `PatchOfficialXAIResponsesRequest` 与 `PatchGrokBuildResponsesRequest` 分离；官方模式仅做必要模型处理，Build 模式才执行 CLI 兼容清洗。 |
| OAuth session 保存在进程内 map | 回调落到另一实例或服务重启后 session 丢失 | Redis TTL Store；Lua 原子 GET+DEL 实现一次性消费。 |
| 只依赖数据库临时不可调度状态 | 调度快照尚未更新时仍可能立即重选故障账号 | OpenAI/Grok 共用运行时 fast-path block，并保留数据库状态作为持久层。 |
| Token 刷新锁竞争后继续使用旧 Token | 可能返回并再次缓存已经过期的 access token | 有界等待新缓存、重新读取数据库；若仍过期则明确失败，不缓存旧 Token。 |
| 整包覆盖式合并 | 容易覆盖当前版本更强的安全、流式和多实例能力 | 按哈希和调用链逐文件融合，每项带定向测试。 |

## 4. 本轮额外修复

在融合对照实现的同时，针对独立 Code Review 确认的问题完成：

1. Grok OAuth Token URL 在依赖注入阶段强制通过 HTTPS、xAI 域名和私网地址校验，错误时启动失败；
2. Grok 401/403/429/5xx 临时冷却立即进入运行时调度封禁；
3. 跨协议 SSE 转换不再缓存完整响应；
4. 裸授权码同样必须提交并验证 OAuth state；
5. 兑换阶段不能覆盖创建 session 时冻结的 proxy 与 redirect URI；
6. Refresh lock race 不再回退到过期 Token；
7. OAuth session 支持多实例与服务重启窗口内恢复；
8. Grok 上游返回 SSE 时统一使用 `actualStream`；
9. 删除手动 `Connection: Keep-Alive` hop-by-hop header；
10. 仓库中人工维护 Go 文件重新执行全量 gofmt 门禁。

## 5. 生产判定

融合后的代码是 **Production Candidate**，不是未经运行时验证即可发布的 Production Final。必须由本地 Agent 在 Go 1.26.5、Node 22.13.1、pnpm 9.15.9、PostgreSQL、Redis、Docker 和真实/Mock xAI 环境中完成生产测试计划中的全部 P0 门禁后，才能提升为 Production Ready。
