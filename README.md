# KazokuCal

KazokuCal は、ブラウザで月間の家族カレンダーを編集し、A4 横向き PDF として出力する SPA です。参照写真のような、印刷して手書き追記しやすい大きな日付・曜日ヘッダ・余白・複数日にまたがる横長予定を持つレイアウトを生成します。

## 機能

- 年、月、週始まり（月曜/日曜）の選択
- 日本の祝日表示
- パパ/ママのテレワーク表示
- 日別予定の追加、編集、削除
- 個人予定、グループ予定、全体共有予定の同時表示
- 黒、赤、青の予定色
- 複数日にまたがる横長の帯予定
- A4 横向き PDF のブラウザ表示とダウンロード
- MySQL/MariaDB によるサーバー側自動保存
- JSON の読み書き
- リバースプロキシ向けヘッダ認証
- subpath 配信対応

## 技術スタック

- フロントエンド: Svelte、Flowbite Svelte、TypeScript、Vite
- バックエンド: Go、Gin
- データベース: MySQL/MariaDB
- PDF 生成: Python、ReportLab、holidays
- コンテナ: Debian 13 slim ランタイム

## 起動方法

```bash
docker compose up --build
```

起動後、ブラウザで次にアクセスします。

```text
http://localhost:28080
```

Compose では開発用 MariaDB も同時に起動します。MariaDB はホスト側 `23306` に公開されます。

## 環境変数

| 変数 | 既定値 | 説明 |
| --- | --- | --- |
| `PORT` | `8080` | HTTP 待受ポート |
| `AUTH_ENABLED` | `false` | `true` の場合、認証ヘッダが必須 |
| `AUTH_USER_HEADER` | `X-Forwarded-User` | ユーザー名ヘッダ |
| `AUTH_EMAIL_HEADER` | `X-Forwarded-Email` | メールアドレスヘッダ |
| `AUTH_GROUPS_HEADER` | `X-Forwarded-Groups` | グループヘッダ。`,` または `;` 区切り |
| `AUTHORIZED_GROUPS` | 空 | 認識対象のグループ一覧。空の場合は header の全グループを許可 |
| `APP_BASE_PATH` | 空 | `/kazokucal` などの subpath。空の場合は root 配信 |
| `STATIC_DIR` | `/app/static` | ビルド済みフロントエンド配置先 |
| `PDFGEN_SCRIPT` | `/app/pdfgen/generate_calendar.py` | PDF 生成スクリプト |
| `HOLIDAY_SCRIPT` | `/app/pdfgen/list_holidays.py` | 祝日 JSON 生成スクリプト |
| `DB_DSN` | なし | MySQL/MariaDB 接続 DSN。Compose では自動設定 |

## subpath 配信

`APP_BASE_PATH` に `/kazokucal` のようなパスを設定すると、SPA と API をその配下で配信します。

```yaml
services:
  calendar-app:
    environment:
      APP_BASE_PATH: "/kazokucal"
```

この場合のアクセス URL は `http://localhost:28080/kazokucal/` です。`/kazokucal` へのアクセスは `/kazokucal/` にリダイレクトします。

フロントエンドは相対パスで API を呼び出すため、API は `/kazokucal/api/...` として利用されます。リバースプロキシで subpath を削らずにバックエンドへ転送してください。

## ヘッダ認証

`AUTH_ENABLED=false` の場合、ローカル開発用として認証なしで利用できます。

`AUTH_ENABLED=true` の場合、`AUTH_USER_HEADER` で指定したヘッダが空のリクエストは拒否されます。`GET /api/me` は次の形式で現在ユーザー情報を返します。

```json
{
  "authenticated": true,
  "user": "alice",
  "email": "alice@example.com",
  "groups": ["family", "admin"]
}
```

`AUTHORIZED_GROUPS` を設定すると、`AUTH_GROUPS_HEADER` に含まれる値のうち、この一覧に含まれない group は無視されます。

## API

`APP_BASE_PATH` が空の場合:

- `GET /api/healthz`
- `GET /api/me`
- `GET /api/holidays?year=2026`
- `GET /api/calendar?year=2026&month=4`
- `PUT /api/calendar`
- `POST /api/pdf`

`APP_BASE_PATH=/kazokucal` の場合は、上記 API は `/kazokucal/api/...` 配下になります。

`GET /api/calendar` は指定年月の保存済みカレンダーを返します。未保存の場合は空のカレンダーを返します。

- 個人カレンダーの `weekStartsOn` / `telework`
- 個人予定
- `AUTH_GROUPS_HEADER` と `AUTHORIZED_GROUPS` で許可された group の予定
- `default` カレンダーに保存された全体共有予定

を合成した JSON を返します。

`PUT /api/calendar` は現在のカレンダー JSON を 1 か月分まるごと保存します。認証ありでは個人予定は認証ユーザー配下、グループ予定は選択した group 配下に保存されます。認証なしでは `default` カレンダーに保存され、これは認証ユーザーからも全体共有予定として閲覧されます。

`POST /api/pdf` はカレンダー JSON を受け取り、`application/pdf` を返します。フロントエンドの「グループの予定を含める」トグルが Off の場合、PDF も個人予定のみを出力します。

## データ保存

データはバックエンド API 経由で MySQL/MariaDB に保存します。

スキーマはアプリ起動時に自動初期化されます。主なテーブルは次の通りです。

- `calendars`
- `telework_days`
- `schedule_items`
- `multi_day_items`

`owner_key` は次のように使い分けます。

- 個人予定: 認証ユーザー名またはメールアドレス
- グループ予定: `group:<group-name>`
- 全体共有予定: `default`

## PDF 生成方式

PDF はブラウザ印刷ではなく、Go バックエンドから Python の ReportLab スクリプトを呼び出して生成します。A4 横向き、mm 単位の固定レイアウト、手動グリッド描画、曜日ヘッダ、日付、祝日、テレワーク、日別予定、複数日にまたがる帯予定を明示的に描画します。

日本の祝日は Python の `holidays` ライブラリを使っています。祝日ルールがライブラリ側で保守されるため、独自実装より更新しやすい構成です。PDF とフロントエンドプレビューの祝日表示は同じ Python 由来のデータを使います。

日本語表示のため、ランタイムには `fonts-morisawa-bizud-gothic`、`fonts-noto-core`、`fonts-noto-extra` を入れています。PDF では日本語に BIZ UD Gothic / BIZ UDP Gothic、英数字に Noto Sans を使う mixed-font 構成です。

- 年、月番号、大きな日付数字: Noto Sans Bold
- 英語の月名、曜日英語ラベル: Noto Sans SemiBold
- 曜日の日本語ラベル、テレワーク表示: BIZ UDゴシック Bold
- 祝日名、予定本文、複数日予定本文: BIZ UDPゴシック Regular
- 予定本文内の ASCII 英数字、時刻、記号: Noto Sans Medium

フォントファイルは PDF に埋め込まれます。

## 開発コマンド

フロントエンドだけを開発する場合:

```bash
cd frontend
npm install
npm run dev
```

バックエンドだけを実行する場合:

```bash
cd backend
go run ./cmd/server
```

ローカルでバックエンドを動かす場合は、MariaDB を起動し、`DB_DSN`、`PDFGEN_SCRIPT`、`HOLIDAY_SCRIPT` を設定してください。Python 依存関係も必要です。

```bash
python3 -m venv .venv
. .venv/bin/activate
pip install -r pdfgen/requirements.txt
DB_DSN='kazokucal:kazokucal@tcp(localhost:23306)/kazokucal?parseTime=true&charset=utf8mb4&loc=Asia%2FTokyo' \
PDFGEN_SCRIPT=../pdfgen/generate_calendar.py \
HOLIDAY_SCRIPT=../pdfgen/list_holidays.py \
go run ./cmd/server
```

## Header 認証の動作確認

固定 header を注入する確認用 compose を用意しています。

```bash
docker compose -f docker-compose.yml -f docker-compose.auth.yml up --build
```

起動後のアクセス先:

- `http://localhost:28181` : `alice` / groups=`family,dev`
- `http://localhost:28182` : `bob` / groups=`family`
- `http://localhost:28183` : `carol` / groups header はあるが `AUTHORIZED_GROUPS` に含まれないため group なし

この compose では `AUTHORIZED_GROUPS=family,dev,ops` を設定しているため、`ignored-group` はアプリ側で無視されます。

## 既知の制限

- 複数日の帯予定は週ごとに分割して描画します。
- 1 つの週に多数の帯予定が重なる場合、PDF とプレビューでは上位数件を優先表示します。

