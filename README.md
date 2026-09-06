# Home Signal

Lenovo Tab M8 を家庭内デジタルサイネージにする、Go + BarefootJS の LAN 向け Web アプリです。甲府の天気、Google Calendar、ニュース、外部 Flex Timer の状態をサーバーで集約し、タブレットには描画用データだけを送ります。

## 開発

```sh
npm install
npm run dev
# http://127.0.0.1:8092
```

## 設定

| 環境変数 | 内容 | 既定値 |
|---|---|---|
| `LISTEN_ADDR` | listen address | `127.0.0.1:8092` |
| `GOOGLE_CALENDAR_ID` | 表示するカレンダー ID | 未設定 |
| `GOOGLE_SERVICE_ACCOUNT_FILE` | Calendar readonly 権限を持つサービスアカウント JSON | 未設定 |
| `NEWS_FEED_URL` | RSS 2.0 feed | NHK 主要ニュース |
| `FLEX_TIMER_URL` | [Flex Timer API](openapi/flex-timer.yaml) の base URL | 未設定 |
| `FLEX_TIMER_TOKEN` | Flex Timer の Bearer token | 未設定 |

Google Calendar は対象カレンダーをサービスアカウントのメールアドレスへ「予定の表示」権限で共有してください。未設定または各サービスが停止中でも、取得済みキャッシュを表示してサイネージ全体は動作を続けます。

## 本番ビルド

```sh
npm run build
APP_ENV=production go run .
```

品質チェックは TSX/TypeScript/CSS/JSON を oxfmt と oxlint、Go を golangci-lint で検証します。

```sh
npm run format
npm run check
```

Tab M8 は Chrome の「ホーム画面に追加」またはキオスクブラウザで横向き表示を想定しています。縦向きレイアウトと `prefers-reduced-motion` にも対応しています。

A [BarefootJS](https://barefootjs.dev) app scaffolded with the **net/http (Go standard library, html/template SSR)** adapter.

## Getting started

```sh
npm install
npm run dev
```

## Build & deploy

```sh
npm run build
```

## `bf` CLI cheat sheet

The `bf` CLI is the first reference for component APIs and framework docs — run `bf --help` for the full command list.

| Command | What it does |
| --- | --- |
| `bf search <term>` | Search the component registry |
| `bf add <component>` | Add a component from the registry |
| `bf docs <component>` | Show a component's API surface |
| `bf debug graph <component>` | Inspect a `"use client"` component's reactive signal graph |
| `bf guide` | Open the framework guide |

## Generated output

The compiled output directory (produced by `vite build`) is regenerated on every build — don't edit it by hand.
