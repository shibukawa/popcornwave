---
title: E2E テスト
description: Playwright で開発サーバーを操作し、同じデータセットファイルで HTTP 経由のシードとアサーションを行う。
sidebar:
  order: 2
---

[testutil](/ja/productivity/testing/) はデプロイするアプリケーションそのものを起動し、
HTTP で到達します。ただしそのクライアントは Go の `http.Client` です。見えるのは
レスポンスまでで、その先はありません。スクリプトは走らず、
[ダイアログは開かず](/ja/guides/interactivity/browser-controls/)、
[フラグメントがページに収まる](/ja/guides/interactivity/fragments/)こともない。
テストしたい振る舞いにブラウザ側の半分が含まれるなら、テストがブラウザを動かすしか
ありません。その層が [Playwright](https://playwright.dev/) です。そして Popcorn Wave
は途中まで迎えに来ています。`pwdev` ビルドモードではアプリケーション自身がシードと
アサーションのエンドポイントを提供するので、ブラウザスイートは、他のテストがすでに
使っているのと同じデータセットファイルを通してデータベースを初期化し、検証できます。

書き始める前に、配分を考えてください。ブラウザテストはこのサイトが扱う中で最も高価な
テストです。桁違いに遅く、書き込みは本物です。テストとサーバーが別プロセスである以上、
[`WithTransaction`](/ja/productivity/testing/#withtransaction) が巻き戻してくれる
ものは何もありません。カバレッジの大半は、隔離が無料で付いてくる `testutil` に置く
べきです。ブラウザテストが元を取るのは、Go のクライアントには観測できない振る舞いを
ブラウザが担っているところだけです。

## Playwright を開発サーバーに向ける

スイートの雛形は `npm init playwright@latest` が作ります。差し替える価値があるのは
設定です。Popcorn Wave アプリケーションを動かすならこうなります。

```ts
// playwright.config.ts
import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
	testDir: './e2e',
	workers: 1,
	use: { baseURL: 'http://127.0.0.1:8080' },
	projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
	webServer: {
		command: 'pw dev',
		url: 'http://127.0.0.1:8080/',
		reuseExistingServer: !process.env.CI,
		timeout: 120_000,
	},
});
```

サーバーは、どこで走らせるときも [`pw dev`](/ja/pw/project/dev/) です。手元の
マシンではループはすでに動いていて、`reuseExistingServer` は対抗馬を立てる代わりに
そのサーバー —— いま目の前にある、生成もマイグレーションも済み、変更のたびにビルドし
直されるアプリケーション —— をスイートに使わせます。CI では `command` が同一の
ループを起動します。サービス、生成、マイグレーション、そして開発用認証プロバイダまで
含めてです。ログインページが何の支度もなしにスイートで動くのはこのためです。長めの
`timeout` はループの起動、つまりポートのバインドではなくビルドを待つためのものです。

`workers: 1` は用心ではなく算数です。すべてのテストが 1 つのアプリケーションと 1 つの
データベースを相手にし、書き込みはすべてコミットされるので、ワーカーが 2 つあれば
テーブルの状態が交錯します。1 ファイル内のテストはもともと順に実行されます。この指定は
その順序をファイル間にも広げるものです。直列実行で足りなくなったスイートに必要なのは
ワーカーごとのデータベースで、それはたいていのアプリケーションには過剰な構えです。

この設定が指定していないものにも意味があります。`APP_ENV` がありません。スイートは
開発環境で、開発データベースに対して走ります。これは書き忘れではなく判断です。後述の
シードエンドポイントはこの環境に施錠されていて、それは
[開発向けの緩和すべてが使う許可リスト](/ja/guides/architecture/security/#開発以外では起動しない)
と同じものです。帰結ははっきり言っておきます。**スイートはデータセットが名指しする
テーブルを入れ直すので、手で打ち込んだ行はテスト実行を生き残りません。** その喪失が
高くつくと感じたなら、その行は[データセット](/ja/productivity/seed-data/)に入れる
べきだったのです。

## 最初の画面テスト

```ts
// e2e/members.spec.ts
import { test, expect } from '@playwright/test';

test('メンバー一覧に全員が表示される', async ({ page }) => {
	await page.goto('/members');
	await expect(page.getByRole('listitem')).toHaveText(['Frank', 'Grace', 'Heidi']);
});
```

セレクタは CSS クラスや生成された id ではなく、ロールと見えているテキストを通します。
ページの意味を保ったままテンプレートを書き換えてもテストが生き残るのはそのためで、
内部関数ではなく公開ルートを叩く、というテストの流儀のブラウザ版にあたります。

この 3 人は `initial.yaml` が挿入する行 ——
[シードデータ](/ja/productivity/seed-data/)で紹介したデータセット —— です。そして
そこが、このままでは弱いところでもあります。アサートしている状態を、誰も用意して
いません。前回の実行や、手作業でいじった午後が、`member` テーブルに何を残していても
おかしくない。画面テストにも、Go のスイートがフィクスチャから得ていたもの —— テスト
自身が用意する既知の開始状態 —— が必要です。

## スイートからのシード投入

`pwdev` ビルドでは、アプリケーションが自分のデータセットに自分で答えます。

```
POST /_pw/test/seed/{dataset}     データセットを動いているデータベースに適用する
GET  /_pw/test/assert/{dataset}   データベースをデータセットと突き合わせる
```

名前の解決は `pw seed` や `testutil.WithSeed` とまったく同じです。`testdata/seed`
からの相対で、拡張子は省略でき、サブディレクトリも使えます。しかも同じファイルに
対して解決されるので、CLI、Go のスイート、ブラウザスイートの 3 者がずれずに済みます。
短いヘルパーでエンドポイントをテストの語彙にします。

```ts
// e2e/db.ts
import { expect, type APIRequestContext } from '@playwright/test';

export async function seed(request: APIRequestContext, dataset: string) {
	const response = await request.post(`/_pw/test/seed/${dataset}`);
	expect(response.status(), await response.text()).toBe(204);
}

export async function assertDB(request: APIRequestContext, dataset: string) {
	const response = await request.get(`/_pw/test/assert/${dataset}`);
	expect.soft(response.status(), await response.text()).toBe(204);
}
```

Playwright の `request` フィクスチャは `baseURL` を最初から知っているので、呼び出しは
テスト対象のサーバーに届きます。書き込むテストは、走る前に入れ直します。

```ts
import { test, expect } from '@playwright/test';
import { seed } from './db';

test.beforeEach(async ({ request }) => seed(request, 'initial'));

test('メンバーをアーカイブすると行が消える', async ({ page }) => {
	await page.goto('/members');
	await page.getByRole('row', { name: 'Grace' })
		.getByRole('button', { name: 'Archive' }).click();
	await expect(page.getByRole('row', { name: 'Grace' })).toHaveCount(0);
});
```

既定の操作は clear-insert です。ファイルが名指しした各テーブルは並べた行そのものに
戻り、出てこないテーブルには触れません —— Go スイートの
[`server.Seed`](/ja/productivity/testing/#テストの途中で入れ直す) と同じ意味論です。
同じ機構が同じファイルを読んでいるのだから当然でもあります。リセットは開始時に。
終了時の後始末はしないでください。後始末は失敗すると「次の」テストを静かに汚しますが、
入れ直しから始まるテストは、前のテストが何を残していようと正しく走ります。呼び出しは
動いているプロセスへの HTTP リクエスト 1 本なので、全テストの `beforeEach` に置いても
測るほどのコストになりません。

ブラウザスイートのためだけのデータセット —— たとえばページネーションを発生させる
ための幅広いカタログ —— は、サブディレクトリに置きます。

```ts
await seed(request, 'e2e/wide_catalog');   // testdata/seed/e2e/wide_catalog.yaml
```

引数なしの `pw seed` はサブディレクトリに降りないので、開発者の日常的な入れ直しが、
テストだけが欲しかったデータを適用してしまうことはありません。

エンドポイントが存在するかどうかは 3 つの錠が決めます。他の開発向け緩和と同じ並びで、
`pwdev` ビルドモードであること、`APP_ENV` が `dev` に解決されること、そして転送
ヘッダを持たないループバックからの呼び出しであることです。リリースバイナリはエンド
ポイントのバイト列を持たず、3 つが揃わない限り、閉じられた `/_pw` 名前空間が 404 を
返します —— エンドポイントが最初から存在しなかった場合と見分けはつきません。

## フィクスチャの往復

Go スイートの[フィクスチャの話](/ja/productivity/testing/#フィクスチャ) —— 1 つの
ファイルを開始状態に、もう 1 つを期待状態に —— は、ブラウザスイートからも成立します。
assert エンドポイントは `server.AssertDB` の HTTP 版だからです。

```ts
import { test, expect } from '@playwright/test';
import { seed, assertDB } from './db';

test('アーカイブしても他のメンバーは消えない', async ({ page, request }) => {
	await seed(request, 'initial');
	await page.goto('/members');
	await page.getByRole('row', { name: 'Grace' })
		.getByRole('button', { name: 'Archive' }).click();
	await expect(page.getByRole('row', { name: 'Grace' })).toHaveCount(0);
	await assertDB(request, 'after_archive');
});
```

ページのアサーションはフローが正しく見えたと言い、`after_archive.yaml` はデータベース
がそれに同意していると、テーブルまるごとの単位で言います。目的のメンバーを正しく
アーカイブし、ついでに別人の行まで消してしまうハンドラは、最初の検査を通って 2 つ目で
落ちます —— [フィクスチャ](/ja/productivity/testing/#フィクスチャ)が Go で捕まえる
のと同じ巻き添え被害を、TypeScript から出ずに捕まえられるわけです。不一致は 409 と
テーブルごとの差分(プレーンテキスト)で返り、ヘルパーはそれを失敗メッセージとして
表に出します。`expect.soft` は、ずれを報告しつつテストの残りを続ける `AssertDB` の
振る舞いをそのまま映したものです。

Go スイートとの違いを 1 つだけ握っておいてください。ここにテストトランザクションは
なく、エンドポイントが比べるのはコミット済みの状態です。ブラウザテストが生むのは
まさにそれなので普段は問題になりませんが、処理中のリクエストと競走するアサーションは
早すぎる比較になります。上の例のように、ページが結果を見せてからアサートしてください。
また、比較ではなく計算が要る検査 —— カウンタ、導出値 —— は
[`server.Context()`](/ja/productivity/testing/#データベースに対するアサーション) を
持つ `testutil` のテストであって、ブラウザテストの仕事ではありません。

## ログインフロー

[開発用認証プロバイダ](/ja/productivity/dev-identity-provider/)は、スイートがすでに
操作しているループの一部です。`pw dev` がプロバイダを起動し、issuer とクライアント
資格情報をアプリケーションに渡します。手元でも CI でも同じです。ログインページも他の
ページと同じように操作してください。なお、見えるフローではなくログインの「ロジック」
なら、そもそもブラウザは要りません。
[`WithIdentityProvider`](/ja/productivity/testing/#withidentityprovider) が Go の
テストの中で交換全体を、より速く、データベーストランザクションを保ったまま完結させます。

## 専用データベース

スイートが開発データベースに触れてはならないとき —— 1 つの PostgreSQL を複数人で
共有している、手元に残しておきたいデータをデータセットが踏み潰す —— は、シード
エンドポイントは付いてきません。開発環境に施錠されているのは意図したとおりの動作です。
もう 1 つの環境を宣言し、施錠のない CLI に戻ってください。

```toml
# config.e2e.toml
[server]
port = 8090

[middleware.rdb]
enabled = true

[[middleware.rdb.connections]]
dsn = "sqlite://e2e.db"
```

あとは `APP_ENV=e2e` がこのファイルを一斉に選びます。`webServer` に
`env: { ...process.env, APP_ENV: 'e2e' }` を足せば同じループがこの設定で立ち上がり、
`pw migrate` と `pw seed` も
[同じ変数に従います](/ja/pw/database/seed/#dsn-の取得元)。シードはエンドポイントへの
POST の代わりに、テストの合間に `pw seed` をサブプロセスとして実行します。呼び出しの
たびにアプリケーションをコンパイルして DSN を知る分だけ遅くなりますが、正しく動き
ます。境界はセッションです。`dev` の外では `session.cookie.secure = false` という
緩和は起動時に拒否され、WebKit は素の `http` ループバックからの `Secure` クッキーを
保存しません。フローがセッションクッキーに乗るアプリケーションは、スイートを開発環境に
置いたままにしてください。専用環境はそれ以外のためのものです。
