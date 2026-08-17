---
title: DynamoDB クエリフォーマット
description: dynamo 構造体タグ、.pw.dynamo の宣言構文、コード生成時に行われる検査の一覧。
sidebar:
  order: 4
---

DynamoDB のアクセスには宣言が2つあり、互いに突き合わされます。`dynamo` タグを持つ Go の
構造体が*そのままスキーマ*であり——別の DDL はありません——`.pw.dynamo` ファイルがそれを
読むアクセスパターンを宣言します。`pw generate` は前者からアイテムのコーデックとテーブル
定義を、後者からパターン1つにつき1つの名前付き関数を作ります。

ここでは、Go 構造体と `.pw.dynamo` ファイルの両方を説明します。ストアを有効にする方法、
`[middleware.dynamo]` のキー、スキーマの適用方法については
[DynamoDB](/ja/guides/storage/dynamodb/)を参照してください。

## 生成が見る場所

どちらも `popcornwave.toml` の `generate.dynamo` が挙げるディレクトリに置きます。このキーは
`pw add dynamo` が書きます。どのディレクトリにも属さない `.pw.dynamo` は黙って飛ばされるの
ではなく、パス付きで報告されます。

生成は使われ方に導かれます。コーデックは実際に呼ばれた方向についてだけ現れるので、読み取りを
消すと生成コードもそれに合わせて縮みます。`.pw.dynamo` の宣言はその結果型の使用として数え
られるので、DynamoDB の用途が宣言だけのパッケージでも、生成されたクエリが必要とする
デコーダは手に入ります。

例外はキービルダです。`partitionkey` を宣言した型は、呼び出しが必要とするかどうかに関わらず
`ItemKey` とテーブル定義を受け取ります。アイテムを読む正式な書き方が
`Load(ctx, table, v.ItemKey())` であり、メソッド呼び出しは生成が発見できるものではないから
です。

## `dynamo` タグ

```go
type Reading struct {
	Sensor  Sensor    `dynamo:"sensor,partitionkey"`
	At      int64     `dynamo:"at,sortkey"`
	Celsius float64   `dynamo:"celsius"`
	Flags   []string  `dynamo:"flags,stringset,omitempty"`
	Taken   time.Time `dynamo:"taken,unixtime"`
	Ignored string    `dynamo:"-"`
}
```

```
dynamo:"<属性名>[,<オプション>...]"
```

名前を空にすると Go のフィールド名を使います。`dynamo:"-"` はそのフィールドを飛ばし、
非公開のフィールドは常に飛ばされます。

| オプション | 意味 |
| --- | --- |
| `partitionkey` | このフィールドがテーブルのパーティションキー |
| `sortkey` | このフィールドがテーブルのソートキー |
| `omitempty` | フィールドがゼロ値のとき属性そのものを書かない |
| `stringset` | スライスを `L` ではなく `SS` として保存する |
| `numberset` | スライスを `NS` として保存する |
| `binaryset` | スライスを `BS` として保存する |
| `unixtime` | `time.Time` をエポックからの秒数の `N` として保存する |

この一覧に無いオプションは生成エラーです。ここが持つ価値のある差で、リフレクション方式の
マッパは知らないオプションを「何も無い」と読み、セットを頼んだ場所に黙って `L` を保存します。

タグの綴りは `dynamo` であって、AWS SDK の `dynamodbav` ではありません。`dynamo` を持たずに
`dynamodbav` だけを持つフィールドは、Go の名前で黙って保存されるのではなく生成エラーです。

### 属性の型

| Go の型 | 属性 | 備考 |
| --- | --- | --- |
| `string` | `S` | 空文字列も値であり、保存される |
| `int`…`int64`, `uint`…`uint64` | `N` | `strconv` 経由。`float64` は通さない |
| `float32`, `float64` | `N` | |
| `bool` | `BOOL` | |
| `[]byte` | `B` | |
| `time.Time` | RFC 3339 nano の `S`, `unixtime` なら `N` | |
| `[]T` | `L`、セットオプション付きなら `SS`／`NS`／`BS` | |
| `map[string]T` | `M` | 文字列でないキーは生成エラー |
| ネストした構造体 | `M` | 同じパッケージで宣言されている必要がある |
| `*T` | 指す先の値、nil なら `NULL` | |
| `dynamodb.AttributeValue` | そのまま保存される | 脱出口 |

名前付きの型は、その基底型が使えるところならどこでも使えます。`type Sensor string` は `S` で、
生成コードが変換します。

数値は端から端までテキストです。DynamoDB の数値は有効桁数 38 桁を運びますが `float64` は
運べないので、ここで float を経由するものは何もありません。フィールドより広い値は、黙って
折り返るのではなくデコードエラーになります。どの Go の型にも収まらない桁数の数値でも、
`dynamodb.AttributeValue` のフィールドなら往復できます。

デコードは、アイテムにその属性が無いときフィールドをそのままにします。古い版の構造体で
書かれたアイテムも、エラー無しにデコードできます。

## クエリの宣言

```
[export] statement <Name>(<param>: <GoType>, ...): dynamo.<shape><<ItemType>> {
  table <name>
  key <attribute> = {param} [and <attribute> <predicate>]
}
```

```
export statement ReadingsSince(sensor: Sensor, from: int64): dynamo.many<Reading> {
  table reading
  key sensor = {sensor} and at > {from}
}

export statement ReadingsBetween(sensor: Sensor, lo: int64, hi: int64): dynamo.page<Reading> {
  table reading
  key sensor = {sensor} and at between {lo} and {hi}
}

statement readingsForSensor(sensor: Sensor): dynamo.many<Reading> {
  table reading; key sensor = {sensor}
}
```

- パラメータの型は、自分のパッケージが綴るとおりの Go の型です。名前付きの型や `[]byte` も
  含みます。
- どちらの句も必須です。順序はどちらでもよく、1行に書くときは `;` で区切ります。
- `//` から行末までがコメントです。
- `export` は名前自身の大小と一致している必要があります。Go が可視性を名前で決めるからです。
  片方だけは、黙って改名されるのではなく生成エラーです。

### 結果の形

形が選ぶのは行数ではなく*リクエスト*の形です。Query は常に複数行を返すからです。

| 形 | 生成される戻り値 | リクエスト |
| --- | --- | --- |
| `dynamo.many<T>` | `iter.Seq2[T, error]` | range が進むごとに1ページ分 |
| `dynamo.page<T>` | `(dynamobind.Page[T], error)` | ちょうど1回 |

`Page[T]` は `Count`、`ScannedCount`、`LastEvaluatedKey` を運びます。イテレータはそのどれも
報告しないので、返す量の100倍をフィルタで走査しているクエリも、そうでないクエリと見分けが
つきません。途中で止めた実行を再開することもできません。何回リクエストするかは書き手の判断
のままで、だから両方あります。

### キー条件

パーティションキーの述語は必須で、先頭に来て、常に `=` です。DynamoDB がそこに他を許さない
からです。その後ろにソートキーの述語を最大1つ置けます。

| 書き方 | 送られるもの |
| --- | --- |
| `at = {p}` | `=` |
| `at < {p}`, `at <= {p}`, `at > {p}`, `at >= {p}` | その比較 |
| `at between {lo} and {hi}` | `BETWEEN` |
| `begins_with(at, {p})` | `begins_with`。文字列のソートキーのみ |

### `table` 句

`table` はこのパターンが走るテーブルを名指し、それが生成シグネチャからテーブルの引数を
取り除きます。型ではなく本文に置かれているのは、型が1つのテーブルではないからです。同じ
構造体がテスト用のテーブルにも本番のテーブルにも入る以上、型に書いたテーブルは事実でない
ことを主張してしまいます。

名前は DynamoDB が受け付けるもの——英数字と `_`、`-`、`.` で3〜255 文字——に対して検査される
ので、サービスが拒否する名前は最初の呼び出しでの `ValidationException` ではなく生成エラーに
なります。

デプロイ先で別の名前になっている場合、それはここには書きません。下の「宣言上の名前と実際の
名前」を参照してください。

### 生成されるシグネチャ

```go
func ReadingsSince(ctx context.Context,
	sensor Sensor, from int64, opts ...dynamodb.QueryOption) iter.Seq2[Reading, error]
```

テーブルの引数もクライアントの引数もありません。可変長のオプションはドライバへ届くので、
`dynamodb.WithLimit`、`WithScanForward`、`WithConsistentRead`、`WithIndex` はどれも効きます。
生成された式の名前と値は最後に追加されるので、呼び出し側のオプションが宣言の記述する条件を
置き換えることはできません。

### 予約語は処理済み

DynamoDB は `status`、`name`、`size`、`type`、`data`、`year`、`count`、`timestamp` を含む
573 語を予約しており、式にそのまま書くと `ValidationException` で拒否されます。生成された
クエリはすべての属性を無条件にエイリアスするので、この問題は起きません。

```go
const readingsSinceKeyCondition = "#k0 = :v0 AND #k1 > :v1"

var readingsSinceAttributeNames = map[string]string{"#k0": "sensor", "#k1": "at"}
```

名前が生成時に分かっているので、式もエイリアスの表も定数です。呼び出しごとに組み立てるものは
無く、予約語の一覧を抱えて最新に保つ必要もありません。

## アイテム操作

```go
h, err := dynamo.Handle(ctx)

LoadOn[T](ctx, h, table, key, opts...) (T, error)
StoreOn(ctx, h, table, v, opts...) error
RemoveOn(ctx, h, table, v, opts...) error
UpdateOn(ctx, h, table, v, expression, opts...) error

StoreReturningOn(ctx, h, table, v, opts...) (T, bool, error)
RemoveReturningOn(ctx, h, table, v, opts...) (T, bool, error)

QueryPageOn[T](ctx, h, table, keyCond, opts...) (Page[T], error)
ScanPageOn[T](ctx, h, table, opts...) (Page[T], error)
QueryOn[T](ctx, h, table, keyCond, opts...) iter.Seq2[T, error]
ScanOn[T](ctx, h, table, opts...) iter.Seq2[T, error]

StoreAllOn(ctx, h, table, vs) (unprocessed []T, err error)
LoadAllOn[T](ctx, h, table, keys, opts...) (items []T, unprocessed []dynamodb.Key, err error)
```

これらがテーブル名を取るのは、読み取る宣言を持たないからです。ハンドルは
`database/dynamo` のプロセスクライアントに設定済みのテーブル名解決を束ねたもので、
セクションを有効にしないまま呼び出すと、panic ではなくクライアントが無いことを名指した
エラーを受け取ります。宣言済みクエリは同じハンドルを自分で解決するので、ハンドルを
受け取るのはこの直接操作だけです。

`Store` は `PutItem` で、アイテム全体を置き換えます。`Update` は DynamoDB の更新式をそのまま
受け取り、構造体タグが実際に与えられる部分であるキーだけを補います。`StoreReturning` と
`RemoveReturning` は `ALL_OLD` を要求します。返る bool は、そこに何も無かったときに false に
なり、それはエラーではありません。

`StoreAll` と `LoadAll` は入力を DynamoDB が受け付けるリクエストへ分割します。`MaxBatchWrite`
は 25、`MaxBatchGet` は 100 で、どちらも公開されているので、入力量を調整する呼び出し側は
分割が使うのと同じ数値を読めます。サービスが受け付けなかった分は `unprocessed` として返り、
未処理分を再試行するかどうかは、アプリケーション側で決めます。ドライバはすでにトランスポートを再試行しているので、ここで
ループを回すとそれを黙って掛け算することになります。`LoadAll` は DynamoDB が返す順のまま
アイテムを返し、何にも一致しなかったキーは単に含まれません。

`Query` と `QueryPage` の検査されない文字列キー条件の形は、宣言で表せないもののために残って
います。そのテキストはタグと突き合わされませんし、上の予約語のエイリアスも自分で書く必要が
あります。

## 宣言上の名前と実際の名前

`table` 句の名前も、アイテム操作が渡す名前も、コードが宣言する名前です。それをデプロイ先の
名前へ写す関数が1つあり、リクエスト経路もマイグレータも同じものを通ります。

```toml
[middleware.dynamo]
table_prefix = "myapp-"

[[middleware.dynamo.table_names]]
declared = "reading"
deployed = "readings-prod-8f21c"
```

明示的なエントリが勝ち、無ければ接頭辞が効き、それも無ければ宣言した名前がそのまま使われ
ます。どちらのキーでも表せないデプロイには `dynamo.WithTableResolver` が合成された関数ごと
差し替えます。どのコードも宣言していないテーブルを名指す `table_names` のエントリは、黙って
何もしない行ではなくエラーです。

## エラー

ドライバのセンチネルはすべて生き残るので、見つからなかったことがゼロ値として届くことは
ありません。

```go
_, err := dynamobind.Load[Reading](ctx, "reading", key)
if errors.Is(err, dynamodb.ErrItemNotFound) { … }

var driverError *dynamodb.Error
if errors.As(err, &driverError) {
	log.Println(driverError.Op, driverError.RequestID, driverError.Retryable())
}
```

デコードの失敗は属性と両方の型を名指します。`AsError` はリフレクションを必要とする
`errors.As` を使わずに連鎖をたどります。

```go
if mapping, ok := dynamobind.AsError(err); ok {
	log.Println(mapping.Attribute, mapping.Expected, mapping.Got) // at N S
}
```

## 生成エラー

どの検査も、型とフィールド、あるいはステートメントと属性を名指します。

タグと型の検査:

- 未知の `dynamo` タグオプション
- `dynamo` タグを持たないフィールドの `dynamodbav` タグ
- 1つの属性名へ対応づく2つのフィールド
- 2つの `partitionkey`、2つの `sortkey`、`partitionkey` の無い `sortkey`
- 属性が `S`・`N`・`B` のいずれでもないキーのフィールド
- 属性の形を持たない Go の型、文字列でないキーのマップ、要素型と合わないセットオプション
- 別パッケージで宣言されたネスト構造体
- `EncodeItem`・`DecodeItem`・`ItemKey` をすでに手で宣言している型

クエリの検査:

- `table` 句が無い、または2つあるステートメント
- DynamoDB が拒否するテーブル名
- `dynamo` タグを持たないアイテム型、`partitionkey` を持たないアイテム型
- 型が持たない属性
- キー句の中の非キー属性——メッセージがそれの属すべき句を名指します
- `=` でないパーティションキーの述語、先頭でないパーティションキーの述語
- 2つ以上のソートキーの述語
- 文字列として保存されない属性への `begins_with`
- 属性の Go の型と一致しないパラメータ
- 宣言されたパラメータを指さないプレースホルダ、一度も使われないパラメータ
- 同じ名前の2つのステートメント

## 宣言できる範囲の外

| 無いもの | 帰結 |
| --- | --- |
| filter、projection、condition、update の各式 | `filter` 句はそう告げるメッセージで拒否される。それらの式は自分で渡す |
| セカンダリインデックス | `gsi` タグが無いので、宣言型クエリはテーブル自身のキーに対して走る。`dynamodb.WithIndex` はドライバへ届くが、条件がそのインデックスに合うかは検査されない |
| シングルテーブル設計 | `<Type>Table` が1つの型を記述し、型付きの読み取りが全アイテムを1つの型としてデコードするので、1つの構造体が1つのテーブルを持つ |
| 楽観的ロックと TTL | `version` タグも `ttl` タグも無く、どちらも呼び出し側が管理する |
| トランザクション、PartiQL、Streams、DAX | ドライバが持たない |

どれも Popcorn Wave の選択ではありません。下の層ができることの縁です。
