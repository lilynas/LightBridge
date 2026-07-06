# 0.2.60 版本更新

## 修复问题

1. 修复系统更新后 OpenAI OAuth 账号被误迁移为 Gemini OAuth 的问题
    - 模块迁移现在会识别 `chatgpt_account_id`、`chatgpt_user_id`、`chatgpt_plan_type`、OpenAI `id_token` claims、`plus/team/enterprise/edu/k12` 等 OpenAI OAuth 指纹。
    - 即使历史数据或导入数据的外层 `platform/type` 被误标为 `gemini` / `gemini_oauth`，只要凭据证明是 OpenAI OAuth，就会按 OpenAI 模块账号迁移。
    - 迁移时同步保留 `session_token`、`organization_id`、`plan_type`、`subscription_expires_at`、`email` 等 OpenAI OAuth 元数据，避免账号类型正确但套餐/组织字段缺失。

2. 修复 CRS 同步覆盖 OpenAI OAuth 平台的问题
    - Gemini OAuth 同步分支不再无条件把同一 `crs_account_id` 的已有账号写成 `gemini`。
    - 已存在的 OpenAI OAuth 账号会保持 OpenAI 平台；如果 CRS Gemini 分组里携带 OpenAI OAuth 指纹，也会按 OpenAI 处理。
    - 预览分类同步使用同一平台判断逻辑，避免同步前后显示的平台不一致。

3. 修复 OpenAI Responses 路径账号能力筛选不准确的问题
    - Responses、Messages 与 Responses WebSocket 入站现在按 `responses` 文本端点能力筛选账号。
    - `responses` 与 `chat_completions` 作为文本类 OpenAI 能力互通，避免支持 Responses 的自定义上游在 Chat Completions / Messages 转发链路中被误排除。
    - `openai_responses`、`openai-chat-completions` 等协议值会归一为对应 OpenAI 能力，减少配置别名导致的调度失败。

## 优化调整

1. OpenAI OAuth 迁移判定更稳健
    - OpenAI `id_token` 会以 JWT payload 方式解析，不再依赖普通字符串匹配。
    - `plan_type` 属于弱指纹，仅在识别到 OpenAI 套餐值并伴随 OAuth token 时用于判定，避免误伤 Gemini / Antigravity OAuth。
    - 保留 Gemini `project_id`、`oauth_type`、`tier_id` 等指纹优先级，确保真正的 Gemini Code Assist / Google One 账号仍保持 Gemini 平台。

2. Router 与 OpenAI 能力调度回归覆盖增强
    - 补充 Router 消息协议矩阵测试，确认 router 模式允许跨消息协议转换，passthrough / full_passthrough 仍按协议一致性过滤。
    - 补充 MIMO / Custom OpenAI Responses 账号调度测试，防止 Responses 能力账号再次被误认为 Chat-only 或被提前过滤。

## 兼容性 / 破坏性变更

1. 本次正式版不包含数据库结构迁移
    - 影响范围：无需手动执行新的 SQL migration。
    - 处理方式：正常升级应用即可。

2. 已被历史版本写坏的平台字段不会自动批量回滚
    - 影响范围：如果账号已经在数据库中变成 `platform=gemini`，本次修复会阻止后续继续写坏，但不会无条件改写历史数据。
    - 处理方式：升级后建议检查 OpenAI OAuth 账号；如发现历史损坏账号，需要按凭据指纹进行一次性数据修复。

## 升级说明

1. 升级到 `0.2.60` 后建议检查 OAuth 账号平台
    - 重点检查携带 `chatgpt_account_id`、`chatgpt_plan_type`、`plan_type=plus/team`、OpenAI `id_token` 的账号是否仍为 OpenAI。
    - 如使用 CRS 同步，建议同步前先通过预览确认 OpenAI OAuth 不再显示为 Gemini OAuth。

2. 升级后建议验证 Custom OpenAI Responses 上游
    - 对 MIMO / OpenAI Responses 兼容上游，确认账号能力配置包含 `responses` 或对应协议别名。
    - Chat Completions、Messages、Responses 入站都应能调度到合适的文本类 OpenAI 上游。

## 验证与发布备注

1. 已验证模块迁移回归
    - `go test ./internal/modulemigration`

2. 已验证后端服务包
    - `go test ./internal/service`

3. 已验证 CRS OAuth 平台保护
    - `go test ./internal/service -run 'TestCRSGeminiOAuthTargetPlatform'`
