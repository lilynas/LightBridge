<div align="center">

# LightBridge

**一个模块化、轻量、优雅的 AI API 反代理桥接服务。**

[![状态](https://img.shields.io/badge/status-active%20development-2ea44f)](#开发路线图)
[![基于](https://img.shields.io/badge/built%20on-Sub2API-00ADD8)](#致谢)
[![架构](https://img.shields.io/badge/architecture-modular-7c3aed)](#架构设计)
[![许可](https://img.shields.io/badge/license-see%20LICENSE-blue)](LICENSE)

[English](README.md) | 简体中文 | [日本語](README_JA.md)

</div>

---

## 简介

LightBridge 是一个主要基于
[Sub2API](https://github.com/Wei-Shaw/sub2api)，并结合多个开源反代理项目思路改进而来的 AI API 反代理服务。

它的目标不是做一个臃肿的一体化平台，而是在稳定的网关核心之上，以更轻量的方式组合上游账号、协议转换、路由调度、插件扩展和现代化管理界面，让个人开发者、小团队和自托管服务维护者可以更轻松地搭建自己的 AI API Bridge。

## 项目定位

LightBridge 关注四件事：

- **模块化设计**：网关核心、Provider 适配器、认证、路由、日志、统计和 UI 功能尽量解耦。
- **轻量优雅的组合**：吸收 Sub2API 和其他反代理项目的成熟设计，但避免把所有能力堆进核心。
- **功能丰富的插件**：通过插件扩展 Provider、OAuth、二次验证、统计、计费、限流和自动化能力。
- **现代化 UI 界面**：提供更清晰的管理后台，用于配置 Provider、查看请求、管理路由、监控服务状态。

## 核心功能

### 网关与协议

- 提供 OpenAI 兼容的下游 API 入口，方便现有 SDK、CLI、IDE 插件和 Agent 工具接入。
- 支持上游 AI 服务的反向代理转发。
- 支持面向不同 Provider 的请求/响应转换层。
- 面向聊天、Agent、CLI 工具的流式响应场景设计。
- 支持请求头、会话信息和账号上下文保留，为粘性会话和账号隔离打基础。

### Provider 与账号管理

- 支持多 Provider、多账号组的接入模型。
- 支持 API Key、OAuth 等上游认证方式。
- 支持 Provider 级别配置、健康状态和启停控制。
- 支持按模型、Provider、优先级、权重或客户端显式选择进行路由。
- 为一号一代理、账号隔离、风控隔离等场景预留扩展空间。

### 插件系统

- Provider 插件：扩展新的 AI 服务或第三方反代理服务。
- 认证插件：扩展 OAuth、Passkey、TOTP 2FA 等登录/授权能力。
- 统计插件：扩展用量分析、额度管理、账单导出和监控面板。
- 策略插件：扩展请求过滤、限流、并发控制、模型别名和路由规则。
- 模块市场：规划支持插件打包、发布、校验、安装、启停和升级。

### 运维能力

- 管理后台用于日常配置和状态查看。
- 请求元数据日志和基础用量统计。
- 速率限制、并发限制和额度感知路由。
- 支持本地开发、服务器部署和容器化部署路径。
- 后续规划可重复部署、在线升级和更完善的观测能力。

## 架构设计

```text
客户端 / SDK / CLI / IDE 插件
        |
        v
OpenAI 兼容 API 入口
        |
        v
LightBridge Gateway Core
        |
        +--> 认证与 API Key 层
        +--> 路由与调度层
        +--> 插件运行时
        +--> 日志、统计、额度与限流
        |
        v
Provider Adapters
        |
        +--> Sub2API 兼容上游流程
        +--> OAuth 订阅账号
        +--> API Key 上游 Provider
        +--> 第三方反代理项目集成
```

LightBridge 会尽量保持核心网关稳定，把 Provider 特定逻辑放到适配器和插件中。这样既能保持下游 API 的兼容性，也能更灵活地实验上游协议、账号隔离、代理策略和功能模块。

## 当前状态

LightBridge 处于积极开发阶段。当前 README 描述的是项目最新定位和目标产品形态：

- 以 Sub2API 为主要基础的反代理服务。
- 更轻、更模块化的服务边界。
- 更丰富的 Provider、认证、监控、统计和自动化插件生态。
- 更现代的自托管管理界面。

在稳定版本发布前，API、模块协议、部署命令和目录结构都可能调整。生产部署前请固定版本，并仔细阅读对应版本的更新记录。

## 快速开始

稳定安装方式正在整理中。在正式发布安装包和镜像前，请以当前分支中的开发文档和部署文件为准。

预期自托管流程：

```bash
# 1. 克隆仓库
git clone <your-lightbridge-repository-url>
cd LightBridge

# 2. 配置环境变量
cp .env.example .env

# 3. 使用 Docker Compose 或本地开发脚本启动
docker compose up -d
```

客户端接入通常类似：

```text
Base URL: http://localhost:<port>/v1
API Key:  <LightBridge client key>
```

## 开发路线图

| 阶段 | 重点 | 状态 |
| --- | --- | --- |
| 0.1 | 核心反代理、OpenAI 兼容 API、基础 Provider 路由 | 进行中 |
| 0.2 | Sub2API 兼容层、Provider/账号隔离、请求转换管线 | 计划中 |
| 0.3 | 插件运行时、模块打包、Provider Marketplace | 计划中 |
| 0.4 | 现代化管理后台、日志、健康检查、路由控制 | 计划中 |
| 0.5 | 额度、限流、用量统计、计费 Hook | 计划中 |
| 0.6 | 生产部署文档、Docker 镜像、升级策略 | 计划中 |
| 1.0 | 稳定 API 合约、插件 SDK、长期兼容策略 | 计划中 |

## 目录规划

目标仓库结构如下：

```text
LightBridge/
  backend/       网关核心、Provider 适配器、持久化和业务服务
  frontend/      管理后台和用户侧管理界面
  deploy/        部署脚本、容器配置、服务示例
  docs/          使用指南、参考文档和架构说明
  assets/        Logo、截图和项目媒体资源
```

## 开发原则

- 保持核心小而稳定。
- 用清晰的 Provider Adapter 承载协议差异。
- 把路由、额度、认证、日志和限流作为一等运维能力。
- 插件边界要清晰、可测试、可替换。
- 默认不记录敏感请求正文，除非用户明确启用调试。
- Provider 特定行为和限制必须靠近实现位置记录清楚。

## 安全说明

LightBridge 可能处理上游账号、OAuth Token、API Key 和用户请求流量。生产部署建议：

- 使用 HTTPS。
- 使用强管理员密码，并定期轮换客户端 API Key。
- 限制管理后台访问范围。
- 对数据库、配置文件和密钥文件设置合适权限。
- 安装插件前审查来源、权限和代码。
- 除非排障需要，不要开启请求正文日志。

## 致谢

LightBridge 主要参考和继承了 Sub2API 的设计与实践，同时也吸收了多个开源反代理项目的成熟思路。项目目标是在保留可靠能力的基础上，让整体架构更模块化、更轻量、更容易扩展，也更适合长期自托管运维。

如果复用、移植或改造上游代码，请保留对应项目的原始协议和署名要求。

## 许可证

见 [LICENSE](LICENSE)。
