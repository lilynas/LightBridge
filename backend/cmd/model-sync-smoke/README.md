# Custom 模型拉取单向测试

该命令复用 LightBridge 的 `AccountTestService.FetchUpstreamSupportedModels`，只发送一次模型列表请求：

- 不启动完整 LightBridge
- 不连接或写入数据库
- 不修改账号配置
- API Key 只从环境变量读取

## OpenAI 兼容上游

```bash
cd backend
LIGHTBRIDGE_MODEL_SYNC_API_KEY='your-api-key' \
  go run ./cmd/model-sync-smoke \
  -protocol openai_chat_completions \
  -base-url https://api.example.com/v1
```

## 自定义模型列表地址

```bash
cd backend
LIGHTBRIDGE_MODEL_SYNC_API_KEY='your-api-key' \
  go run ./cmd/model-sync-smoke \
  -protocol openai_responses \
  -base-url https://api.example.com/v1 \
  -models-url https://api.example.com/internal/models
```

支持的协议：

- `openai_responses`
- `openai_chat_completions`
- `openai_embeddings`
- `anthropic_messages`
- `gemini`

本地 HTTP 测试地址需要额外传入 `-allow-http`。成功时输出解析后的模型数量、模型列表和耗时；失败时输出经过生产链路处理的错误。

如需复用账号使用的代理，可增加：

```bash
-proxy-url http://proxy.example.com:7890
```

输出只会标记是否启用代理，不会打印代理地址、代理认证信息或 API Key。
