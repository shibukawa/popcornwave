---
title: ビルドツール設定
description: popcornwave.toml の全キー。pw が何を生成し、pw dev が何を並走させ、マイグレーションとスタイルシートがどこにあるか。
sidebar:
  order: 3
---

`popcornwave.toml` はプロジェクトルートに置かれ、`pw` コマンドのものです。書いてあるのは
*プロジェクト*のこと——生成がどのディレクトリを読むか、ソースがどのコンパイラ向けに
書かれているか、開発中にアプリケーションの横で何が動くか。

ランタイムの設定は一切ありません。ポート、コネクションプール、クッキー、ログレベルは
`config.{APP_ENV}.toml` にあり、
[アプリケーション設定](/ja/reference/configuration/)に一覧があります。この分離は
慣習ではなく強制です。ここに `server` や `session` テーブルを書けばエラーですし、
データベースの接続文字列も同様です。2 つのファイルは、別のプログラムが別のタイミングで
読みます。

このファイルを書くのは [`pw init`](/ja/pw/project/init/) で、機能を追加したときに
編集するのは [`pw add`](/ja/pw/project/add/) です。手で編集することも想定されています。
以下はローダーが実際に検査している規則です。

## `[project]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `name` | *(必須)* | プロジェクト名。`pw dev` が注入する `OTEL_SERVICE_NAME` でもある |
| `main` | *(必須)* | [`pw build`](/ja/pw/project/build/) がコンパイルするパッケージ。例 `"./cmd/myapp"` |
| `toolchain` | `"tinygo"` | ソースがどのコンパイラ向けに作られたか。`tinygo` または `go` |
| `database` | `"sqlite"` | `.pw.sql` がどの方言で生成されるか。`sqlite`、`postgres`、`mysql` |

`toolchain` も `database` も、これ以外の値は拒否します。そしてどちらの既定値も、好みでは
なく歴史です。キーが存在しなかった頃のプロジェクトは TinyGo でしかありえず、SQLite でしか
ありえませんでした。

`database` は*生成*への入力です。生成された Go があなたの SQL をどの方言として読むかを
決めます。アプリケーションが実際に接続するエンジンは、いまも `middleware.rdb.dsn` の
スキームから決まります。この 2 つを一致させるのはあなたの仕事です。保持を禁じられている
DSN を、このファイルが検査できるはずもありません。

## `[generate]`

```toml
[generate]
handlers = ["handlers"]
templates = ["handlers", "templates"]
queries = ["queries"]
config = ["cmd/myapp"]
pages = []
```

各 purpose は、[`pw generate`](/ja/pw/project/generate/) が*その purpose のために*読んで
よいディレクトリを列挙します。それ以外は読みません。`queries` が挙げていないディレクトリの
`.pw.sql` は、生成から見えません。だからこそ生成は、所有する purpose の外に置かれた
`.pw.html` や `.pw.sql` を黙って拾うのではなく、警告します。

| キー | 読む対象 | 必須 |
| --- | --- | --- |
| `generate.handlers` | ハンドラのソース。ルートとバインディングの解析対象 | はい |
| `generate.templates` | `.pw.html` テンプレート。ドキュメントシェルを含む | はい |
| `generate.queries` | `.pw.sql` のソース | はい |
| `generate.config` | 設定の登録 | はい |
| `generate.pages` | [ページツリー](/ja/advanced/discovered-routing/)のルート | いいえ |

`pages` 以外はすべて必須で、その purpose が何も生成しないことを表すのが `[]` です。
キーを書かないことではそれを表現できません。だから空リストと省略は同じではありません。
`pages` だけが例外なのは、ページツリーが存在する前に作られたプロジェクトにはキーもツリーも
無いからです。

エントリ自体の規則は次のとおりです。

- プロジェクトからの相対パスで、存在するディレクトリを指すこと
- 重複しないこと。同じ purpose の別エントリの内側に入れ子にしないこと——内側のソースは
  2 回計画され、2 回目の計画が 1 回目の出力を消してしまいます
- [ドキュメントシェル](/ja/guides/frontend/templates/)を持つ `generate.templates`
  エントリはちょうど 1 つ。2 つ目はエラーです
- `generate.pages` のエントリはツリー全体です。`templates` や `handlers` に重ねて
  挙げることも、それらのエントリと入れ子にすることもできません

ディレクトリ名は既定値であって、識別子ではありません。どの利用者も名前ではなく purpose の
リストを読むので、`handlers/` を `web/` に改名するのは、ディレクトリを移動して 1 行編集する
ことです。生成されるパッケージ名はディレクトリに従うので、ソースはそのままコンパイルできます。

## `[dev.watch]`

```toml
[dev.watch]
includes = []
excludes = []
```

[`pw dev`](/ja/pw/project/dev/) はリビルドの入力を探してモジュールを歩きます。生成と違って
既定の動作があるので、両方のキーとも省略できます。`includes` は歩きが見落とすファイルや
glob パターンを相対パスで追加します。`excludes` はディレクトリのサブツリーを飛ばします。
歩きを遅くするだけの大きなツリーには効きます。

## `[dev.idp]`

```toml
[dev.idp]
enabled = false
config = "devidp.toml"
port = 0
```

`pw dev` がアプリケーションの横で動かす
[開発用の認証プロバイダ](/ja/productivity/dev-identity-provider/)です。`enabled = true`
にはユーザー定義ファイルの存在が必要です。`port = 0` は空いているループバックポートを
確保します。`pw dev` が解決済みの issuer をアプリケーションに注入するので、これが有用な
既定値です。固定の番号が意味を持つのは、このプロジェクトの外に登録されたクライアントが
いる場合だけです。

## `[dev.otel]`

```toml
[dev.otel]
enabled = true
port = 0
max = 0
```

[テレメトリビューア](/ja/productivity/dev-telemetry-viewer/)であり、ここで唯一、既定で
有効なブロックです。`port = 0` がループバックポートを確保し、`pw dev` が解決済みの
エンドポイントを注入するのは `dev.idp` とまったく同じです。`max` はシグナルごとの保持件数の
上限で、`0` ならビューア自身の既定値になります。

`dev.idp` も `dev.otel` も、影響するのは `pw dev` だけです。

## `[migration]`

| キー | 既定値 | 意味 |
| --- | --- | --- |
| `dir` | `"migrations"` | [マイグレーションファイル](/ja/productivity/migrations/)の場所。プロジェクトからの相対パス |
| `auto` | `true` | `pw dev` の開始時に未適用のマイグレーションを適用する |

`auto` が有効にするのは、開発ループのこの 1 ステップだけです。アプリケーションが起動時に
自分でマイグレーションを実行するようになることは決してありません。それはコードに明示的に
書く呼び出しのままです。リクエストを処理するプロセスが、スキーマを変更すべきプロセスである
ことは滅多にないからです。

`dir` はツール側のパスです。ファイルの場所を `pw` に伝えるだけで、ランタイムの意味は
ありません。

## `[assets.tailwind]`

```toml
[assets.tailwind]
enabled = true
input = "assets/app.css"
output = "public/generated/app.css"
minify = true
```

プロジェクトを [Tailwind](/ja/guides/frontend/styling/) 付きで作った場合にあり、そうでなければ
ありません。`input` と `output` は別のファイルで、どちらもプロジェクトからの相対パスです。

`minify` だけは変わっています。[`pw build`](/ja/pw/project/build/) はキーの値に関わらず
minify し、`pw dev` は決してしません。この値が実際に効くのは
[`pw doctor`](/ja/pw/project/doctor/) で、デプロイ先の環境に対して minify されていない
スタイルシートを readiness の指摘として報告します。`true` のままにしてください。

Tailwind のプラグインはここでは設定しません。CSS のエントリに書く `@plugin` 宣言であり、
解決するのは Tailwind CLI です。Popcorn Wave はエントリをそのまま渡すだけで、プラグインの
レジストリを持ちません。

## ファイル全体にかかる規則

- **未知のキーはエラーです。** 打ち間違いが黙って無視されることはありません。
- **相対パスはこのファイルのディレクトリから解決され**、絶対パスは拒否されます。
  プロジェクトはひとまとまりで移動します。
- **コマンドのフラグがファイルより優先されます。** `pw migrate --dir=other` は、何も
  編集せずにその 1 回だけ別のディレクトリを読みます。
- **このファイルがプロジェクトの位置を決めます。** プロジェクトを対象にするコマンドは
  ——[`pw init`](/ja/pw/project/init/) と `pw version` 以外のすべて——作業ディレクトリから
  上に向かってこのファイルを探します。だから `pw` はどのサブディレクトリからでも動きます。
- **ランタイムの値は禁止です。** `server`、`session`、`security`、`middleware`、
  `observability` のテーブルはもう一方のファイルのものですし、データベースの接続文字列も
  そちらです。`project.database` が名指すのはエンジンであって、DSN でも資格情報でも
  ありません。
