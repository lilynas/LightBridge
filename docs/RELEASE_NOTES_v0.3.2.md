# LightBridge 0.3.2 正式版

## Release 清单确定性修复

- 代码清单现在优先只扫描 Git 跟踪的发布源文件，不再把本地忽略文件或未跟踪报告写入发布清单。
- 在不包含 `.git` 元数据的源码归档中保留文件系统回退，并统一按仓库相对路径排序，确保不同环境生成完全一致的清单。
- 修复 `v0.3.1` 在 `Verify repository inventory and committed secrets` 阶段因 clean checkout 与本地工作区内容不同而失败的问题。

## 包含的 Grok Builder 修复

- 完整包含 `v0.3.1` 的 Grok Builder Tool Calling 协议兼容修复。
- 补齐 Responses function call、reasoning 输出项的流式生命周期事件，并保持索引、item ID 与 call ID 稳定一致。
- 保留严格客户端要求的零值索引和空字段，同时加入 Grok Build CLI 客户端识别与回归测试。

## 发布验证范围

- 工作区与无 Git 元数据的干净发布归档均通过代码清单校验。
- 已提交密钥扫描与 Release 配置校验通过。
- `apicompat` 和 Router 客户端识别关键回归测试通过。
