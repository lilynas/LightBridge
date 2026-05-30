<div align="center">

# LightBridge

**AI API トラフィックのための、モジュール型で軽量なリバースプロキシブリッジ。**

[![Status](https://img.shields.io/badge/status-active%20development-2ea44f)](#ロードマップ)
[![Built on](https://img.shields.io/badge/built%20on-Sub2API-00ADD8)](#謝辞)
[![Architecture](https://img.shields.io/badge/architecture-modular-7c3aed)](#アーキテクチャ)
[![License](https://img.shields.io/badge/license-see%20LICENSE-blue)](LICENSE)

[English](README.md) | [简体中文](README_CN.md) | 日本語

</div>

---

## 概要

LightBridge は、主に
[Sub2API](https://github.com/Wei-Shaw/sub2api)
をベースにし、複数のオープンソースのリバースプロキシ実装から得た知見を組み合わせて改善する AI API リバースプロキシサービスです。

巨大なオールインワンプラットフォームを目指すのではなく、安定したゲートウェイコア、組み合わせやすい Provider アダプター、実用的なプラグイン、そして現代的な管理 UI に焦点を当てています。個人開発者、小規模チーム、自ホスト運用者が、自分の AI API Bridge をより少ない負担で構築・拡張・運用できることを目標としています。

## LightBridge の特徴

- **モジュール型設計**: ゲートウェイコア、Provider、認証、ルーティング、ログ、統計、UI を分離して拡張しやすくします。
- **軽量で自然な組み合わせ**: Sub2API や他のリバースプロキシの実績ある設計を活かしつつ、コアを肥大化させません。
- **豊富なプラグイン**: Provider、OAuth、2FA、統計、課金、レート制御、自動化などをプラグインで追加できます。
- **現代的な UI**: Provider 設定、ルーティング、リクエストログ、サービス状態を扱う管理画面を提供します。
- **OpenAI 互換を重視**: 既存の SDK、CLI、IDE プラグイン、Agent ツールから接続しやすい API サーフェスを目指します。

## 主な機能

### ゲートウェイとプロトコル

- OpenAI 互換の下流 API エントリポイント。
- 上流 AI サービスへのリバースプロキシ転送。
- Provider ごとのリクエスト/レスポンス変換レイヤー。
- チャット、Agent、CLI ツール向けのストリーミング対応設計。
- ヘッダー、セッション、アカウントコンテキストを扱うための基盤。

### Provider とアカウント管理

- 複数 Provider、複数アカウントグループの管理。
- API Key や OAuth 形式の上流認証パターン。
- Provider ごとの設定、ヘルス状態、有効/無効制御。
- モデル、Provider、優先度、重み、明示的なクライアント指定によるルーティング。
- アカウント隔離、専用プロキシ、リスク制御戦略のための拡張余地。

### プラグイン

- 新しい AI サービスや外部リバースプロキシ向けの Provider プラグイン。
- OAuth、Passkey、TOTP 2FA などの認証プラグイン。
- 使用量分析、クォータ、請求フック、監視パネルなどの運用プラグイン。
- リクエストフィルター、ミドルウェア、ポリシー、ルーティング拡張。
- モジュールの配布、検証、インストール、起動停止、更新を行う Marketplace 形式の仕組み。

### 運用

- 日常管理のための Admin Dashboard。
- リクエストメタデータログと基本的な使用量統計。
- レート制限、同時実行制限、クォータを考慮したルーティング。
- ローカル開発、サーバーデプロイ、コンテナ運用への対応方針。
- 再現可能なデプロイ、オンライン更新、可観測性の強化を計画中。

## アーキテクチャ

```text
Client / SDK / CLI / IDE Plugin
        |
        v
OpenAI-compatible API
        |
        v
LightBridge Gateway Core
        |
        +--> Auth and API Key Layer
        +--> Routing and Scheduling Layer
        +--> Plugin Runtime
        +--> Logs, Metrics, Quotas
        |
        v
Provider Adapters
        |
        +--> Sub2API-compatible upstream flows
        +--> OAuth subscription accounts
        +--> API-key upstream providers
        +--> Third-party reverse-proxy integrations
```

LightBridge はゲートウェイコアを小さく保ち、Provider 固有の処理をアダプターとプラグインに分離します。これにより、下流 API の互換性を維持しながら、上流プロトコル、アカウント隔離、プロキシ戦略、機能モジュールを柔軟に拡張できます。

## 現在の状態

LightBridge は活発に開発中です。この README は、現在のプロジェクト方針と目標となるユーザー向け機能を説明しています。

- Sub2API に着想を得たリバースプロキシ基盤。
- より軽量でモジュール化されたサービス境界。
- Provider、認証、監視、統計、自動化のためのプラグインエコシステム。
- 自ホスト運用に適した現代的な管理 UI。

安定版リリースまでは、API、モジュール仕様、デプロイ手順、ディレクトリ構成が変わる可能性があります。本番環境ではバージョンを固定し、アップグレード前に変更履歴を確認してください。

## クイックスタート

安定したインストール手順は整理中です。正式なリリース成果物やイメージが公開されるまでは、利用しているブランチの開発ガイドとデプロイファイルに従ってください。

想定される自ホスト手順:

```bash
# 1. Clone
git clone <your-lightbridge-repository-url>
cd LightBridge

# 2. Configure environment
cp .env.example .env

# 3. Start with Docker Compose or local development scripts
docker compose up -d
```

クライアント設定例:

```text
Base URL: http://localhost:<port>/v1
API Key:  <LightBridge client key>
```

## ロードマップ

| Stage | Focus | Status |
| --- | --- | --- |
| 0.1 | コアリバースプロキシ、OpenAI 互換 API、基本 Provider ルーティング | 進行中 |
| 0.2 | Sub2API 互換レイヤー、Provider/アカウント隔離、リクエスト変換パイプライン | 計画中 |
| 0.3 | プラグインランタイム、モジュールパッケージ、Provider Marketplace | 計画中 |
| 0.4 | 現代的な管理 UI、ログ、ヘルスチェック、ルーティング制御 | 計画中 |
| 0.5 | クォータ、レート制限、使用量分析、請求フック | 計画中 |
| 0.6 | 本番デプロイ文書、Docker イメージ、アップグレード戦略 | 計画中 |
| 1.0 | 安定 API 契約、プラグイン SDK、長期互換ポリシー | 計画中 |

## リポジトリ構成

目標とする構成:

```text
LightBridge/
  backend/       ゲートウェイコア、Provider アダプター、永続化、サービス
  frontend/      管理ダッシュボードとユーザー向け管理 UI
  deploy/        デプロイスクリプト、コンテナ設定、サービス例
  docs/          ガイド、リファレンス、アーキテクチャノート
  assets/        ロゴ、スクリーンショット、プロジェクトメディア
```

## 開発方針

- コアは小さく、安定して、組み合わせやすく保つ。
- プロトコル差分は明示的な Provider Adapter に閉じ込める。
- ルーティング、クォータ、認証、ログ、レート制御を運用上の主要機能として扱う。
- プラグイン境界は明確で、テスト可能で、置き換え可能にする。
- ユーザーが明示的に有効化しない限り、機密性の高いリクエスト本文を記録しない。
- Provider 固有の挙動と制限は、実装に近い場所で文書化する。

## セキュリティ

LightBridge は上流認証情報、OAuth Token、API Key、ユーザートラフィックを扱う可能性があります。本番運用では以下を推奨します。

- HTTPS の利用。
- 強力な管理者認証とクライアントキーの定期ローテーション。
- Admin Dashboard へのアクセス制限。
- データベース、設定ファイル、シークレットファイルの適切な権限管理。
- プラグインをインストールする前の出所、権限、コード確認。
- デバッグ目的以外ではリクエスト本文ログを無効にする。

## 謝辞

LightBridge は主に Sub2API の設計と実装パターンを参考にし、複数のオープンソースのリバースプロキシプロジェクトからも学んでいます。信頼できる既存の挙動を活かしながら、よりモジュール化され、軽量で、拡張しやすく、運用しやすいサービスにすることを目指しています。

上流コードを再利用、移植、改変する場合は、元プロジェクトのライセンスと attribution を保持してください。

## ライセンス

[LICENSE](LICENSE) を参照してください。
