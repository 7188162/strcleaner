# strcleaner — CSV 文字列正規化 & 重複排除ツール
Windows / Linux / macOS 対応のコンソールアプリ。Excel 由来の CSV、CP932(Shift\_JIS)・UTF-8(BOM 有無) を想定。

---

## 概要

- 設定ファイル（YAML/TOML）と環境変数、コマンドライン引数で動作を制御し、CSV の指定列を正規化します。
- **NFKC 正規化** を起点に、大小文字変換・幅変換・各種記号の統一/削除・HTML タグ除去・非印刷文字除去などを実行。
- **重複排除（データクレンジング）** に対応：
  - 正規化（する/しない）済みの値でキーを作成し、
  - 末尾にキー列を **追加** / 対象列を **置換** / 同一キーの行を **drop（first/last 選択）** できます。
- 列指定は **1 オリジン**（人に優しい Excel 流儀）。
- ログは `YYYY-MM-DD hh:mm:ss level message` 形式。

---

## ディレクトリ構成（例）

```
strcleaner/
├─ cmd/
│  └─ strcleaner/
│     └─ main.go
├─ internal/
│  ├─ config/
│  │  └─ config.go
│  ├─ csvproc/
│  │  └─ csvproc.go
│  ├─ logging/
│  │  └─ logging.go
│  └─ normalize/
│     ├─ normalize.go
│     └─ normalize_test.go
├─ go.mod
└─ README_strcleaner.md (このファイル)
```

---

## ビルド & 実行

### 前提

- Go 1.22 以降

### セットアップ

```bash
# 初期化
mkdir strcleaner && cd strcleaner

go mod init github.com/yourorg/strcleaner

# 依存導入
go get github.com/spf13/{cobra,viper}
go get github.com/sirupsen/logrus
go get golang.org/x/text

go mod tidy

# ビルド
go build ./cmd/strcleaner

# テスト
go test ./...
```

### 使い方（最短）

```bash
# 例: config.yaml を使って入力CSVを処理し、out.csv に出力
./strcleaner -i in.csv -o out.csv -c config.yaml -v
```

> **CLI の基本フラグ**：
>
> - `-i, --input` 入力 CSV（必須）
> - `-o, --output` 出力 CSV（未指定なら STDOUT）
> - `-c, --config` 設定ファイル（YAML/TOML）
> - `-q, --quiet` / `-s, --silent` 重要な結果のみ
> - `-v, --verbose` 詳細ログ
>
> 高度なオプションは **設定ファイル** または **環境変数** で指定してください（優先度は *CLI > Env > File > Default*）。

---

## 設定（YAML 例）

> TOML でも同じキーで指定可能。

### Python（pandas）スクリプトと同等のクリーニングを行う例

```yaml
# config.yaml
columns: [1]            # 1オリジン: 正規化する列 (例: 論文題目)
code_page: utf8         # cp932 なら "cp932"
has_header: true        # 1行目はヘッダ

normalize:
  to_lower: true
  remove_html: true
  remove_parens: true
  remove_non_printable: true
  dash_to_hyphen: false   # ハイフン類は統一せず、下で“削除”
  write_back: true        # 正規化した値で元列を上書き
  remove_chars: |-
      　 ' ‘ ’ “ ” " “” ` ゛
      . , 、 < > － – — ― ‐ - ー ： : ； ; 《 》 « » /

# 重複排除は使わない場合
# dedupe: { enabled: false }
```

### 重複排除：キー列を末尾に追加（append）

```yaml
columns: [1]
code_page: utf8
has_header: true

normalize:
  to_lower: true
  remove_html: true
  remove_parens: true
  remove_non_printable: true
  dash_to_hyphen: false
  write_back: false          # ★ 元の列は変更しない
  remove_chars: |-
      　 ' ‘ ’ “ ” " “” ` ゛
      . , 、 < > － – — ― ‐ - ー ： : ； ; 《 》 « » /

dedupe:
  enabled: true
  columns: [1,2]             # 例: 題目 + 著者 でキー作成（1オリジン）
  use_normalized: true       # ★ キーは正規化後の値で作る
  append_key: true           # ★ 末尾にキー列を追加
  replace_target: false
  drop_duplicates: false
  output_header: __dedupe_key
  delimiter: "|"
```

### 重複排除：対象列をキーで置換＋重複は最後を残す（keep=last）

```yaml
columns: [1]
code_page: utf8
has_header: true

normalize:
  to_lower: true
  remove_html: true
  remove_parens: true
  remove_non_printable: true
  dash_to_hyphen: false
  write_back: true           # 正規化した値で元列を上書き
  remove_chars: |-
      　 ' ‘ ’ “ ” " “” ` ゛
      . , 、 < > － – — ― ‐ - ー ： : ； ; 《 》 « » /

dedupe:
  enabled: true
  columns: [1,2]
  use_normalized: true
  append_key: false
  replace_target: true       # ★ columns 先頭（ここでは1列目）をキーで置換
  drop_duplicates: true
  keep: last                 # ★ 最後を残す（first/last）
  delimiter: "|"
```

---

## できること（正規化パイプライン）

順序は以下で固定：

0. **Unicode 正規化（NFKC）**
1. 幅/種別の変換
   - 半角カナ↔全角カナ（`half_kana_to_full` / `full_kana_to_half`）
   - 全角数字→半角（`full_digit_to_half`）
   - 全角括弧付き/丸付き数字→半角（`paren_num_to_half`）
2. **ハイフン統一**（`dash_to_hyphen` が true のとき `-` に統一）
3. **大小文字変換**（`to_upper`/`to_lower`）
4. **カッコ類の削除**（`remove_parens`）
5. **HTML タグ除去**（`remove_html`）
6. **非印刷文字の削除**（`remove_non_printable`：`\p{Cc}` と `\p{Cf}`）
7. **任意文字の削除**（`remove_chars` に列挙）
8. **前後空白のトリム**

> **メモ**: `write_back: false` にすると、上記の正規化は **キー計算には使うが元列へは書き戻さない** ことが可能です（`use_normalized` と組み合わせ）。

---

## 重複排除（dedupe）の仕様

- `columns`（1オリジン）で指定した列の **正規化後/前（**``**）** の値を連結し、キーを作成。
- `append_key` でキー列を末尾に追加（`output_header` はヘッダ行がある場合の見出し）。
- `replace_target` で `dedupe.columns` の **先頭列をキー文字列に置換**。
- `drop_duplicates` が true の場合：
  - `keep: first`（既定）… 最初に出現した行を残す
  - `keep: last` … 最後に出現した行を残す（全行メモリ保持）

> 連結区切りは `delimiter`（既定 `|`）。

---

## 文字コード（入出力）

- `code_page: utf8`（既定）/ `cp932` をサポート。
- `cp932` 指定時は、読み込みに Shift\_JIS デコーダ、書き込みにエンコーダを適用（BOM 有無は気にせず UTF-8 も可）。

---

## ログ

- 既定レベル: `info`（`-v` で `debug`、`-q/-s` で `error` 相当）。
- 出力: `stdout`（`log.output` で `stderr` に変更可）。
- 形式: `2006-01-02 15:04:05 level message`（logrus TextFormatter）。

**設定例**

```yaml
log:
  level: info   # trace|debug|info|warn|error|fatal|panic
  output: stdout
```

---

## 優先順位 & 環境変数

- 優先順位: **CLI > 環境変数 > 設定ファイル > 既定値**
- 環境変数のプレフィックスは `STRCLEANER_`。

**例**

```bash
# PowerShell 例
$env:STRCLEANER_CODE_PAGE = "cp932"
$env:STRCLEANER_HAS_HEADER = "true"
$env:STRCLEANER_NORMALIZE__TO_LOWER = "true"          # ネストは「__」など環境に応じて置換
$env:STRCLEANER_DEDUPE__ENABLED = "true"
```

> 環境変数のネスト表現はシェルや OS により挙動が異なるため、基本は **設定ファイル** を推奨。

---

## 入出力サンプル

**入力 (in.csv)**

```
題目,著者
Ａ<sup>Ｂ</sup>（Ｃ）,山田 太郎
Ａb<Br>ｶﾀｶﾅ,山田　太郎
```

**設定 (抜粋)**

```yaml
has_header: true
columns: [1]
normalize:
  to_lower: true
  remove_html: true
  remove_parens: true
  remove_non_printable: true
  write_back: true
  dash_to_hyphen: false
  remove_chars: |-
      　 ' ‘ ’ “ ” " “” ` ゛ . , 、 < > － – — ― ‐ - ー ： : ； ; 《 》 « » /
```

**出力 (out.csv)**

```
題目,著者
ab c,山田 太郎
abｶﾀｶﾅ,山田 太郎
```

※ 例示のため空白除去などは適宜。

---

## トラブルシュート

- ``
  - `go.mod` の `module` と import パスが不一致です。`module github.com/yourorg/strcleaner` に修正するか、`replace github.com/yourorg/strcleaner => .` を追加。
- ``**（build 失敗）**
  - `width.Transformer` に `TransformRune` はありません。`transform.String(width.Narrow, s)` を使用（本リポジトリは修正済み）。
- ``
  - `log.level` 未指定。`info` を既定で適用（本リポジトリは防御コード済み）。
- **列は 1 オリジン？**
  - はい。`columns: [1,3]` は 1 列目と 3 列目を指します。`0` 以下は無視。

---

## セキュリティ & 運用

- 入力/設定は外部データとして扱い、OS コマンド実行等は行いません。
- HTML はタグ単位で全除去（属性等の XSS 脅威は対象外＝まるごと捨てる方針）。
- `drop_duplicates: true` かつ `keep: last` は **全行をメモリ保持**します。大規模データは分割処理をご検討ください。

---

## 参考リンク

- Go Modules: [https://go.dev/doc/modules](https://go.dev/doc/modules)
- Cobra: [https://github.com/spf13/cobra](https://github.com/spf13/cobra)
- Viper: [https://github.com/spf13/viper](https://github.com/spf13/viper)
- logrus: [https://github.com/sirupsen/logrus](https://github.com/sirupsen/logrus)
- x/text (unicode/norm, width): [https://pkg.go.dev/golang.org/x/text](https://pkg.go.dev/golang.org/x/text)
- Unicode 正規化（Python `unicodedata`）: [https://docs.python.org/3/library/unicodedata.html](https://docs.python.org/3/library/unicodedata.html)

---

## 変更履歴（要点のみ）

- NFKC 正規化を常時実施
- HTML タグ除去 / 非印刷文字除去 / カッコ削除スイッチ追加
- 列指定を **1 オリジン** に変更
- 重複排除（append\_key / replace\_target / drop + keep=first|last）を追加
- `normalize.write_back` / `dedupe.use_normalized` を追加
- ヘッダ行サポート（`has_header`）を追加

---

## ライセンス

- （必要に応じて追記してください）

