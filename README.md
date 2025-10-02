# strcleaner — CSV 文字列正規化 & 重複排除ツール

> Windows / Linux / macOS 対応のコンソールアプリ。Excel 由来の CSV、CP932(Shift\_JIS)・UTF-8(BOM 有無) を想定。

---

## 概要

* 設定ファイル（YAML/TOML）と環境変数、コマンドライン引数で動作を制御し、CSV の指定列を正規化します。
* **NFKC 正規化** を起点に、大小文字変換・幅変換・各種記号の統一/削除・HTML タグ除去・非印刷文字除去などを実行。
* **重複排除（データクレンジング）** に対応：

  * 正規化（する/しない）済みの値でキーを作成し、
  * 末尾にキー列を **追加** / 対象列を **置換** / 同一キーの行を **drop（first/last 選択）** できます。
* 列指定は **1 オリジン**（人に優しい Excel 流儀）。
* ログは `YYYY-MM-DD hh:mm:ss level message` 形式。

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

* Go 1.22 以降

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

* `-i, --input` 入力 CSV（必須）
* `-o, --output` 出力 CSV（未指定なら STDOUT）
* `-c, --config` 設定ファイル（YAML/TOML）
* `-q, --quiet` / `-s, --silent` 重要な結果のみ
* `-v, --verbose` 詳細ログ
* `--no-strict-config` 設定ファイルの未知キーを許容（厳格チェックを無効化）

高度なオプションは **設定ファイル** または **環境変数** で指定してください（優先度は *CLI > Env > File > Default*）。 **設定ファイル** または **環境変数** で指定してください（優先度は *CLI > Env > File > Default*）。

---

## 設定（YAML 例）

> TOML でも同じキーで指定可能。

### Python（pandas）スクリプトと同等のクリーニングを行う例

```yaml
# config.yaml
columns: [1]            # 1オリジン: 正規化する列 (例: 論文題目)
code_page: utf8         # cp932 なら "cp932"
has_header: true        # 1行目はヘッダ
output:
  line_ending: crlf     # 既定: crlf （Excel想定）
  utf8_bom: true        # 既定: true

normalize:
  to_lower: true
  remove_html: true
  remove_parens: true
  remove_non_printable: true
  dash_to_hyphen: false   # ハイフン類は統一せず、下で“削除”
  write_back: true        # 正規化した値で元列を上書き
  remove_chars: |-
      　 ' ‘ ’ “ ” \" “” ` ゛
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

### 完全版サンプル（推奨ひな形）

```yaml
# config.yaml — フルオプション例（このままコピペして用途に合わせて調整）

# 正規化対象列（1オリジン）— 例: 1列目=題目, 2列目=著者
columns: [1, 2]

# 入出力の文字コード
code_page: utf8           # utf8 | cp932
has_header: true          # 先頭行がヘッダなら true

# 出力フォーマット
output:
  line_ending: crlf       # crlf(既定) | lf
  utf8_bom: true          # UTF-8 のとき BOM を付けるか（既定 true, cp932 では無視）

# ログ
log:
  level: info             # trace|debug|info|warn|error|fatal|panic
  output: stdout          # stdout|stderr|<path/to/logfile>

# 正規化オプション
normalize:
  # 文字種・幅・ケース
  to_upper: false
  to_lower: true
  half_kana_to_full: false
  full_kana_to_half: false
  full_digit_to_half: true
  paren_num_to_half: true
  dash_to_hyphen: false          # trueだとハイフン類を"-"に統一, falseなら下の remove_chars で削除推奨

  # 構文要素の除去
  remove_parens: true            # 各種カッコの除去
  remove_html: false             # すべての <...> を削除する（タグ本体のみ。属性含む）
  remove_html_tags: [u, i, br]   # 特定タグだけ除去（中身は保持）。大小文字は不問
  remove_substrings:             # リテラル一致で除去（属性付きタグは残したい等の用途）
    - "<sup>"                    
    - "</sup>"
    - "<sub>"
    - "</sub>"

  # 非印刷制御
  remove_non_printable: false    # Cc/Cf を一括削除
  remove_crlf_only: true         # CR/LF だけ削除（上と独立して動作）

  # 一括カテゴリ削除（必要に応じて）
  remove_punctuation: false      # Unicode P*（句読点・括弧など）
  remove_symbols: false          # Unicode S*（記号。通貨記号なども含む）
  remove_emoji: false            # 代表的な絵文字ブロック＋ZWJ/VS-16/肌色修飾

  # 正規化値の書き戻し
  write_back: false              # true: 対象列を正規化後で上書き / false: 元列は保持（キー生成だけ正規化したいとき）

  # 個別文字の削除（集合の配列 or 文字列）— 例示（必要に応じて削る/足す）
  remove_chars:
    - "-–—―‐ｰー－"             # ハイフン類
    - "\u3000\u2000\u00A0"     # 全角/EN SPACE/NBSP
    - "『』「」｢｣（）()[]<>【】《》≪≫"  # 括弧類
    - "、。，．,.;；:"             # 句読点類
    - "/"                        # スラッシュ
    # 引用符類（コードポイント指定例）
    - "\u0022\u201C\u201D\u2018\u2019\u0060\u309B"

# 重複排除（キー作成と出力制御）
dedupe:
  enabled: true
  columns: [1, 2]                # キーに使う列（1オリジン）
  use_normalized: true           # 正規化後の値でキー作成（false: 原文のまま）
  append_key: true               # 末尾にキー列を追加
  output_header: 正規化キー       # 追加列の見出し（has_header: true のとき）
  replace_target: false          # columns 先頭列をキーで置換
  delimiter: "|"                 # 連結区切り
  drop_duplicates: true          # 同一キーの重複行を落とす
  keep: first                    # first(既定) | last
  ignore_empty_key: true         # 空キーは drop 対象外（安全網）

# 実行時タイムアウト（長時間処理対策）
timeout: 10m
```

> 補足:
>
> * `remove_chars` は **文字列/配列の両対応**。配列の各要素は「削除したい文字の集合」です。
> * Excel 互換（Windows用途中心）なら `code_page: cp932`, `output.line_ending: crlf`, `output.utf8_bom: true` が無難です。

---

## できること（正規化パイプライン）

順序は以下で固定：

0. **Unicode 正規化（NFKC）**
1. 幅/種別の変換

   * 半角カナ↔全角カナ（`half_kana_to_full` / `full_kana_to_half`）
   * 全角数字→半角（`full_digit_to_half`）
   * 全角括弧付き/丸付き数字→半角（`paren_num_to_half`）
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

* `columns`（1オリジン）で指定した列の **正規化後/前（**\`\`**）** の値を連結し、キーを作成。
* `append_key` でキー列を末尾に追加（`output_header` はヘッダ行がある場合の見出し）。
* `replace_target` で `dedupe.columns` の **先頭列をキー文字列に置換**。
* `drop_duplicates` が true の場合：

  * `keep: first`（既定）… 最初に出現した行を残す
  * `keep: last` … 最後に出現した行を残す（全行メモリ保持）

> 連結区切りは `delimiter`（既定 `|`）。

---

## 文字コード・出力フォーマット（入出力）

* `code_page: utf8`（既定）/ `cp932` をサポート。
* `cp932` 指定時は、読み込みに Shift\_JIS デコーダ、書き込みにエンコーダを適用。
* **出力改行コード** は `output.line_ending` で制御（`crlf` 既定 / `lf`）。
* **UTF-8 の BOM 有無** は `output.utf8_bom` で制御（既定: `true`）。`cp932` では無視されます。

**設定例**

```yaml
code_page: utf8
output:
  line_ending: crlf   # crlf|lf（既定: crlf）
  utf8_bom: true      # true|false（既定: true）
```

---

## ログ

* 既定レベル: `info`（`-v` で `debug`、`-q/-s` で `error` 相当）。
* 出力: `stdout`（`log.output` で `stderr` に変更可）。
* 形式: `2006-01-02 15:04:05 level message`（logrus TextFormatter）。

**設定例**

```yaml
log:
  level: info   # trace|debug|info|warn|error|fatal|panic
  output: stdout
```

---

## 優先順位 & 環境変数

* 優先順位: **CLI > 環境変数 > 設定ファイル > 既定値**
* 環境変数のプレフィックスは `STRCLEANER_`。

**例**

```bash
# PowerShell 例
$env:STRCLEANER_CODE_PAGE = "cp932"
$env:STRCLEANER_HAS_HEADER = "true"
$env:STRCLEANER_OUTPUT__LINE_ENDING = "lf"  # crlf|lf
$env:STRCLEANER_OUTPUT__UTF8_BOM = "false"  # true|false
$env:STRCLEANER_NORMALIZE__TO_LOWER = "true"
$env:STRCLEANER_DEDUPE__ENABLED = "true"
```

> 環境変数のネスト表現はシェルや OS により挙動が異なるため、基本は **設定ファイル** を推奨。

---

## 厳格YAMLモード（unknownキー検出）

* 既定では **厳格モード** で設定ファイルを検証し、未知フィールドやインデント誤りを **即エラー** にします。
* 一時的に無効化したい場合は \`\` を付与してください。

**例：エラー表示**

```
invalid config file (unknown/ misplaced key?): yaml: unmarshal errors:
  line 23: field dedupe not found in type config.NormalizeConfig
```

**例：厳格チェックを無効化**

```bash
./strcleaner -i in.csv -o out.csv -c config.yaml --no-strict-config
```

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

* \`\`

  * `go.mod` の `module` と import パスが不一致です。`module github.com/yourorg/strcleaner` に修正するか、`replace github.com/yourorg/strcleaner => .` を追加。
* \`\`**（build 失敗）**

  * `width.Transformer` に `TransformRune` はありません。`transform.String(width.Narrow, s)` を使用（本リポジトリは修正済み）。
* \`\`

  * `log.level` 未指定。`info` を既定で適用（本リポジトリは防御コード済み）。
* **列は 1 オリジン？**

  * はい。`columns: [1,3]` は 1 列目と 3 列目を指します。`0` 以下は無視。

---

## セキュリティ & 運用

* 入力/設定は外部データとして扱い、OS コマンド実行等は行いません。
* HTML はタグ単位で全除去（属性等の XSS 脅威は対象外＝まるごと捨てる方針）。
* `drop_duplicates: true` かつ `keep: last` は **全行をメモリ保持**します。大規模データは分割処理をご検討ください。

---

## 参考リンク

* Go Modules: [https://go.dev/doc/modules](https://go.dev/doc/modules)
* Cobra: [https://github.com/spf13/cobra](https://github.com/spf13/cobra)
* Viper: [https://github.com/spf13/viper](https://github.com/spf13/viper)
* logrus: [https://github.com/sirupsen/logrus](https://github.com/sirupsen/logrus)
* x/text (unicode/norm, width): [https://pkg.go.dev/golang.org/x/text](https://pkg.go.dev/golang.org/x/text)
* Unicode 正規化（Python `unicodedata`）: [https://docs.python.org/3/library/unicodedata.html](https://docs.python.org/3/library/unicodedata.html)

---

## 変更履歴（要点のみ）

* NFKC 正規化を常時実施
* HTML タグ除去 / 非印刷文字除去 / カッコ削除スイッチ追加
* 列指定を **1 オリジン** に変更
* 重複排除（append\_key / replace\_target / drop + keep=first|last）を追加
* `normalize.write_back` / `dedupe.use_normalized` を追加
* ヘッダ行サポート（`has_header`）を追加
