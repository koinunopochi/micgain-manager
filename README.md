# Mic Gain Manager

macOSのマイク入力音量を自動で固定し続けるツールです。ZoomやTeamsなどのビデオ会議アプリケーションが勝手に音量を変更してしまう問題を解決します。

## このツールについて

本ツールは、設定した音量レベルを定期的に強制適用することで、外部アプリケーションによる意図しない音量変更を防ぎます。ブラウザから操作できるWeb UIとコマンドラインインターフェースの両方を提供しており、用途に応じて使い分けることができます。

単一の実行ファイルとして動作し、特別なインストール作業は必要ありません。設定はJSONファイルとして保存され、CLIとWeb UIの両方から同じ設定を参照・変更できます。

> **注意**: このツールはmacOSの`osascript`コマンドを使用して音量を制御します。そのため、macOS上で直接実行する必要があり、Dockerなどのコンテナ環境では動作しません。

## クイックスタート

### ビルド方法

まず、`go-task`がインストールされていない場合はインストールします。

```bash
brew install go-task
```

次に、プロジェクトをビルドします。

```bash
task build
```

ビルドが成功すると、`dist/micgain-manager`に実行ファイルが生成されます。

### 起動方法

用途に応じて3つの起動方法があります。

**Web UIとスケジューラを同時に起動する（推奨）**

最も一般的な使い方です。Web UIで設定を変更しながら、バックグラウンドで音量を自動維持します。

```bash
./dist/micgain-manager serve
```

ブラウザで http://127.0.0.1:7070 を開くと、Web UIにアクセスできます。

**スケジューラのみをバックグラウンドで起動する**

設定変更はCLIで行い、音量の自動維持だけを実行したい場合に使用します。

```bash
./dist/micgain-manager daemon
```

**CLIで設定と即時適用を行う**

スケジューラを起動せず、設定変更と即時適用だけを行いたい場合に使用します。

```bash
# 音量を70%に設定して即座に適用
./dist/micgain-manager config set --volume 70 --apply-now

# 現在の設定を確認
./dist/micgain-manager config get
```

## コマンドリファレンス

### daemon

スケジューラのみを起動します。設定ファイルに記載されたインターバルごとに音量を自動で元に戻します。Web UIは起動しません。

```bash
./dist/micgain-manager daemon
```

このコマンドは、バックグラウンドプロセスとして常時起動させたい場合に適しています。設定の変更はCLIまたは設定ファイルの直接編集で行います。

### web

Web UIのみを起動します。スケジューラは起動しないため、音量の自動維持機能は動作しません。

```bash
./dist/micgain-manager web --addr 127.0.0.1:7070
```

設定の確認や変更だけを行いたい場合に使用します。`--addr`オプションでリスニングアドレスとポートを指定できます。

### serve

Web UIとスケジューラの両方を起動します。通常はこのコマンドを使用することを推奨します。

```bash
./dist/micgain-manager serve --addr 127.0.0.1:7070
```

Web UIで設定を変更しながら、バックグラウンドで音量を自動維持します。`--addr`オプションでリスニングアドレスとポートを指定できます。

### config get

現在の設定内容を表示します。

```bash
./dist/micgain-manager config get
```

出力例:

```json
{
  "targetVolume": 70,
  "intervalSeconds": 90,
  "enabled": true,
  "lastApplyStatus": "ok",
  "lastApplied": "2025-10-29T12:34:56+09:00"
}
```

### config set

設定を変更します。複数のオプションを組み合わせて使用できます。

```bash
# 音量を80%、インターバルを45秒に設定
./dist/micgain-manager config set --volume 80 --interval 45s

# 設定後すぐに適用
./dist/micgain-manager config set --volume 80 --apply-now

# スケジューラを無効化
./dist/micgain-manager config set --enabled false
```

`--apply-now`オプションを指定すると、設定保存と同時に音量が即座に適用されます。

### apply

現在の設定値または指定した音量を即座に適用します。設定ファイルは変更されません。

```bash
# 設定ファイルの値で適用
./dist/micgain-manager apply

# 一時的に50%に変更（設定ファイルは更新されない）
./dist/micgain-manager apply --volume 50
```

一時的に異なる音量を試したい場合に便利です。

### shell

対話型シェルを起動します。繰り返しコマンドを実行する場合に便利です。

```bash
./dist/micgain-manager shell
```

シェル内では、通常のコマンドを直接入力できます。また、`log`コマンドでログレベルを動的に変更できます。

```
micgain> config get
micgain> config set --volume 75 --apply-now
micgain> log -vvv
micgain> exit
```

シェル内で使用できる特別なコマンド：

- `help`: 利用可能なコマンド一覧を表示
- `log -v`, `log -vv`, `log -vvv`: ログレベルを変更
- `log --show`: 現在のログレベルを表示
- `exit` または `quit`: シェルを終了

プロンプト文字列は`--prompt`オプションでカスタマイズできます。

```bash
./dist/micgain-manager shell --prompt "mgain> "
```

## 実用例

### 会議中に音量を固定する

会議の前に音量を設定し、デーモンとして起動します。

```bash
# 音量を70%に設定して即座に適用
./dist/micgain-manager config set --volume 70 --apply-now

# バックグラウンドでデーモンを起動
./dist/micgain-manager daemon &

# 会議終了後にプロセスを停止
pkill micgain-manager
```

### Web UIで管理する

ブラウザから設定を管理したい場合は、serveコマンドで起動します。

```bash
./dist/micgain-manager serve
```

ブラウザで http://127.0.0.1:7070 を開くと、GUIで設定を変更できます。

### macOS起動時に自動実行する

LaunchAgentを使用して、macOS起動時に自動的にデーモンを起動できます。

`~/Library/LaunchAgents/com.micgain.manager.plist`に以下の内容で保存します。

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.micgain.manager</string>
    <key>ProgramArguments</key>
    <array>
        <string>/path/to/micgain-manager</string>
        <string>daemon</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
</dict>
</plist>
```

保存後、以下のコマンドで登録します。

```bash
launchctl load ~/Library/LaunchAgents/com.micgain.manager.plist
```

## Web API

Web UIを起動している場合、HTTP APIを通じてプログラムから設定を操作できます。

### エンドポイント

| エンドポイント | メソッド | 説明 |
|--------------|---------|------|
| `/api/config` | GET | 現在の設定と状態を取得 |
| `/api/config` | PUT | 設定を更新 |
| `/api/apply` | POST | 即座に音量を適用 |

### 使用例

設定を取得する:

```bash
curl http://127.0.0.1:7070/api/config
```

音量を75%に変更して即座に適用する:

```bash
curl -X PUT http://127.0.0.1:7070/api/config \
  -H "Content-Type: application/json" \
  -d '{"targetVolume": 75, "applyNow": true}'
```

現在の設定で即座に適用する:

```bash
curl -X POST http://127.0.0.1:7070/api/apply
```

## 設定ファイル

設定はJSON形式で保存されます。デフォルトの保存先は`~/.config/micgain-manager/config.json`です。

```json
{
  "targetVolume": 50,
  "intervalSeconds": 90,
  "enabled": true,
  "lastApplyStatus": "ok",
  "lastApplied": "2025-10-29T10:30:00+09:00",
  "lastError": ""
}
```

### パラメータの説明

**targetVolume**: 維持する音量レベル（0-100の整数値）。デフォルトは50です。

**intervalSeconds**: 音量を適用する間隔（秒単位）。デフォルトは90秒です。

**enabled**: スケジューラの有効/無効を設定します。`false`に設定すると、スケジューラは動作しません。

**lastApplied**: 最後に音量が適用された日時（ISO 8601形式）。

**lastApplyStatus**: 最後の適用結果。`never`、`ok`、`error`のいずれか。

**lastError**: エラーが発生した場合のエラーメッセージ。正常時は空文字列。

## アーキテクチャ

本プロジェクトは、ヘキサゴナルアーキテクチャ（ポート&アダプタパターン）を採用しています。ビジネスロジックをドメイン層に集約し、外部システムとの接続をアダプタ層で抽象化することで、保守性とテスタビリティを高めています。

### ディレクトリ構成

```
internal/
  domain/              # ドメイン層（ビジネスロジック）
    entity.go          # Config, ScheduleState エンティティ
    service.go         # SchedulerService（純粋関数）
    repository.go      # ポート定義（インターフェース）

  usecase/             # ユースケース層
    scheduler.go       # SchedulerUseCase実装

  adapter/
    primary/           # プライマリアダプタ（入力）
      cli/             # CLIコマンド実装
      web/             # Web API実装
    secondary/         # セカンダリアダプタ（外部システム）
      volume/          # osascript音量制御実装
      repository/      # JSON永続化実装
```

### 依存関係

すべての依存がドメイン層に向かう設計になっています。

```
CLI → usecase → domain ← repository
Web → usecase → domain ← volume
```

この構造により、外部システムの変更がビジネスロジックに影響を与えにくくなっています。

## トラブルシューティング

### 音量が変わらない

macOSの権限設定を確認してください。初回実行時に権限を求めるダイアログが表示されることがあります。システム環境設定からターミナルやアプリケーションに必要な権限が付与されているか確認してください。

### "osascript failed"エラーが表示される

以下の点を確認してください。

まず、macOS上で実行しているかを確認します。DockerやVMなどの仮想環境では動作しません。

次に、システム環境設定の「セキュリティとプライバシー」から「プライバシー」タブを開き、必要な権限が付与されているか確認してください。

### Web UIにアクセスできない

ファイアウォール設定やポート番号を確認してください。デフォルトでは`127.0.0.1:7070`でリスニングしています。

別のポートを使用したい場合は、`--addr`オプションでポート番号を指定できます。

### 設定が保存されない

`~/.config/micgain-manager/`ディレクトリへの書き込み権限を確認してください。ディレクトリが存在しない場合は自動的に作成されますが、親ディレクトリに書き込み権限が必要です。

## 開発

### ビルド

開発用のビルドは以下のコマンドで実行できます。

```bash
task build
```

### テスト実行

テストを実行する場合は、以下のコマンドを使用します。

```bash
go test ./...
```

### プロジェクト構造

```
.
├── cmd/
│   └── micgain-manager/     # アプリケーションのエントリーポイント
├── internal/
│   ├── domain/              # ドメインロジック
│   ├── usecase/             # ユースケース層
│   ├── adapter/             # アダプタ層
│   │   ├── primary/         # 入力アダプタ
│   │   └── secondary/       # 出力アダプタ
│   └── logging/             # ログ機能
└── Taskfile.yml             # タスク定義ファイル
```

## ライセンス

MIT License (c) 2025 koinunopochi

## 参考資料

本プロジェクトのアーキテクチャは、以下の記事を参考にしています。

- [ヘキサゴナルアーキテクチャ - nrslib.com](https://nrslib.com/hexagonal-architecture/)
