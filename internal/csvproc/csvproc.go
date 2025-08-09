package csvproc

import (
	"encoding/csv"
	"io"
	"os"
	"strings"

	"github.com/yourorg/strcleaner/internal/config"
	"github.com/yourorg/strcleaner/internal/logging"
	"github.com/yourorg/strcleaner/internal/normalize"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func Process(inFile, outFile string, conf config.Config, log logging.Logger) error {
	in, err := os.Open(inFile)
	if err != nil {
		return err
	}
	defer in.Close()

	var reader io.Reader = in
	if strings.EqualFold(conf.CodePage, "cp932") {
		reader = transform.NewReader(in, japanese.ShiftJIS.NewDecoder())
	}
	r := csv.NewReader(reader)
	r.LazyQuotes = true

	var out io.Writer = os.Stdout
	if outFile != "" {
		f, err := os.Create(outFile)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}
	if strings.EqualFold(conf.CodePage, "cp932") {
		out = transform.NewWriter(out, japanese.ShiftJIS.NewEncoder())
	}
	w := csv.NewWriter(out)

	opts := normalize.Options{
		ToUpper:            conf.Normalize.ToUpper,
		ToLower:            conf.Normalize.ToLower,
		HalfKanaToFull:     conf.Normalize.HalfKanaToFull,
		FullKanaToHalf:     conf.Normalize.FullKanaToHalf,
		FullDigitToHalf:    conf.Normalize.FullDigitToHalf,
		ParenNumToHalf:     conf.Normalize.ParenNumToHalf,
		DashToHyphen:       conf.Normalize.DashToHyphen,
		RemoveParens:       conf.Normalize.RemoveParens,
		RemoveNonPrintable: conf.Normalize.RemoveNonPrintable,
		RemoveHTML:         conf.Normalize.RemoveHTML,
		RemoveChars:        conf.Normalize.RemoveChars,
	}

	// 1→0 オリジン変換
	toZero := func(cols []int) []int {
		z := make([]int, 0, len(cols))
		for _, c := range cols {
			if c > 0 {
				z = append(z, c-1)
			}
		}
		return z
	}
	targetCols := toZero(conf.Columns)

	dedupeCols := targetCols
	if len(conf.Dedupe.Columns) > 0 {
		dedupeCols = toZero(conf.Dedupe.Columns)
	}
	sep := conf.Dedupe.Delimiter
	if sep == "" {
		sep = "|"
	}

	type row struct {
		fields []string
		key    string // dedupe key（Enabled のときのみ使用）
	}
	var header []string
	var rows []row

	// ---- 読み込み（全行をメモリに積む。keep=last対応のため） ----------
	first := true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("CSV 読取エラー: %v", err)
			continue
		}

		if first && conf.HasHeader {
			header = append([]string{}, rec...)
			first = false
			continue
		}
		first = false

		// 正規化（対象列のみ）
		for _, col := range targetCols {
			if col >= 0 && col < len(rec) {
				rec[col] = normalize.Clean(rec[col], opts)
			}
		}

		// 重複キー作成
		var key string
		if conf.Dedupe.Enabled && len(dedupeCols) > 0 {
			values := make([]string, 0, len(dedupeCols))
			for _, col := range dedupeCols {
				if col >= 0 && col < len(rec) {
					values = append(values, rec[col])
				} else {
					values = append(values, "")
				}
			}
			key = strings.Join(values, sep)

			// 置換（columns先頭）
			if conf.Dedupe.ReplaceTarget && len(dedupeCols) > 0 {
				firstCol := dedupeCols[0]
				if firstCol >= 0 && firstCol < len(rec) {
					rec[firstCol] = key
				}
			}

			// 追加
			if conf.Dedupe.AppendKey {
				rec = append(rec, key)
			}
		}

		rows = append(rows, row{fields: rec, key: key})
	}

	// ---- ヘッダ出力 -----------------------------------------------------
	if conf.HasHeader {
		if conf.Dedupe.Enabled && conf.Dedupe.AppendKey {
			header = append(header, conf.Dedupe.OutputHeader)
		}
		if err := w.Write(header); err != nil {
			return err
		}
	}

	// ---- 重複排除 -------------------------------------------------------
	writeRow := func(i int) error {
		return w.Write(rows[i].fields)
	}

	if conf.Dedupe.Enabled && conf.Dedupe.DropDuplicates && len(dedupeCols) > 0 {
		seen := map[string]struct{}{}
		switch conf.Dedupe.Keep {
		case "last":
			// 後ろから走査し、最初に出会ったキーを採用
			keep := make([]bool, len(rows))
			for i := len(rows) - 1; i >= 0; i-- {
				k := rows[i].key
				if _, ok := seen[k]; !ok {
					seen[k] = struct{}{}
					keep[i] = true
				}
			}
			for i := 0; i < len(rows); i++ {
				if keep[i] {
					if err := writeRow(i); err != nil {
						return err
					}
				}
			}
		default: // "first"
			for i := 0; i < len(rows); i++ {
				k := rows[i].key
				if _, ok := seen[k]; ok {
					continue
				}
				seen[k] = struct{}{}
				if err := writeRow(i); err != nil {
					return err
				}
			}
		}
	} else {
		// 重複排除しない/キー未指定 → すべて出力
		for i := 0; i < len(rows); i++ {
			if err := writeRow(i); err != nil {
				return err
			}
		}
	}

	w.Flush()
	return w.Error()
}
