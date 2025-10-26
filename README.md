# Mic Gain Manager

macOSのマイク入力音量を「ユーザーが明示的に変更したとき以外は固定したい」というニーズ向けの、ホストネイティブなロックツールです。ポート＆アダプタ構成でCLIとWeb UIを提供し、同一の設定ファイルを共有します。

> **重要**: 入力音量の変更は`osascript`でmacOSに直接指示する必要があるため、ツールは必ずホスト(macOS)上で実行してください。コンテナや仮想環境内では実際の音量を操作できません。

## 主な特徴
- Configストア (JSON) を介してCLIとWeb UIが同じデフォルト値・現在値を共有
- スケジューラが一定間隔で`set volume input volume <N>`を実行し、第三者アプリによる自動調整を打ち消す
- 設定変更時に「今すぐ適用 / 次のスケジュールまで待つ」を選択可能
- Web UIは単一バイナリ内蔵の静的ページで、ブラウザから即時実行・監視が可能
- CobraベースのCLIコマンド (serve / config get|set / apply / shell) で`--help`補助付きの操作が可能
- 単体バイナリをホストに配置するだけで利用でき、追加のコンテナ環境は不要

## 使い方

### 1. バイナリ実行 (macOSホスト)
```bash
# まず task build で dist/micgain-manager を生成
task build

# サーバー(Web UI + REST + スケジューラ)起動
MICGAIN_CFG=~/.config/micgain-manager/config.json ./dist/micgain-manager serve --config "$MICGAIN_CFG" --addr 127.0.0.1:7070
```
ブラウザで http://127.0.0.1:7070/ にアクセスし、音量やインターバルを更新できます。

### 2. CLIで設定/即時適用
```bash
# 現在の設定を確認
./dist/micgain-manager config get

# 音量80%、インターバル45秒に変更し即時適用
./dist/micgain-manager config set --volume 80 --interval 45s --apply-now

# 一時的に50%へ即時適用 (設定値は書き換えない)
./dist/micgain-manager apply --volume 50
```

### 3. 対話型シェル
繰り返し操作するときは `shell` サブコマンドが便利です。ターミナル内で `micgain>` プロンプトが立ち上がり、`config set --volume 70` などを直接入力できます。
```bash
./dist/micgain-manager shell -v        # -v や --config など通常のグローバルフラグも利用可
# シェル内で利用できる主なコマンド:
#   serve --addr 0.0.0.0:7070
#   config get
#   apply --volume 40
#   log -vvvv        <- ログをトレースレベルまで引き上げ
#   log --show       <- 現在のログレベルを確認
#   exit / quit      <- シェル終了
```
`log -v`, `log -vvv` のように `-v` を増やすほどログが詳細になります（0=warn, -v=info, -vv=debug, -vvv/-vvvv=trace）。通常のサブコマンド実行時も `-v` を付ければ同様に反映されます。

### 4. Taskfile経由でビルド/実行
`go-task` をまだ導入していない場合は `brew install go-task/tap/go-task` 等でセットアップしてください。
```bash
# dist/micgain-manager にビルド
task build

# (ADDRを環境変数で上書きしつつ) ビルド→サーバー起動
ADDR=0.0.0.0:8080 task serve

# 対話型シェルをビルド後に起動 (ARGS環境変数で追加フラグ指定可)
ARGS="-v --prompt 'mgain> '" task shell

# 生成物の掃除
task clean
```

## Web API
- `GET /api/config` : 現在の設定と最後の適用状況、次回予定時刻を返す
- `PUT /api/config` : `targetVolume`, `intervalSeconds`, `enabled`, `applyNow` を更新
- `POST /api/apply` : 直ちに現在値で適用

## 構成/アーキテクチャ
- `internal/core` : Configロード/保存とスケジューラを司るユースケース層
- `internal/adapters/volume` : macOS向け`osascript`実装 (将来別アダプタへ差し替え可)
- `internal/web` : REST + UIのアダプタ層
- `cmd/micgain-manager` : CLIエントリーポイント (port)

## FAQ
**Q. どの設定が初期値として使われる？**  
A. `~/.config/micgain-manager/config.json` が既定の保存先で、初回起動時は「音量50%、間隔90秒、スケジューラ有効」です。CLI/Webどちらからでも同じ値を読み書きします。

**Q. 再生アプリが音量を上書きし続ける場合の対策は？**  
A. サーバーを起動しておけばインターバルごとに強制的に戻します。さらに`config set --apply-now`などで必要に応じて手動リセットも可能です。

**Q. apply-nowで失敗するときは？**  
A. `osascript`出力とエラーをCLI/Web両方で返しています。権限ダイアログがmacOS側に表示されていないか確認してください。

## ライセンス
TODO: 必要に応じて追記してください。
