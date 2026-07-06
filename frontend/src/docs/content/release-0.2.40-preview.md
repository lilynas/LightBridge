# 0.2.40-preview 版本更新

## 新增功能

1. Grok 平台接入预览
    - 新增 Grok 平台账号、OAuth/Refresh Token 创建、令牌刷新、额度拉取与网关转发链路。
    - 账号创建、重授权、账号列表、平台图标、平台标签、模型白名单与额度展示支持 Grok。
    - 新增 Grok 平台迁移与模型定价数据，便于在预览版中验证完整调度与计费用例。

2. 账号导入导出增强
    - 新增 authconv 转换链路，支持 Codex、Codex Manager、codex2api、sub2api、Session 与 CPA 等格式互转。
    - 账号导入弹窗支持 JSONL、多文件解析、格式覆盖和兼容模式。
    - 新增导出格式弹窗，便于将管理员账号数据导出到不同外部格式。

3. 用量与运维可观测性增强
    - 用户用量页趋势图、模型图和日志统计支持 API Key 筛选与请求来源切换。
    - 运维系统日志补充 API Key 维度，便于定位具体调用来源。
    - 管理仪表盘与图表组件补充 upstream、mapping、requested 等模型来源统计能力。

4. 平台级额度能力扩展
    - 默认平台额度、用户平台额度弹窗和展示组件支持 Custom 平台。
    - 用户平台额度保存会保留后端返回的未知平台，避免全量替换时误删自定义 Provider 限额。

## 修复问题

1. 修复 Codex/OpenAI 网关上下文与协议兼容问题。
    - `store=false` 场景仅移除不可复用的 `rs_*` reasoning 引用，保留可安全续链的 reasoning 内容。
    - 修复 Codex 工具回传、WebSocket 转发、图像生成桥接和模型别名相关的兼容细节。

2. 修复 authconv 数据保真问题。
    - 非 native 导出正确处理账号过期时间的秒/毫秒时间戳。
    - 导入导出链路保留 `chatgpt_user_id`，避免跨格式迁移后账号身份匹配丢失。

3. 修复导入 UI 与实际行为不一致的问题。
    - JSONL 文件不再按单个 JSON 解析。
    - “格式覆盖”会实际传入转换器，而不是继续自动识别。

4. 修复 Custom 平台额度前端覆盖不完整的问题。
    - 设置页、用户 quota 弹窗、用户端展示、仪表盘统计和类型定义统一覆盖 Custom 平台。
    - 管理员保存用户额度时不会清掉已有自定义平台额度。

5. 修复 Grok Refresh Token 能力只存在于 API 层的问题。
    - 创建账号 UI 已接入 Grok Refresh Token 校验和批量创建流程。
    - 补齐中英文界面文案。

## 优化调整

1. 账号与渠道管理体验优化
    - 账号使用量、账号筛选、重授权、模型白名单和平台展示组件补充新平台与来源字段。
    - 账号表格和用户表格的分页大小记忆更加稳定。

2. 错误透传与调度策略优化
    - 错误透传规则支持更多平台枚举。
    - Antigravity 请求转换、调度快照和 token 刷新链路补充更多兼容保护。

3. 文档与操作说明同步
    - LightBridge Router、管理员渠道和系统设置文档补充本次预览版相关说明。

## 兼容性 / 破坏性变更

1. 本次预览版包含数据库迁移。
    - 影响范围：运维系统日志 API Key 维度、Grok 平台账号和额度数据。
    - 处理方式：升级后请确保后端迁移正常执行；如使用手动迁移流程，请应用 `153_ops_system_logs_add_api_key_id.sql` 和 `154_grok_platform.sql`。

2. 账号导入导出链路会保留更多身份字段。
    - 影响范围：导出的外部格式可能新增 `chatgpt_user_id` 等字段。
    - 处理方式：下游工具如未识别这些字段，可忽略不影响原有 token 使用。

## 升级说明

1. 升级到 `0.2.40-preview` 后建议验证 Grok 平台配置。
    - 在账号管理中创建 Grok OAuth 或 Refresh Token 账号。
    - 检查模型白名单、额度刷新、用量统计和网关转发是否符合预期。

2. 已有 Custom 平台额度会继续保留。
    - 管理员进入用户 quota 弹窗保存时，不会再因为前端平台枚举缺失而清空自定义平台限额。

## 验证与发布备注

1. 已验证前端类型检查。
    - `pnpm --dir frontend typecheck`

2. 已验证本次前端关键回归。
    - `pnpm --dir frontend test:run src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts src/components/user/__tests__/UserPlatformQuotaCell.spec.ts src/api/__tests__/settings.authSourceDefaults.spec.ts src/views/admin/__tests__/SettingsView.spec.ts src/utils/__tests__/authconv.spec.ts src/__tests__/integration/data-import.spec.ts`

3. 已验证代码格式与空白。
    - `git diff --check`
